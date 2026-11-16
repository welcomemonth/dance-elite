package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/wxpay"
)

// fakeOrderQuerier 是主动查单的测试替身。
type fakeOrderQuerier struct {
	tx    *wxpay.Transaction
	err   error
	calls int
}

func (f *fakeOrderQuerier) QueryOrder(_ context.Context, _ string) (*wxpay.Transaction, error) {
	f.calls++
	return f.tx, f.err
}

// expiredPendingOrder 构造一个已过期的待支付订单，返回用户/赛事/订单与报名单元。
func expiredPendingOrder(t *testing.T, f *regFixture, key string) (int64, int64, int64, *model.Order) {
	t.Helper()
	ctx := context.Background()
	eventID := mustCreateRegisteringEvent(t, f, "超时赛事-"+key, defaultRegGroups)
	userID := mustInsertRegUser(t, f.db, "o-expiry-"+key)
	if _, err := f.regSvc.Submit(ctx, userID, eventID,
		submitReq(validIDCardMale2016, model.DanceTypeClassical, key)); err != nil {
		t.Fatalf("submit: %v", err)
	}
	order, err := f.stores.Orders.GetByRequest(ctx, userID, key)
	if err != nil || order == nil {
		t.Fatalf("get order: %v", err)
	}
	mustExecSQL(t, f.db, `UPDATE orders SET expire_at = '2020-01-01T00:00:00+08:00' WHERE id = ?`, order.ID)
	order, err = f.stores.Orders.Get(ctx, order.ID)
	if err != nil {
		t.Fatalf("reload order: %v", err)
	}
	groupID := mustGroupID(t, f.db, eventID, model.GroupLevelU11, model.DanceTypeClassical)
	return userID, eventID, groupID, order
}

func TestOrderExpirySweeperCancelsNotPay(t *testing.T) {
	f := newRegFixture(t)
	ctx := context.Background()
	_, eventID, groupID, order := expiredPendingOrder(t, f, "key-expiry-notpay")

	mock := wxpay.NewMockGateway() // QueryOrder 恒返回 NOTPAY
	payments := NewPaymentService(f.stores, mock, mock)
	sweeper := NewOrderExpirySweeper(f.stores, payments, 15)
	if err := sweeper.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	latest, err := f.stores.Orders.Get(ctx, order.ID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if latest.Status != model.OrderStatusCanceled || latest.CancelReason == nil ||
		*latest.CancelReason != model.OrderCancelReasonSystem {
		t.Fatalf("order = %+v, want canceled/system", latest)
	}
	if taken := groupTaken(t, f.db, groupID); taken != 0 {
		t.Fatalf("group taken = %d, want 0", taken)
	}
	registration, err := f.stores.Registrations.Get(ctx, order.RegistrationID)
	if err != nil {
		t.Fatalf("get registration: %v", err)
	}
	if registration.Status != model.RegistrationStatusCanceled {
		t.Fatalf("registration status = %q, want canceled", registration.Status)
	}
	_ = eventID
}

func TestOrderExpirySweeperSettlesPaidOrder(t *testing.T) {
	f := newRegFixture(t)
	ctx := context.Background()
	_, _, groupID, order := expiredPendingOrder(t, f, "key-expiry-paid")

	querier := &fakeOrderQuerier{tx: &wxpay.Transaction{
		OutTradeNo:    order.OrderNo,
		TransactionID: "wx-tx-expiry-1",
		TradeState:    wxpay.TradeStateSuccess,
		AmountTotal:   order.Amount,
		SuccessTime:   time.Now().In(time.Local).Format(time.RFC3339),
	}}
	payments := NewPaymentService(f.stores, wxpay.NewMockGateway(), querier)
	sweeper := NewOrderExpirySweeper(f.stores, payments, 15)
	if err := sweeper.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	latest, err := f.stores.Orders.Get(ctx, order.ID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if latest.Status != model.OrderStatusPaid {
		t.Fatalf("order status = %q, want paid", latest.Status)
	}
	if taken := groupTaken(t, f.db, groupID); taken != 1 {
		t.Fatalf("group taken = %d, want 1", taken)
	}
}

func TestOrderExpirySweeperSkipsOnQueryError(t *testing.T) {
	f := newRegFixture(t)
	ctx := context.Background()
	_, _, _, order := expiredPendingOrder(t, f, "key-expiry-err")

	querier := &fakeOrderQuerier{err: errors.New("network down")}
	payments := NewPaymentService(f.stores, wxpay.NewMockGateway(), querier)
	sweeper := NewOrderExpirySweeper(f.stores, payments, 15)
	if err := sweeper.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	latest, err := f.stores.Orders.Get(ctx, order.ID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if latest.Status != model.OrderStatusPendingPayment {
		t.Fatalf("order status = %q, want pending_payment（查单失败不取消）", latest.Status)
	}
}

func TestPrepayExpiredOrderCancelsWithoutPrepay(t *testing.T) {
	f := newRegFixture(t)
	ctx := context.Background()
	userID, _, _, order := expiredPendingOrder(t, f, "key-prepay-expired")

	gateway := &fakeGateway{}
	querier := &fakeOrderQuerier{} // 未支付（nil 交易）
	payments := NewPaymentService(f.stores, wxpay.NewMockGateway(), querier)
	svc := NewOrderService(f.stores, gateway, payments, 15)

	_, err := svc.Prepay(ctx, userID, "openid", order.ID)
	if err == nil {
		t.Fatal("过期订单预下单应被拒绝")
	}
	var biz *httpx.BizError
	if !errors.As(err, &biz) || biz.Code != httpx.CodeConflict {
		t.Fatalf("err = %v, want 40901", err)
	}
	if gateway.calls != 0 {
		t.Fatalf("prepay calls = %d, want 0（过期订单不得向微信下单）", gateway.calls)
	}
	latest, gerr := f.stores.Orders.Get(ctx, order.ID)
	if gerr != nil {
		t.Fatalf("get order: %v", gerr)
	}
	if latest.Status != model.OrderStatusCanceled {
		t.Fatalf("order status = %q, want canceled", latest.Status)
	}
}
