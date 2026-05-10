package model

import "time"

// Channel 支付渠道
type Channel string

const (
	ChannelWeChat Channel = "wechat"
	ChannelAlipay Channel = "alipay"
)

// TradeType 交易类型（不同渠道含义略有不同）
//   - wechat: NATIVE / JSAPI / APP / H5
//   - alipay: PAGE  / WAP   / APP / QR
type TradeType string

const (
	TradeNative TradeType = "NATIVE"
	TradeJSAPI  TradeType = "JSAPI"
	TradeApp    TradeType = "APP"
	TradeH5     TradeType = "H5"
	TradePage   TradeType = "PAGE"
	TradeWap    TradeType = "WAP"
	TradeQR     TradeType = "QR"
)

// OrderStatus 订单状态
type OrderStatus string

const (
	StatusPending  OrderStatus = "PENDING"  // 已创建，等待支付
	StatusPaid     OrderStatus = "PAID"     // 已支付成功
	StatusFailed   OrderStatus = "FAILED"   // 支付失败
	StatusClosed   OrderStatus = "CLOSED"   // 已关闭
	StatusRefunded OrderStatus = "REFUNDED" // 已退款（含全部和部分退款）
)

// Order 内部业务订单
type Order struct {
	ID             uint64      `gorm:"primaryKey;autoIncrement"                              json:"id"`
	OutTradeNo     string      `gorm:"type:varchar(64);uniqueIndex;not null"                 json:"out_trade_no"`
	Channel        Channel     `gorm:"type:varchar(16);index;not null"                       json:"channel"`
	TradeType      TradeType   `gorm:"type:varchar(16);not null"                             json:"trade_type"`
	Subject        string      `gorm:"type:varchar(255);not null"                            json:"subject"`
	Description    string      `gorm:"type:varchar(512)"                                     json:"description,omitempty"`
	Amount         int64       `gorm:"not null"                                              json:"amount"` // 金额（分）
	Currency       string      `gorm:"type:varchar(8);not null;default:'CNY'"                json:"currency"`
	Status         OrderStatus `gorm:"type:varchar(16);index;not null;default:'PENDING'"     json:"status"`
	ChannelTradeNo string      `gorm:"type:varchar(64);index"                                json:"channel_trade_no,omitempty"`
	PayerOpenID    string      `gorm:"type:varchar(128)"                                     json:"payer_open_id,omitempty"`
	ClientIP       string      `gorm:"type:varchar(64)"                                      json:"client_ip,omitempty"`
	NotifyURL      string      `gorm:"type:varchar(255)"                                     json:"notify_url,omitempty"`
	ReturnURL      string      `gorm:"type:varchar(255)"                                     json:"return_url,omitempty"`
	ExtraJSON      string      `gorm:"type:text"                                             json:"extra,omitempty"` // 额外业务字段（JSON）
	PaidAt         *time.Time  `json:"paid_at,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

func (Order) TableName() string { return "orders" }

// Refund 退款记录
type Refund struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"                  json:"id"`
	OutTradeNo     string    `gorm:"type:varchar(64);index;not null"           json:"out_trade_no"`
	OutRefundNo    string    `gorm:"type:varchar(64);uniqueIndex;not null"     json:"out_refund_no"`
	Channel        Channel   `gorm:"type:varchar(16);not null"                 json:"channel"`
	RefundAmount   int64     `gorm:"not null"                                  json:"refund_amount"`
	TotalAmount    int64     `gorm:"not null"                                  json:"total_amount"`
	Reason         string    `gorm:"type:varchar(255)"                         json:"reason,omitempty"`
	Status         string    `gorm:"type:varchar(16);not null;default:'INIT'"  json:"status"`
	ChannelRefundID string   `gorm:"type:varchar(64);index"                    json:"channel_refund_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (Refund) TableName() string { return "refunds" }

// PaymentNotifyLog 支付/退款回调流水（用于审计与排障）
type PaymentNotifyLog struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"             json:"id"`
	Channel    Channel   `gorm:"type:varchar(16);index;not null"      json:"channel"`
	EventType  string    `gorm:"type:varchar(64);index;not null"      json:"event_type"`
	OutTradeNo string    `gorm:"type:varchar(64);index"               json:"out_trade_no,omitempty"`
	RawHeaders string    `gorm:"type:text"                            json:"raw_headers,omitempty"`
	RawBody    string    `gorm:"type:longtext"                        json:"raw_body,omitempty"`
	Verified   bool      `gorm:"not null;default:false"               json:"verified"`
	CreatedAt  time.Time `json:"created_at"`
}

func (PaymentNotifyLog) TableName() string { return "payment_notify_logs" }
