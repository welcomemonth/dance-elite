package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
)

const actionRegistrationWithdraw = "admin.registration.withdraw"

// AdminRegistrationActionService 承载后台对报名单的非资金操作（S2 人工退赛）。
//
// 退赛 ≠ 退款：只把有效报名置为 withdrawn 并释放名额，订单与支付流水保持原状；
// 如需退款仍走客服登记 + 退款工单（T15）。重复退赛幂等返回。
type AdminRegistrationActionService struct {
	stores *store.Store
}

// NewAdminRegistrationActionService 构造报名单操作服务。
func NewAdminRegistrationActionService(stores *store.Store) *AdminRegistrationActionService {
	return &AdminRegistrationActionService{stores: stores}
}

// Withdraw 人工退赛（仅 admin / super_admin，路由层保证角色）。
func (s *AdminRegistrationActionService) Withdraw(ctx context.Context, adminID, registrationID int64, reason string) (*model.AdminWithdrawRegistrationResponse, error) {
	if registrationID <= 0 {
		return nil, httpx.ErrBadRequest("报名 ID 非法")
	}
	released := false
	err := s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		registration, err := s.stores.Registrations.GetTx(ctx, tx, registrationID)
		if errors.Is(err, store.ErrNotFound) {
			return httpx.ErrNotFound("报名不存在")
		}
		if err != nil {
			return fmt.Errorf("加载报名单 %d: %w", registrationID, err)
		}
		// 幂等：已退赛直接返回，不重复释放名额。
		if registration.Status == model.RegistrationStatusWithdrawn {
			return nil
		}
		if registration.Status != model.RegistrationStatusActive {
			return httpx.ErrStateDenied("仅有效报名可退赛（待支付/已取消请走订单取消）")
		}
		order, err := s.stores.Orders.GetByRegistrationTx(ctx, tx, registrationID)
		if errors.Is(err, store.ErrNotFound) {
			return httpx.ErrNotFound("报名对应订单不存在")
		}
		if err != nil {
			return fmt.Errorf("加载报名单 %d 的订单: %w", registrationID, err)
		}
		if order.Status != model.OrderStatusPaid {
			return httpx.ErrStateDenied("订单非已支付状态，退款中/已退款请走退款工单")
		}

		if err := s.stores.Registrations.WithdrawTx(ctx, tx, registrationID); err != nil {
			if errors.Is(err, store.ErrConflict) {
				return httpx.ErrStateDenied("报名状态已变化，请刷新后重试")
			}
			return fmt.Errorf("退赛报名单 %d: %w", registrationID, err)
		}
		if err := s.stores.Groups.ReleaseSlotTx(ctx, tx, registration.GroupID); err != nil {
			return fmt.Errorf("释放组别 %d 名额: %w", registration.GroupID, err)
		}
		released = true

		detail := map[string]any{
			"order_no": order.OrderNo,
			"group_id": registration.GroupID,
			"reason":   reason,
		}
		targetType := "registration"
		return adminLog(ctx, s.stores, tx, &adminID, actionRegistrationWithdraw,
			&targetType, &registrationID, detail)
	})
	if err != nil {
		return nil, err
	}
	return &model.AdminWithdrawRegistrationResponse{
		RegistrationID: registrationID,
		Status:         model.RegistrationStatusWithdrawn,
		Released:       released,
	}, nil
}
