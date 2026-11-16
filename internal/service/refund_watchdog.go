package service

import (
	"context"
	"time"

	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/pkg/wxpay"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// RefundWatchdog 兜底卡死的退款工单（T34）：processing/approved 超过窗口仍无回调时
// 主动查询微信退款单——SUCCESS 完成退款；CLOSED/ABNORMAL 置失败并回退订单；
// PROCESSING 或查询失败留待下轮。PAY_MOCK=true 时整体跳过（mock 无真实退款单）。
type RefundWatchdog struct {
	stores  *store.Store
	refunds *RefundService
	query   wxpay.RefundQuerier
	window  time.Duration
	batch   int
	now     func() time.Time
	enabled bool
}

// NewRefundWatchdog 构造退款看门狗任务。
func NewRefundWatchdog(stores *store.Store, refunds *RefundService, query wxpay.RefundQuerier,
	windowMinutes int, payMock bool) *RefundWatchdog {
	if windowMinutes <= 0 {
		windowMinutes = 10
	}
	return &RefundWatchdog{
		stores:  stores,
		refunds: refunds,
		query:   query,
		window:  time.Duration(windowMinutes) * time.Minute,
		batch:   50,
		now:     time.Now,
		enabled: !payMock,
	}
}

// Name 返回任务名（调度器日志用）。
func (j *RefundWatchdog) Name() string { return "refund_watchdog" }

// RunOnce 执行一轮退款单巡检。
func (j *RefundWatchdog) RunOnce(ctx context.Context) error {
	if !j.enabled {
		return nil
	}
	cutoff := utils.FormatISO(j.now().Add(-j.window))
	tickets, err := j.stores.RefundTickets.ListStale(ctx, cutoff, j.batch)
	if err != nil {
		return err
	}
	log := logger.FromCtx(ctx)
	settled, failed := 0, 0
	for i := range tickets {
		ticket := tickets[i]
		result, err := j.query.QueryRefund(ctx, ticket.TicketNo)
		if err != nil {
			log.Warnw("退款单查询失败，本轮跳过", "ticket_no", ticket.TicketNo, "error", err)
			continue
		}
		switch result.RefundStatus {
		case wxpay.RefundStatusSuccess:
			if err := j.refunds.settleRefund(ctx, result, ""); err != nil {
				log.Warnw("退款查单补结账失败", "ticket_no", ticket.TicketNo, "error", err)
				continue
			}
			settled++
		case wxpay.RefundStatusClosed, wxpay.RefundStatusAbnormal:
			if err := j.refunds.settleRefundFailure(ctx, result); err != nil {
				log.Warnw("退款查单失败态落库失败", "ticket_no", ticket.TicketNo, "error", err)
				continue
			}
			failed++
		}
	}
	if len(tickets) > 0 {
		log.Infow("退款看门狗巡检完成", "checked", len(tickets), "refunded", settled, "failed", failed)
	}
	return nil
}
