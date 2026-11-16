package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// orderExpired 判断待支付订单是否已超过支付窗口：
// 优先 orders.expire_at；历史订单为空时按 created_at + minutes 兜底。
func orderExpired(order *model.Order, now time.Time, minutes int) bool {
	if order.Status != model.OrderStatusPendingPayment {
		return false
	}
	if order.ExpireAt != nil && *order.ExpireAt != "" {
		if t, err := utils.ParseISO(*order.ExpireAt); err == nil {
			return !now.Before(t)
		}
	}
	if t, err := utils.ParseISO(order.CreatedAt); err == nil {
		return !now.Before(t.Add(time.Duration(minutes) * time.Minute))
	}
	return false
}

// orderExpireAt 返回订单的支付过期时刻（RFC3339）；缺失时按创建时间 + minutes 估算。
// 供微信统一下单的 time_expire 与本地超时判定保持一致。
func orderExpireAt(order *model.Order, minutes int) string {
	if order.ExpireAt != nil && *order.ExpireAt != "" {
		return *order.ExpireAt
	}
	if t, err := utils.ParseISO(order.CreatedAt); err == nil {
		return utils.FormatISO(t.Add(time.Duration(minutes) * time.Minute))
	}
	return ""
}

// OrderExpirySweeper 回收超时未支付订单（T34）：到期 pending 订单先向微信查单，
// SUCCESS 补结账、失败终态取消、确认未支付才系统取消并释放名额；
// 查询失败本轮跳过，不盲目取消。
type OrderExpirySweeper struct {
	stores        *store.Store
	payments      *PaymentService
	cancel        *orderCanceler
	expireMinutes int
	batch         int
	now           func() time.Time
}

// NewOrderExpirySweeper 构造待支付订单超时回收任务。
func NewOrderExpirySweeper(stores *store.Store, payments *PaymentService, expireMinutes int) *OrderExpirySweeper {
	if expireMinutes <= 0 {
		expireMinutes = 15
	}
	return &OrderExpirySweeper{
		stores:        stores,
		payments:      payments,
		cancel:        newOrderCanceler(stores),
		expireMinutes: expireMinutes,
		batch:         100,
		now:           time.Now,
	}
}

// Name 返回任务名（调度器日志用）。
func (j *OrderExpirySweeper) Name() string { return "order_expiry" }

// RunOnce 执行一轮超时回收。单个订单失败只记日志，不影响同轮其他订单。
func (j *OrderExpirySweeper) RunOnce(ctx context.Context) error {
	cutoff := utils.FormatISO(j.now().Add(-time.Duration(j.expireMinutes) * time.Minute))
	orders, err := j.stores.Orders.ListExpiredPending(ctx, cutoff, j.batch)
	if err != nil {
		return fmt.Errorf("查询超时待支付订单: %w", err)
	}
	canceled := 0
	for i := range orders {
		order := orders[i]
		applied, err := j.payments.QueryAndApply(ctx, &order)
		if err != nil {
			logger.WarnEvent(ctx, "order.expire_check_failed", "超时订单查单失败，本轮跳过",
				"order_no", order.OrderNo, "error", err)
			continue
		}
		if applied {
			continue
		}
		if _, err := j.cancel.cancelPending(ctx, order.ID, model.OrderCancelReasonSystem); err != nil {
			if !errors.Is(err, errOrderNotCancelable) {
				logger.WarnEvent(ctx, "order.cancel_failed", "超时订单取消失败",
					"order_no", order.OrderNo, "error", err)
			}
			continue
		}
		canceled++
	}
	if len(orders) > 0 {
		logger.Event(ctx, "order_expiry.swept", "待支付订单超时回收完成",
			"checked", len(orders), "canceled", canceled)
	}
	return nil
}
