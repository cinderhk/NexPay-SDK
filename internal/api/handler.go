package api

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nexpay/nexpay-sdk/internal/model"
	"github.com/nexpay/nexpay-sdk/internal/payment"
	"github.com/nexpay/nexpay-sdk/internal/service"
	"github.com/nexpay/nexpay-sdk/internal/version"
	"github.com/nexpay/nexpay-sdk/pkg/response"
	"go.uber.org/zap"
)

type Handler struct {
	svc *service.PaymentService
	log *zap.Logger
}

func NewHandler(svc *service.PaymentService, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// CreatePaymentReq 统一下单请求
type CreatePaymentReq struct {
	Channel     string `json:"channel"      binding:"required,oneof=wechat alipay"`
	OutTradeNo  string `json:"out_trade_no" binding:"required,max=64"`
	Subject     string `json:"subject"      binding:"required,max=255"`
	Description string `json:"description"  binding:"max=512"`
	Amount      int64  `json:"amount"       binding:"required,min=1"` // 单位：分
	TradeType   string `json:"trade_type"   binding:"required"`
	PayerOpenID string `json:"payer_open_id"`
	ClientIP    string `json:"client_ip"`
	NotifyURL   string `json:"notify_url"`
	ReturnURL   string `json:"return_url"`
	Extra       string `json:"extra"`
	ExpireSecs  int    `json:"expire_secs"`
}

func (h *Handler) CreatePayment(c *gin.Context) {
	var req CreatePaymentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	channel := model.Channel(strings.ToLower(req.Channel))
	tradeType := model.TradeType(strings.ToUpper(req.TradeType))

	pReq := payment.PrepayRequest{
		OutTradeNo:  req.OutTradeNo,
		Subject:     req.Subject,
		Description: req.Description,
		Amount:      req.Amount,
		TradeType:   tradeType,
		ClientIP:    fallbackIP(req.ClientIP, c.ClientIP()),
		NotifyURL:   req.NotifyURL,
		ReturnURL:   req.ReturnURL,
		PayerOpenID: req.PayerOpenID,
	}
	if req.ExpireSecs > 0 {
		pReq.ExpireAt = time.Now().Add(time.Duration(req.ExpireSecs) * time.Second)
	}

	resp, order, err := h.svc.CreatePayment(c.Request.Context(), channel, pReq, req.Extra)
	if err != nil {
		if errors.Is(err, service.ErrUnknownChannel) || errors.Is(err, payment.ErrChannelDisabled) {
			response.BadRequest(c, err.Error())
			return
		}
		if errors.Is(err, payment.ErrUnsupportedTradeType) {
			response.BadRequest(c, "unsupported trade_type for the channel")
			return
		}
		response.Upstream(c, err.Error())
		return
	}
	response.OK(c, gin.H{"order": order, "pay": resp})
}

func (h *Handler) QueryPayment(c *gin.Context) {
	outTradeNo := c.Param("out_trade_no")
	if outTradeNo == "" {
		response.BadRequest(c, "out_trade_no is required")
		return
	}
	order, query, err := h.svc.QueryPayment(c.Request.Context(), outTradeNo)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			response.Fail(c, 404, response.CodeNotFound, err.Error())
			return
		}
		response.Upstream(c, err.Error())
		return
	}
	response.OK(c, gin.H{"order": order, "channel_status": query})
}

type RefundReq struct {
	OutTradeNo   string `json:"out_trade_no"  binding:"required"`
	OutRefundNo  string `json:"out_refund_no" binding:"required"`
	RefundAmount int64  `json:"refund_amount" binding:"required,min=1"`
	Reason       string `json:"reason"`
}

func (h *Handler) Refund(c *gin.Context) {
	var req RefundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.svc.Refund(c.Request.Context(), req.OutTradeNo, req.OutRefundNo, req.RefundAmount, req.Reason)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			response.Fail(c, 404, response.CodeNotFound, err.Error())
			return
		}
		response.Upstream(c, err.Error())
		return
	}
	response.OK(c, resp)
}

func (h *Handler) WeChatNotify(c *gin.Context) {
	h.handleNotify(c, model.ChannelWeChat)
}

func (h *Handler) AlipayNotify(c *gin.Context) {
	h.handleNotify(c, model.ChannelAlipay)
}

func (h *Handler) WeChatRefundNotify(c *gin.Context) {
	h.handleRefundNotify(c, model.ChannelWeChat)
}

func (h *Handler) AlipayRefundNotify(c *gin.Context) {
	h.handleRefundNotify(c, model.ChannelAlipay)
}

func (h *Handler) handleNotify(c *gin.Context, channel model.Channel) {
	provider, err := h.svc.HandleNotify(c.Request.Context(), channel, c.Request)
	if err != nil {
		h.log.Error("notify failed",
			zap.String("channel", string(channel)),
			zap.Error(err))
		c.String(400, err.Error())
		return
	}
	provider.AckNotify(c.Writer)
}

func (h *Handler) handleRefundNotify(c *gin.Context, channel model.Channel) {
	provider, err := h.svc.HandleRefundNotify(c.Request.Context(), channel, c.Request)
	if err != nil {
		h.log.Error("refund notify failed",
			zap.String("channel", string(channel)),
			zap.Error(err))
		c.String(400, err.Error())
		return
	}
	provider.AckNotify(c.Writer)
}

func (h *Handler) Health(c *gin.Context) {
	response.OK(c, gin.H{
		"status":  "ok",
		"ts":      time.Now().Unix(),
		"version": version.Get(),
	})
}

func fallbackIP(provided, gin string) string {
	if provided != "" {
		return provided
	}
	return gin
}
