package service

import (
	"context"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/wxpay"
)

// fakeRefundQuerier 是退款单查询的测试替身。
type fakeRefundQuerier struct {
	result *wxpay.RefundNotifyResult
	err    error
	calls  int
}

func (f *fakeRefundQuerier) QueryRefund(_ context.Context, _ string) (*wxpay.RefundNotifyResult, error) {
	f.calls++
	return f.result, f.err
}

func TestRefundWatchdogCompletesStuckRefund(t *testing.T) {
	f := newRefundFixture(t)
	ctx := context.Background()
	_, groupID, orderNo := f.paidOrder(t, "watchdog-ok")
	f.takeSlot(t, groupID)

	created, err := f.svc.CreateTicket(ctx, f.adminID, model.CreateRefundTicketRequest{
		OrderNo: orderNo, Reason: "用户申请退款",
	})
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	// 构造「已受理、回调丢失」：工单 approved、订单 refunding、更新时间早于看门狗窗口。
	f.setOrderStatus(t, orderNo, model.OrderStatusRefunding)
	mustExecSQL(t, f.db, `UPDATE refund_tickets SET status = 'approved', handled_at = '2020-01-01T00:00:00+08:00',
		updated_at = '2020-01-01T00:00:00+08:00' WHERE id = ?`, created.TicketID)

	querier := &fakeRefundQuerier{result: &wxpay.RefundNotifyResult{
		OutRefundNo:  created.TicketNo,
		RefundID:     "wx-refund-watchdog-1",
		RefundStatus: wxpay.RefundStatusSuccess,
		RefundAmount: 98000,
		TotalAmount:  98000,
	}}
	watchdog := NewRefundWatchdog(f.stores, f.svc, querier, 10, false)
	if err := watchdog.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	ticket, err := f.stores.RefundTickets.Get(ctx, created.TicketID)
	if err != nil {
		t.Fatalf("get ticket: %v", err)
	}
	if ticket.Status != model.RefundTicketStatusRefunded {
		t.Fatalf("ticket status = %q, want refunded", ticket.Status)
	}
	order, err := f.stores.Orders.GetByOrderNo(ctx, orderNo)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if order.Status != model.OrderStatusRefunded {
		t.Fatalf("order status = %q, want refunded", order.Status)
	}
	if taken := groupTaken(t, f.db, groupID); taken != 0 {
		t.Fatalf("group taken = %d, want 0", taken)
	}
}

func TestRefundWatchdogMarksFailedOnAbnormal(t *testing.T) {
	f := newRefundFixture(t)
	ctx := context.Background()
	_, _, orderNo := f.paidOrder(t, "watchdog-fail")

	created, err := f.svc.CreateTicket(ctx, f.adminID, model.CreateRefundTicketRequest{
		OrderNo: orderNo, Reason: "用户申请退款",
	})
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	f.setOrderStatus(t, orderNo, model.OrderStatusRefunding)
	mustExecSQL(t, f.db, `UPDATE refund_tickets SET status = 'approved', handled_at = '2020-01-01T00:00:00+08:00',
		updated_at = '2020-01-01T00:00:00+08:00' WHERE id = ?`, created.TicketID)

	querier := &fakeRefundQuerier{result: &wxpay.RefundNotifyResult{
		OutRefundNo:  created.TicketNo,
		RefundID:     "wx-refund-watchdog-2",
		RefundStatus: wxpay.RefundStatusAbnormal,
	}}
	watchdog := NewRefundWatchdog(f.stores, f.svc, querier, 10, false)
	if err := watchdog.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	ticket, err := f.stores.RefundTickets.Get(ctx, created.TicketID)
	if err != nil {
		t.Fatalf("get ticket: %v", err)
	}
	if ticket.Status != model.RefundTicketStatusRefundFailed {
		t.Fatalf("ticket status = %q, want refund_failed", ticket.Status)
	}
	order, err := f.stores.Orders.GetByOrderNo(ctx, orderNo)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if order.Status != model.OrderStatusPaid {
		t.Fatalf("order status = %q, want paid（回退可重试）", order.Status)
	}
}

func TestRefundWatchdogSkippedForMockGateway(t *testing.T) {
	f := newRefundFixture(t)
	ctx := context.Background()
	_, _, orderNo := f.paidOrder(t, "watchdog-mock")

	created, err := f.svc.CreateTicket(ctx, f.adminID, model.CreateRefundTicketRequest{
		OrderNo: orderNo, Reason: "用户申请退款",
	})
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	mustExecSQL(t, f.db, `UPDATE refund_tickets SET status = 'approved', updated_at = '2020-01-01T00:00:00+08:00'
		WHERE id = ?`, created.TicketID)

	querier := &fakeRefundQuerier{}
	watchdog := NewRefundWatchdog(f.stores, f.svc, querier, 10, true)
	if err := watchdog.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if querier.calls != 0 {
		t.Fatalf("query calls = %d, want 0（mock 网关下跳过看门狗）", querier.calls)
	}
}
