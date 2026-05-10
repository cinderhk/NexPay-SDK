package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nexpay/nexpay-sdk/internal/config"
	"github.com/nexpay/nexpay-sdk/internal/model"
	"github.com/nexpay/nexpay-sdk/internal/payment"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/app"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/h5"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

type Provider struct {
	cfg             config.WeChatConfig
	notifyURL       string
	refundNotifyURL string
	client          *core.Client
	handler         *notify.Handler
}

// New 构建微信支付 Provider
//   - notifyBase: 公网回调前缀（如 https://example.com）
func New(ctx context.Context, cfg config.WeChatConfig, notifyBase string) (*Provider, error) {
	if !cfg.Enabled {
		return nil, payment.ErrChannelDisabled
	}

	mchPrivateKey, err := utils.LoadPrivateKeyWithPath(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load wechat private key: %w", err)
	}

	opts := []core.ClientOption{
		option.WithWechatPayAutoAuthCipher(
			cfg.MchID, cfg.SerialNo, mchPrivateKey, cfg.APIv3Key,
		),
	}
	cli, err := core.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("init wechat client: %w", err)
	}

	verifier := buildVerifier(cfg.MchID)
	handler, err := notify.NewRSANotifyHandler(cfg.APIv3Key, verifier)
	if err != nil {
		return nil, fmt.Errorf("init wechat notify handler: %w", err)
	}

	return &Provider{
		cfg:             cfg,
		notifyURL:       notifyBase + cfg.NotifyPath,
		refundNotifyURL: notifyBase + fallbackPath(cfg.RefundNotifyPath, cfg.NotifyPath),
		client:          cli,
		handler:         handler,
	}, nil
}

func (p *Provider) Channel() model.Channel { return model.ChannelWeChat }

// Prepay 根据 TradeType 路由到 NATIVE / JSAPI / APP / H5
func (p *Provider) Prepay(ctx context.Context, req payment.PrepayRequest) (*payment.PrepayResponse, error) {
	notifyURL := req.NotifyURL
	if notifyURL == "" {
		notifyURL = p.notifyURL
	}
	expire := req.ExpireAt
	if expire.IsZero() {
		expire = time.Now().Add(15 * time.Minute)
	}

	switch req.TradeType {
	case model.TradeNative:
		svc := native.NativeApiService{Client: p.client}
		resp, _, err := svc.Prepay(ctx, native.PrepayRequest{
			Appid:       core.String(p.cfg.AppID),
			Mchid:       core.String(p.cfg.MchID),
			Description: core.String(req.Subject),
			OutTradeNo:  core.String(req.OutTradeNo),
			TimeExpire:  core.Time(expire),
			NotifyUrl:   core.String(notifyURL),
			Amount: &native.Amount{
				Total:    core.Int64(req.Amount),
				Currency: core.String("CNY"),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("wechat native prepay: %w", err)
		}
		return &payment.PrepayResponse{
			Channel:    model.ChannelWeChat,
			OutTradeNo: req.OutTradeNo,
			TradeType:  req.TradeType,
			QRCodeURL:  derefStr(resp.CodeUrl),
		}, nil

	case model.TradeJSAPI:
		if req.PayerOpenID == "" {
			return nil, fmt.Errorf("wechat JSAPI requires payer_open_id")
		}
		svc := jsapi.JsapiApiService{Client: p.client}
		resp, _, err := svc.PrepayWithRequestPayment(ctx, jsapi.PrepayRequest{
			Appid:       core.String(p.cfg.AppID),
			Mchid:       core.String(p.cfg.MchID),
			Description: core.String(req.Subject),
			OutTradeNo:  core.String(req.OutTradeNo),
			TimeExpire:  core.Time(expire),
			NotifyUrl:   core.String(notifyURL),
			Amount: &jsapi.Amount{
				Total:    core.Int64(req.Amount),
				Currency: core.String("CNY"),
			},
			Payer: &jsapi.Payer{Openid: core.String(req.PayerOpenID)},
		})
		if err != nil {
			return nil, fmt.Errorf("wechat jsapi prepay: %w", err)
		}
		return &payment.PrepayResponse{
			Channel:    model.ChannelWeChat,
			OutTradeNo: req.OutTradeNo,
			TradeType:  req.TradeType,
			JSAPIParams: map[string]string{
				"appId":     derefStr(resp.Appid),
				"timeStamp": derefStr(resp.TimeStamp),
				"nonceStr":  derefStr(resp.NonceStr),
				"package":   derefStr(resp.Package),
				"signType":  derefStr(resp.SignType),
				"paySign":   derefStr(resp.PaySign),
			},
		}, nil

	case model.TradeApp:
		svc := app.AppApiService{Client: p.client}
		resp, _, err := svc.PrepayWithRequestPayment(ctx, app.PrepayRequest{
			Appid:       core.String(p.cfg.AppID),
			Mchid:       core.String(p.cfg.MchID),
			Description: core.String(req.Subject),
			OutTradeNo:  core.String(req.OutTradeNo),
			TimeExpire:  core.Time(expire),
			NotifyUrl:   core.String(notifyURL),
			Amount: &app.Amount{
				Total:    core.Int64(req.Amount),
				Currency: core.String("CNY"),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("wechat app prepay: %w", err)
		}
		return &payment.PrepayResponse{
			Channel:    model.ChannelWeChat,
			OutTradeNo: req.OutTradeNo,
			TradeType:  req.TradeType,
			AppParams: map[string]string{
				"appid":     p.cfg.AppID,
				"partnerid": derefStr(resp.PartnerId),
				"prepayid":  derefStr(resp.PrepayId),
				"package":   derefStr(resp.Package),
				"noncestr":  derefStr(resp.NonceStr),
				"timestamp": derefStr(resp.TimeStamp),
				"sign":      derefStr(resp.Sign),
			},
		}, nil

	case model.TradeH5:
		svc := h5.H5ApiService{Client: p.client}
		clientIP := req.ClientIP
		if clientIP == "" {
			clientIP = "127.0.0.1"
		}
		resp, _, err := svc.Prepay(ctx, h5.PrepayRequest{
			Appid:       core.String(p.cfg.AppID),
			Mchid:       core.String(p.cfg.MchID),
			Description: core.String(req.Subject),
			OutTradeNo:  core.String(req.OutTradeNo),
			TimeExpire:  core.Time(expire),
			NotifyUrl:   core.String(notifyURL),
			Amount: &h5.Amount{
				Total:    core.Int64(req.Amount),
				Currency: core.String("CNY"),
			},
			SceneInfo: &h5.SceneInfo{
				PayerClientIp: core.String(clientIP),
				H5Info:        &h5.H5Info{Type: core.String("Wap")},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("wechat h5 prepay: %w", err)
		}
		return &payment.PrepayResponse{
			Channel:    model.ChannelWeChat,
			OutTradeNo: req.OutTradeNo,
			TradeType:  req.TradeType,
			MwebURL:    derefStr(resp.H5Url),
		}, nil

	default:
		return nil, payment.ErrUnsupportedTradeType
	}
}

func (p *Provider) Query(ctx context.Context, outTradeNo string) (*payment.QueryResponse, error) {
	svc := native.NativeApiService{Client: p.client}
	resp, _, err := svc.QueryOrderByOutTradeNo(ctx, native.QueryOrderByOutTradeNoRequest{
		OutTradeNo: core.String(outTradeNo),
		Mchid:      core.String(p.cfg.MchID),
	})
	if err != nil {
		return nil, fmt.Errorf("wechat query: %w", err)
	}

	status := mapWechatStatus(derefStr(resp.TradeState))
	var paidAt *time.Time
	if resp.SuccessTime != nil {
		if t, err := time.Parse(time.RFC3339, *resp.SuccessTime); err == nil {
			paidAt = &t
		}
	}
	var amount int64
	if resp.Amount != nil && resp.Amount.Total != nil {
		amount = *resp.Amount.Total
	}
	return &payment.QueryResponse{
		OutTradeNo:     outTradeNo,
		ChannelTradeNo: derefStr(resp.TransactionId),
		Status:         status,
		Amount:         amount,
		PaidAt:         paidAt,
	}, nil
}

func (p *Provider) Refund(ctx context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	notifyURL := req.NotifyURL
	if notifyURL == "" {
		notifyURL = p.refundNotifyURL
	}
	svc := refunddomestic.RefundsApiService{Client: p.client}
	resp, _, err := svc.Create(ctx, refunddomestic.CreateRequest{
		OutTradeNo:  core.String(req.OutTradeNo),
		OutRefundNo: core.String(req.OutRefundNo),
		Reason:      core.String(req.Reason),
		NotifyUrl:   core.String(notifyURL),
		Amount: &refunddomestic.AmountReq{
			Refund:   core.Int64(req.RefundAmount),
			Total:    core.Int64(req.TotalAmount),
			Currency: core.String("CNY"),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("wechat refund: %w", err)
	}
	return &payment.RefundResponse{
		OutRefundNo:     req.OutRefundNo,
		ChannelRefundID: derefStr(resp.RefundId),
		Status:          string(*resp.Status),
	}, nil
}

func (p *Provider) HandleNotify(ctx context.Context, r *http.Request) (*payment.NotifyResult, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read notify body: %w", err)
	}
	r.Body = io.NopCloser(bytesReader(body))

	transaction := new(wxTransaction)
	notifyReq, err := p.handler.ParseNotifyRequest(ctx, r, transaction)
	if err != nil {
		return nil, fmt.Errorf("parse wechat notify: %w", err)
	}

	status := mapWechatStatus(transaction.TradeState)
	var paidAt *time.Time
	if transaction.SuccessTime != "" {
		if t, err := time.Parse(time.RFC3339, transaction.SuccessTime); err == nil {
			paidAt = &t
		}
	}
	return &payment.NotifyResult{
		OutTradeNo:     transaction.OutTradeNo,
		ChannelTradeNo: transaction.TransactionID,
		Status:         status,
		Amount:         transaction.Amount.Total,
		PaidAt:         paidAt,
		EventType:      notifyReq.EventType,
		RawBody:        body,
		IsRefund:       false,
	}, nil
}

func (p *Provider) HandleRefundNotify(ctx context.Context, r *http.Request) (*payment.NotifyResult, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read refund notify body: %w", err)
	}
	r.Body = io.NopCloser(bytesReader(body))

	refund := new(wxRefund)
	notifyReq, err := p.handler.ParseNotifyRequest(ctx, r, refund)
	if err != nil {
		return nil, fmt.Errorf("parse wechat refund notify: %w", err)
	}

	return &payment.NotifyResult{
		OutTradeNo:      refund.OutTradeNo,
		OutRefundNo:     refund.OutRefundNo,
		ChannelRefundNo: refund.RefundID,
		RefundStatus:    mapWechatRefundStatus(refund.RefundStatus),
		Amount:          refund.Amount.Refund,
		EventType:       notifyReq.EventType,
		RawBody:         body,
		IsRefund:        true,
	}, nil
}

func (p *Provider) AckNotify(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": "SUCCESS", "message": "OK"})
}

// wxTransaction 用于解析回调 resource.ciphertext 中的字段（仅取我们需要的）
type wxTransaction struct {
	TransactionID string `json:"transaction_id"`
	OutTradeNo    string `json:"out_trade_no"`
	TradeState    string `json:"trade_state"`
	SuccessTime   string `json:"success_time"`
	Amount        struct {
		Total int64 `json:"total"`
	} `json:"amount"`
}

type wxRefund struct {
	OutTradeNo   string `json:"out_trade_no"`
	OutRefundNo  string `json:"out_refund_no"`
	RefundID     string `json:"refund_id"`
	RefundStatus string `json:"refund_status"`
	Amount       struct {
		Refund int64 `json:"refund"`
		Total  int64 `json:"total"`
	} `json:"amount"`
}

func mapWechatStatus(state string) model.OrderStatus {
	switch state {
	case "SUCCESS":
		return model.StatusPaid
	case "REFUND":
		return model.StatusRefunded
	case "CLOSED", "REVOKED":
		return model.StatusClosed
	case "PAYERROR":
		return model.StatusFailed
	default:
		return model.StatusPending
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func mapWechatRefundStatus(s string) string {
	switch s {
	case "SUCCESS":
		return "SUCCESS"
	case "CLOSED":
		return "CLOSED"
	case "ABNORMAL":
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
