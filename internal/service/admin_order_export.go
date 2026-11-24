package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
)

// 订单导出审计动作（admin_logs.action）。
const adminActionOrderExport = "admin.order.export"

// Export 按当前筛选导出订单列表（S5-6，API 文档 11.1）：仅 admin / super_admin。
// 与列表同一筛选口径，不分页；导出记操作日志（导出人、行数、筛选条件）。
func (s *AdminOrderQueryService) Export(ctx context.Context, adminID int64, q model.AdminOrderListQuery) (*ExportFile, error) {
	if err := validateGroupLevelFilter(q.Level); err != nil {
		return nil, err
	}
	if err := validateDanceTypeFilter(q.DanceType); err != nil {
		return nil, err
	}
	if err := validateAdminOrderStatusFilter(q.Status); err != nil {
		return nil, err
	}
	if err := validateAdminOrderSourceFilter(q.Source); err != nil {
		return nil, err
	}
	paidFrom, paidTo, err := normalizeAdminPaidRange(q.PaidFrom, q.PaidTo)
	if err != nil {
		return nil, err
	}

	rows, err := s.stores.Orders.ListForExport(ctx, store.AdminOrderFilter{
		EventID:         q.EventID,
		Level:           q.Level,
		DanceType:       q.DanceType,
		Status:          q.Status,
		IncludeCanceled: q.IncludeCanceled,
		Source:          q.Source,
		PaidFrom:        paidFrom,
		PaidTo:          paidTo,
		Keyword:         q.Keyword,
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载订单列表失败，请稍后重试")
	}
	content, err := buildOrderWorkbook(rows)
	if err != nil {
		return nil, httpx.ErrInternal(err, "生成导出文件失败，请稍后重试")
	}

	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		return adminLog(ctx, s.stores, tx, &adminID, adminActionOrderExport,
			strPtr("order"), nil, map[string]any{
				"rows":             len(rows),
				"event_id":         q.EventID,
				"status":           q.Status,
				"include_canceled": q.IncludeCanceled,
			})
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "记录操作日志失败，请稍后重试")
	}

	name := fmt.Sprintf("orders_%s.xlsx", s.now().Format("20060102"))
	return &ExportFile{Name: name, Content: content, Rows: len(rows)}, nil
}
