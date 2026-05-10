package repository

import (
	"context"
	"errors"
	"time"

	"github.com/nexpay/nexpay-sdk/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrNotFound = errors.New("record not found")

type OrderRepository interface {
	Create(ctx context.Context, order *model.Order) error
	GetByOutTradeNo(ctx context.Context, outTradeNo string) (*model.Order, error)
	UpdateStatus(ctx context.Context, outTradeNo string, status model.OrderStatus, channelTradeNo string, paidAt *time.Time) error
	MarkRefunded(ctx context.Context, outTradeNo string) error
}

type RefundRepository interface {
	Create(ctx context.Context, r *model.Refund) error
	GetByOutRefundNo(ctx context.Context, outRefundNo string) (*model.Refund, error)
	UpdateStatus(ctx context.Context, outRefundNo, status, channelRefundID string) error
}

type NotifyLogRepository interface {
	Create(ctx context.Context, log *model.PaymentNotifyLog) error
}

type orderRepo struct{ db *gorm.DB }

func NewOrderRepository(db *gorm.DB) OrderRepository { return &orderRepo{db: db} }

func (r *orderRepo) Create(ctx context.Context, order *model.Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *orderRepo) GetByOutTradeNo(ctx context.Context, outTradeNo string) (*model.Order, error) {
	var o model.Order
	err := r.db.WithContext(ctx).Where("out_trade_no = ?", outTradeNo).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *orderRepo) UpdateStatus(ctx context.Context, outTradeNo string, status model.OrderStatus, channelTradeNo string, paidAt *time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order model.Order
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("out_trade_no = ?", outTradeNo).
			First(&order).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}

		nextStatus := strongerStatus(order.Status, status)
		updates := map[string]interface{}{
			"status": nextStatus,
		}
		if channelTradeNo != "" {
			updates["channel_trade_no"] = channelTradeNo
		}
		if paidAt != nil {
			updates["paid_at"] = paidAt
		}

		return tx.Model(&model.Order{}).
			Where("id = ?", order.ID).
			Updates(updates).Error
	})
}

func (r *orderRepo) MarkRefunded(ctx context.Context, outTradeNo string) error {
	return r.UpdateStatus(ctx, outTradeNo, model.StatusRefunded, "", nil)
}

func strongerStatus(current, incoming model.OrderStatus) model.OrderStatus {
	if statusRank(incoming) > statusRank(current) {
		return incoming
	}
	return current
}

func statusRank(s model.OrderStatus) int {
	switch s {
	case model.StatusPending:
		return 10
	case model.StatusFailed:
		return 20
	case model.StatusClosed:
		return 30
	case model.StatusPaid:
		return 40
	case model.StatusRefunded:
		return 50
	default:
		return 0
	}
}

type refundRepo struct{ db *gorm.DB }

func NewRefundRepository(db *gorm.DB) RefundRepository { return &refundRepo{db: db} }

func (r *refundRepo) Create(ctx context.Context, refund *model.Refund) error {
	return r.db.WithContext(ctx).Create(refund).Error
}

func (r *refundRepo) GetByOutRefundNo(ctx context.Context, no string) (*model.Refund, error) {
	var rf model.Refund
	err := r.db.WithContext(ctx).Where("out_refund_no = ?", no).First(&rf).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rf, nil
}

func (r *refundRepo) UpdateStatus(ctx context.Context, outRefundNo, status, channelRefundID string) error {
	updates := map[string]interface{}{"status": status}
	if channelRefundID != "" {
		updates["channel_refund_id"] = channelRefundID
	}
	return r.db.WithContext(ctx).
		Model(&model.Refund{}).
		Where("out_refund_no = ?", outRefundNo).
		Updates(updates).Error
}

type notifyLogRepo struct{ db *gorm.DB }

func NewNotifyLogRepository(db *gorm.DB) NotifyLogRepository { return &notifyLogRepo{db: db} }

func (r *notifyLogRepo) Create(ctx context.Context, log *model.PaymentNotifyLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}
