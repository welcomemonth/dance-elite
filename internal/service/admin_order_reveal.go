package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
)

// actionRevealPhone 是明文查看手机号写入 admin_logs 的动作名（API 文档 11.3）。
const actionRevealPhone = "reveal_phone"

// RevealPhone 返回订单联系手机号明文，并在同一事务内落 reveal_phone 日志
// （谁、何时、看了哪个订单）。权限位由路由层 RequireRole 保证。
func (s *AdminOrderQueryService) RevealPhone(ctx context.Context, actorID int64, orderNo string) (*model.AdminRevealPhoneResponse, error) {
	row, err := s.stores.Orders.GetAdminDetail(ctx, orderNo)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("订单不存在")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载订单失败，请稍后重试")
	}
	phone := ""
	if row.ContactPhone != nil {
		phone = *row.ContactPhone
	}

	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		targetType := "order"
		orderID := row.Order.ID
		return adminLog(ctx, s.stores, tx, &actorID, actionRevealPhone, &targetType, &orderID,
			map[string]any{"order_no": orderNo})
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "记录查看日志失败，请稍后重试")
	}
	return &model.AdminRevealPhoneResponse{ContactPhone: phone}, nil
}
