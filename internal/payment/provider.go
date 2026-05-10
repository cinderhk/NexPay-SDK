package payment

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/nexpay/nexpay-sdk/internal/model"
)

var (
	ErrUnsupportedTradeType = errors.New("unsupported trade type")
	ErrChannelDisabled      = errors.New("payment channel disabled")
	ErrInvalidNotify        = errors.New("invalid notify")
)

// PrepayRequest 统一的下单请求
type PrepayRequest struct {
	OutTradeNo  string
	Subject     string
	Description string
	Amount      int64 // 单位：分
	TradeType   model.TradeType
	ClientIP    string
	NotifyURL   string
	ReturnURL   string
	PayerOpenID string // wechat JSAPI 必填
	ExpireAt    time.Time
}

// PrepayResponse 统一的下单返回
//
// 不同渠道/交易类型返回字段不同：
//   - wechat NATIVE  : QRCodeURL
//   - wechat JSAPI   : JSAPIParams
//   - wechat APP     : AppParams
//   - wechat H5      : MwebURL
//   - alipay PAGE/WAP: PayURL（直接重定向）
//   - alipay APP     : OrderString（客户端拼装）
//   - alipay QR      : QRCodeURL
type PrepayResponse struct {
	Channel     model.Channel          `json:"channel"`
	OutTradeNo  string                 `json:"out_trade_no"`
	TradeType   model.TradeType        `json:"trade_type"`
	QRCodeURL   string                 `json:"qr_code_url,omitempty"`
	MwebURL     string                 `json:"mweb_url,omitempty"`
	PayURL      string                 `json:"pay_url,omitempty"`
	OrderString string                 `json:"order_string,omitempty"`
	JSAPIParams map[string]string      `json:"jsapi_params,omitempty"`
	AppParams   map[string]string      `json:"app_params,omitempty"`
	Raw         map[string]interface{} `json:"raw,omitempty"`
}

// QueryResponse 统一查询返回
type QueryResponse struct {
	OutTradeNo     string            `json:"out_trade_no"`
	ChannelTradeNo string            `json:"channel_trade_no"`
	Status         model.OrderStatus `json:"status"`
	Amount         int64             `json:"amount"`
	PaidAt         *time.Time        `json:"paid_at,omitempty"`
}

// RefundRequest 统一退款请求
type RefundRequest struct {
	OutTradeNo   string
	OutRefundNo  string
	TotalAmount  int64
	RefundAmount int64
	Reason       string
	NotifyURL    string
}

// RefundResponse 统一退款返回
type RefundResponse struct {
	OutRefundNo     string `json:"out_refund_no"`
	ChannelRefundID string `json:"channel_refund_id"`
	Status          string `json:"status"`
}

// NotifyResult 回调解析结果
type NotifyResult struct {
	OutTradeNo      string
	OutRefundNo     string
	ChannelTradeNo  string
	ChannelRefundNo string
	Status          model.OrderStatus
	RefundStatus    string
	Amount          int64
	PaidAt          *time.Time
	EventType       string
	RawBody         []byte
	IsRefund        bool
}

// Provider 支付渠道抽象接口
type Provider interface {
	Channel() model.Channel
	Prepay(ctx context.Context, req PrepayRequest) (*PrepayResponse, error)
	Query(ctx context.Context, outTradeNo string) (*QueryResponse, error)
	Refund(ctx context.Context, req RefundRequest) (*RefundResponse, error)
	// HandleNotify 解析并校验异步通知，校验失败需返回错误
	HandleNotify(ctx context.Context, r *http.Request) (*NotifyResult, error)
	// HandleRefundNotify 解析并校验退款异步通知
	HandleRefundNotify(ctx context.Context, r *http.Request) (*NotifyResult, error)
	// AckNotify 给第三方平台返回成功应答（写入 ResponseWriter）
	AckNotify(w http.ResponseWriter)
}
