package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/nexpay/nexpay-sdk/internal/model"
	"github.com/nexpay/nexpay-sdk/internal/payment"
	"github.com/nexpay/nexpay-sdk/internal/repository"
	"go.uber.org/zap"
)

var (
	ErrUnknownChannel = errors.New("unknown payment channel")
	ErrOrderNotFound  = errors.New("order not found")
	ErrRefundNotFound = errors.New("refund not found")
	ErrAmountMismatch = errors.New("notify amount mismatch")
)

// PaymentService 统一支付业务编排
type PaymentService struct {
	providers  map[model.Channel]payment.Provider
	orderRepo  repository.OrderRepository
	refundRepo repository.RefundRepository
	notifyRepo repository.NotifyLogRepository
	log        *zap.Logger
}

func NewPaymentService(
	providers map[model.Channel]payment.Provider,
	orderRepo repository.OrderRepository,
	refundRepo repository.RefundRepository,
	notifyRepo repository.NotifyLogRepository,
	log *zap.Logger,
) *PaymentService {
	return &PaymentService{
		providers:  providers,
		orderRepo:  orderRepo,
		refundRepo: refundRepo,
		notifyRepo: notifyRepo,
		log:        log,
	}
}

// CreatePayment 创建本地订单并调用渠道下单
func (s *PaymentService) CreatePayment(ctx context.Context, channel model.Channel, req payment.PrepayRequest, extra string) (*payment.PrepayResponse, *model.Order, error) {
	provider, ok := s.providers[channel]
	if !ok {
		return nil, nil, ErrUnknownChannel
	}

	order := &model.Order{
		OutTradeNo:  req.OutTradeNo,
		Channel:     channel,
		TradeType:   req.TradeType,
		Subject:     req.Subject,
		Description: req.Description,
		Amount:      req.Amount,
		Currency:    "CNY",
		Status:      model.StatusPending,
		PayerOpenID: req.PayerOpenID,
		ClientIP:    req.ClientIP,
		NotifyURL:   req.NotifyURL,
		ReturnURL:   req.ReturnURL,
		ExtraJSON:   extra,
	}
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, nil, fmt.Errorf("save order: %w", err)
	}

	resp, err := provider.Prepay(ctx, req)
	if err != nil {
		s.log.Error("prepay failed",
			zap.String("channel", string(channel)),
			zap.String("out_trade_no", req.OutTradeNo),
			zap.Error(err))
		return nil, order, err
	}
	return resp, order, nil
}

// QueryPayment 查询订单
//   - 优先调用渠道接口拿最新状态，并同步落库
func (s *PaymentService) QueryPayment(ctx context.Context, outTradeNo string) (*model.Order, *payment.QueryResponse, error) {
	order, err := s.orderRepo.GetByOutTradeNo(ctx, outTradeNo)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, ErrOrderNotFound
		}
		return nil, nil, err
	}
	provider, ok := s.providers[order.Channel]
	if !ok {
		return order, nil, ErrUnknownChannel
	}

	res, err := provider.Query(ctx, outTradeNo)
	if err != nil {
		return order, nil, err
	}

	if res.Status != order.Status {
		if err := s.orderRepo.UpdateStatus(ctx, outTradeNo, res.Status, res.ChannelTradeNo, res.PaidAt); err != nil {
			s.log.Warn("sync order status failed", zap.Error(err))
		} else {
			order.Status = res.Status
			if res.ChannelTradeNo != "" {
				order.ChannelTradeNo = res.ChannelTradeNo
			}
			order.PaidAt = res.PaidAt
		}
	}
	return order, res, nil
}

// Refund 退款
func (s *PaymentService) Refund(ctx context.Context, outTradeNo, outRefundNo string, refundAmount int64, reason string) (*payment.RefundResponse, error) {
	order, err := s.orderRepo.GetByOutTradeNo(ctx, outTradeNo)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	provider, ok := s.providers[order.Channel]
	if !ok {
		return nil, ErrUnknownChannel
	}

	rec := &model.Refund{
		OutTradeNo:   outTradeNo,
		OutRefundNo:  outRefundNo,
		Channel:      order.Channel,
		RefundAmount: refundAmount,
		TotalAmount:  order.Amount,
		Reason:       reason,
		Status:       "INIT",
	}
	if err := s.refundRepo.Create(ctx, rec); err != nil {
		return nil, fmt.Errorf("save refund: %w", err)
	}

	resp, err := provider.Refund(ctx, payment.RefundRequest{
		OutTradeNo:   outTradeNo,
		OutRefundNo:  outRefundNo,
		TotalAmount:  order.Amount,
		RefundAmount: refundAmount,
		Reason:       reason,
	})
	if err != nil {
		_ = s.refundRepo.UpdateStatus(ctx, outRefundNo, "FAILED", "")
		return nil, err
	}

	if err := s.refundRepo.UpdateStatus(ctx, outRefundNo, "SUCCESS", resp.ChannelRefundID); err != nil {
		s.log.Warn("update refund status failed", zap.Error(err))
	}
	if err := s.orderRepo.MarkRefunded(ctx, outTradeNo); err != nil {
		s.log.Warn("mark order refunded failed", zap.Error(err))
	}
	return resp, nil
}

// HandleNotify 处理回调
//   - 校验签名 → 写入回调流水 → 校验金额 → 更新订单
func (s *PaymentService) HandleNotify(ctx context.Context, channel model.Channel, r *http.Request) (payment.Provider, error) {
	provider, ok := s.providers[channel]
	if !ok {
		return nil, ErrUnknownChannel
	}

	result, err := provider.HandleNotify(ctx, r)
	s.logNotify(ctx, channel, result, err == nil)
	if err != nil {
		return provider, fmt.Errorf("handle notify: %w", err)
	}

	order, err := s.orderRepo.GetByOutTradeNo(ctx, result.OutTradeNo)
	if err != nil {
		return provider, fmt.Errorf("load order: %w", err)
	}
	if result.Amount > 0 && result.Amount != order.Amount {
		s.log.Error("notify amount mismatch",
			zap.String("out_trade_no", result.OutTradeNo),
			zap.Int64("expect", order.Amount),
			zap.Int64("got", result.Amount))
		return provider, ErrAmountMismatch
	}

	if err := s.orderRepo.UpdateStatus(ctx, result.OutTradeNo, result.Status, result.ChannelTradeNo, result.PaidAt); err != nil {
		return provider, fmt.Errorf("update order: %w", err)
	}
	return provider, nil
}

// HandleRefundNotify 处理退款回调
func (s *PaymentService) HandleRefundNotify(ctx context.Context, channel model.Channel, r *http.Request) (payment.Provider, error) {
	provider, ok := s.providers[channel]
	if !ok {
		return nil, ErrUnknownChannel
	}

	result, err := provider.HandleRefundNotify(ctx, r)
	s.logNotify(ctx, channel, result, err == nil)
	if err != nil {
		return provider, fmt.Errorf("handle refund notify: %w", err)
	}
	if result == nil || result.OutRefundNo == "" {
		return provider, fmt.Errorf("refund notify missing out_refund_no")
	}

	refund, err := s.refundRepo.GetByOutRefundNo(ctx, result.OutRefundNo)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return provider, ErrRefundNotFound
		}
		return provider, fmt.Errorf("load refund: %w", err)
	}

	nextRefundStatus := result.RefundStatus
	if nextRefundStatus == "" {
		nextRefundStatus = refund.Status
	}
	if err := s.refundRepo.UpdateStatus(ctx, result.OutRefundNo, nextRefundStatus, result.ChannelRefundNo); err != nil {
		return provider, fmt.Errorf("update refund: %w", err)
	}

	if result.OutTradeNo == "" {
		result.OutTradeNo = refund.OutTradeNo
	}
	if nextRefundStatus == "SUCCESS" {
		if err := s.orderRepo.MarkRefunded(ctx, result.OutTradeNo); err != nil {
			s.log.Warn("mark order refunded failed",
				zap.String("out_trade_no", result.OutTradeNo),
				zap.Error(err))
		}
	}
	return provider, nil
}

func (s *PaymentService) logNotify(ctx context.Context, channel model.Channel, result *payment.NotifyResult, verified bool) {
	logEntry := &model.PaymentNotifyLog{
		Channel:   channel,
		EventType: "",
		RawBody:   "",
		Verified:  verified,
	}
	if result != nil {
		logEntry.OutTradeNo = result.OutTradeNo
		logEntry.EventType = result.EventType
		logEntry.RawBody = string(result.RawBody)
	}
	if logErr := s.notifyRepo.Create(ctx, logEntry); logErr != nil {
		s.log.Warn("save notify log failed", zap.Error(logErr))
	}
}
