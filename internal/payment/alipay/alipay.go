package alipay

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/nexpay/nexpay-sdk/internal/config"
	"github.com/nexpay/nexpay-sdk/internal/model"
	"github.com/nexpay/nexpay-sdk/internal/payment"
	"github.com/smartwalle/alipay/v3"
)

type Provider struct {
	cfg             config.AlipayConfig
	notifyURL       string
	refundNotifyURL string
	returnURL       string
	client          *alipay.Client
}

func New(cfg config.AlipayConfig, notifyBase string) (*Provider, error) {
	if !cfg.Enabled {
		return nil, payment.ErrChannelDisabled
	}
	cli, err := alipay.New(cfg.AppID, cfg.PrivateKey, cfg.IsProduction)
	if err != nil {
		return nil, fmt.Errorf("init alipay client: %w", err)
	}
	if err := cli.LoadAliPayPublicKey(cfg.AliPublicKey); err != nil {
		return nil, fmt.Errorf("load alipay public key: %w", err)
	}
	return &Provider{
		cfg:             cfg,
		notifyURL:       notifyBase + cfg.NotifyPath,
		refundNotifyURL: notifyBase + fallbackPath(cfg.RefundNotifyPath, cfg.NotifyPath),
		returnURL:       cfg.ReturnURL,
		client:          cli,
	}, nil
}

func (p *Provider) Channel() model.Channel { return model.ChannelAlipay }

func (p *Provider) Prepay(ctx context.Context, req payment.PrepayRequest) (*payment.PrepayResponse, error) {
	notifyURL := req.NotifyURL
	if notifyURL == "" {
		notifyURL = p.notifyURL
	}
	returnURL := req.ReturnURL
	if returnURL == "" {
		returnURL = p.returnURL
	}

	// 支付宝金额单位是元，做整分到元的转换（保留两位）
	totalAmount := fmt.Sprintf("%.2f", float64(req.Amount)/100.0)

	switch req.TradeType {
	case model.TradePage:
		body := alipay.TradePagePay{}
		body.NotifyURL = notifyURL
		body.ReturnURL = returnURL
		body.Subject = req.Subject
		body.OutTradeNo = req.OutTradeNo
		body.TotalAmount = totalAmount
		body.Body = req.Description
		body.ProductCode = "FAST_INSTANT_TRADE_PAY"

		u, err := p.client.TradePagePay(body)
		if err != nil {
			return nil, fmt.Errorf("alipay page pay: %w", err)
		}
		return &payment.PrepayResponse{
			Channel:    model.ChannelAlipay,
			OutTradeNo: req.OutTradeNo,
			TradeType:  req.TradeType,
			PayURL:     u.String(),
		}, nil

	case model.TradeWap:
		body := alipay.TradeWapPay{}
		body.NotifyURL = notifyURL
		body.ReturnURL = returnURL
		body.Subject = req.Subject
		body.OutTradeNo = req.OutTradeNo
		body.TotalAmount = totalAmount
		body.Body = req.Description
		body.ProductCode = "QUICK_WAP_WAY"

		u, err := p.client.TradeWapPay(body)
		if err != nil {
			return nil, fmt.Errorf("alipay wap pay: %w", err)
		}
		return &payment.PrepayResponse{
			Channel:    model.ChannelAlipay,
			OutTradeNo: req.OutTradeNo,
			TradeType:  req.TradeType,
			PayURL:     u.String(),
		}, nil

	case model.TradeApp:
		body := alipay.TradeAppPay{}
		body.NotifyURL = notifyURL
		body.Subject = req.Subject
		body.OutTradeNo = req.OutTradeNo
		body.TotalAmount = totalAmount
		body.Body = req.Description
		body.ProductCode = "QUICK_MSECURITY_PAY"

		s, err := p.client.TradeAppPay(body)
		if err != nil {
			return nil, fmt.Errorf("alipay app pay: %w", err)
		}
		return &payment.PrepayResponse{
			Channel:     model.ChannelAlipay,
			OutTradeNo:  req.OutTradeNo,
			TradeType:   req.TradeType,
			OrderString: s,
		}, nil

	case model.TradeQR:
		body := alipay.TradePreCreate{}
		body.NotifyURL = notifyURL
		body.Subject = req.Subject
		body.OutTradeNo = req.OutTradeNo
		body.TotalAmount = totalAmount
		body.Body = req.Description

		resp, err := p.client.TradePreCreate(ctx, body)
		if err != nil {
			return nil, fmt.Errorf("alipay precreate: %w", err)
		}
		if resp.Code != alipay.CodeSuccess {
			return nil, fmt.Errorf("alipay precreate fail: %s %s", resp.Code, resp.Msg)
		}
		return &payment.PrepayResponse{
			Channel:    model.ChannelAlipay,
			OutTradeNo: req.OutTradeNo,
			TradeType:  req.TradeType,
			QRCodeURL:  resp.QRCode,
		}, nil

	default:
		return nil, payment.ErrUnsupportedTradeType
	}
}

func (p *Provider) Query(ctx context.Context, outTradeNo string) (*payment.QueryResponse, error) {
	body := alipay.TradeQuery{OutTradeNo: outTradeNo}
	resp, err := p.client.TradeQuery(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("alipay query: %w", err)
	}
	if resp.Code != alipay.CodeSuccess {
		return nil, fmt.Errorf("alipay query fail: %s %s", resp.Code, resp.Msg)
	}
	status := mapAlipayStatus(string(resp.TradeStatus))
	var paidAt *time.Time
	if resp.SendPayDate != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", resp.SendPayDate, time.Local); err == nil {
			paidAt = &t
		}
	}
	amount, _ := parseAmount(resp.TotalAmount)
	return &payment.QueryResponse{
		OutTradeNo:     outTradeNo,
		ChannelTradeNo: resp.TradeNo,
		Status:         status,
		Amount:         amount,
		PaidAt:         paidAt,
	}, nil
}

func (p *Provider) Refund(ctx context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	body := alipay.TradeRefund{
		OutTradeNo:   req.OutTradeNo,
		RefundAmount: fmt.Sprintf("%.2f", float64(req.RefundAmount)/100.0),
		RefundReason: req.Reason,
		OutRequestNo: req.OutRefundNo,
	}
	resp, err := p.client.TradeRefund(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("alipay refund: %w", err)
	}
	if resp.Code != alipay.CodeSuccess {
		return nil, fmt.Errorf("alipay refund fail: %s %s", resp.Code, resp.Msg)
	}
	return &payment.RefundResponse{
		OutRefundNo:     req.OutRefundNo,
		ChannelRefundID: resp.TradeNo,
		Status:          "SUCCESS",
	}, nil
}

func (p *Provider) HandleNotify(_ context.Context, r *http.Request) (*payment.NotifyResult, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("parse form: %w", err)
	}
	noti, err := p.client.GetTradeNotification(r)
	if err != nil {
		return nil, fmt.Errorf("verify alipay notify: %w", err)
	}

	status := mapAlipayStatus(string(noti.TradeStatus))
	var paidAt *time.Time
	if noti.GmtPayment != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", noti.GmtPayment, time.Local); err == nil {
			paidAt = &t
		}
	}
	amount, _ := parseAmount(noti.TotalAmount)
	rawBody := []byte(r.Form.Encode())

	return &payment.NotifyResult{
		OutTradeNo:     noti.OutTradeNo,
		ChannelTradeNo: noti.TradeNo,
		Status:         status,
		Amount:         amount,
		PaidAt:         paidAt,
		EventType:      noti.NotifyType,
		RawBody:        rawBody,
		IsRefund:       false,
	}, nil
}

func (p *Provider) HandleRefundNotify(ctx context.Context, r *http.Request) (*payment.NotifyResult, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("parse form: %w", err)
	}
	if err := p.client.VerifySign(ctx, r.Form); err != nil {
		return nil, fmt.Errorf("verify alipay refund notify: %w", err)
	}

	outTradeNo := r.Form.Get("out_trade_no")
	outRefundNo := r.Form.Get("out_biz_no")
	channelRefundNo := r.Form.Get("trade_no")
	refundStatus := mapAlipayRefundStatus(r.Form.Get("refund_status"))
	amount, _ := parseAmount(r.Form.Get("refund_fee"))

	eventType := r.Form.Get("notify_type")
	if eventType == "" {
		eventType = "trade_refund"
	}
	return &payment.NotifyResult{
		OutTradeNo:      outTradeNo,
		OutRefundNo:     outRefundNo,
		ChannelRefundNo: channelRefundNo,
		RefundStatus:    refundStatus,
		Amount:          amount,
		EventType:       eventType,
		RawBody:         []byte(r.Form.Encode()),
		IsRefund:        true,
	}, nil
}

func (p *Provider) AckNotify(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))
}

func mapAlipayStatus(s string) model.OrderStatus {
	switch s {
	case "TRADE_SUCCESS", "TRADE_FINISHED":
		return model.StatusPaid
	case "TRADE_CLOSED":
		return model.StatusClosed
	case "WAIT_BUYER_PAY":
		return model.StatusPending
	default:
		return model.StatusPending
	}
}

// parseAmount 元 -> 分
func parseAmount(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	var yuan float64
	if _, err := fmt.Sscanf(s, "%f", &yuan); err != nil {
		return 0, err
	}
	return int64(yuan*100 + 0.5), nil
}

func mapAlipayRefundStatus(s string) string {
	switch s {
	case "REFUND_SUCCESS", "SUCCESS":
		return "SUCCESS"
	case "REFUND_CLOSED", "CLOSED":
		return "CLOSED"
	case "REFUND_FAIL", "FAILED":
		return "FAILED"
	default:
		return "PROCESSING"
	}
}

func fallbackPath(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}
