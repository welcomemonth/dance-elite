package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
)

// RegistrationQueryService 是小程序「我的报名」只读查询服务（开发计划 T35，
// API 文档 3.3/3.4）。仅返回本人报名；待支付与已取消订单不可见。
type RegistrationQueryService struct {
	stores *store.Store
	now    func() time.Time
}

// NewRegistrationQueryService 构造小程序报名查询服务。
func NewRegistrationQueryService(stores *store.Store) *RegistrationQueryService {
	return &RegistrationQueryService{stores: stores, now: time.Now}
}

// List 返回当前用户的报名列表（GET /registrations）。
func (s *RegistrationQueryService) List(ctx context.Context, userID int64, page, pageSize int) (*model.MiniappRegistrationListResponse, error) {
	page, pageSize = normalizePage(page, pageSize)

	rows, total, err := s.stores.Registrations.ListMine(ctx, userID, page, pageSize)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载报名列表失败，请稍后重试")
	}

	list := make([]model.MiniappRegistrationItem, 0, len(rows))
	for i := range rows {
		item, err := miniappRegistrationItemOf(&rows[i])
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return &model.MiniappRegistrationListResponse{
		List: list, Total: total, Page: page, PageSize: pageSize,
	}, nil
}

// Detail 返回当前用户的一条报名详情（GET /registrations/{registration_id}）。
// 不存在、非本人、待支付或已取消统一返回 40401，避免枚举。
func (s *RegistrationQueryService) Detail(ctx context.Context, userID, registrationID int64) (*model.MiniappRegistrationDetail, error) {
	row, err := s.stores.Registrations.GetMine(ctx, userID, registrationID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("报名不存在或非本人")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载报名详情失败，请稍后重试")
	}

	status, ok := miniappRegistrationStatusOf(row.Order.Status)
	if !ok {
		return nil, httpx.ErrNotFound("报名不存在或非本人")
	}

	now := s.now()
	materialStatus := MaterialStatusOf(&row.Registration)
	resp := &model.MiniappRegistrationDetail{
		Status:           status,
		StatusDesc:       miniappRegistrationStatusDesc(status, materialStatus, row.MaterialCheckAt, now),
		MaterialReadonly: materialReadonly(row.MaterialCheckAt, now),
		MaterialStatus:   materialStatus,
		Event: model.MiniappRegistrationEvent{
			ID:        row.Registration.EventID,
			Name:      row.EventName,
			Level:     row.EventLevel,
			EventDate: row.EventDate,
		},
		LevelGroup:  row.Level,
		DanceType:   row.Registration.DanceType,
		PlayerName:  row.PlayerName,
		PlayerNo:    row.PlayerNo,
		Institution: row.Registration.Institution,
		OrderNo:     row.Order.OrderNo,
		Amount:      row.Order.Amount,
		PaidAt:      row.Order.PaidAt,
		Materials:   MaterialsSnapshotOf(&row.Registration),
	}
	if status != model.MiniappRegistrationStatusPaid {
		refund, err := s.latestRefund(ctx, row.Order.ID)
		if err != nil {
			return nil, err
		}
		resp.Refund = refund
	}
	return resp, nil
}

// latestRefund 读取订单最近一张退款工单并映射为小程序展示态；无工单返回 nil。
func (s *RegistrationQueryService) latestRefund(ctx context.Context, orderID int64) (*model.MiniappRegistrationRefund, error) {
	ticket, err := s.stores.RefundTickets.LatestByOrder(ctx, orderID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载退款信息失败，请稍后重试")
	}
	return miniappRefundOf(ticket), nil
}

// miniappRegistrationStatusOf 把订单状态映射为小程序报名展示状态（API 文档 3.3）。
func miniappRegistrationStatusOf(orderStatus string) (string, bool) {
	switch orderStatus {
	case model.OrderStatusPaid:
		return model.MiniappRegistrationStatusPaid, true
	case model.OrderStatusRefunding:
		return model.MiniappRegistrationStatusRefunding, true
	case model.OrderStatusRefunded:
		return model.MiniappRegistrationStatusRefunded, true
	default:
		return "", false
	}
}

// unexpectedStatusError 构造订单状态异常的内部错误（正常不应出现）。
func unexpectedStatusError(status string) error {
	return fmt.Errorf("unexpected order status %q for miniapp registration", status)
}
