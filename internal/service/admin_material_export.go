package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// 资料催收导出审计动作（admin_logs.action）。
const adminActionMaterialExport = "admin.material.export"

// Export 按当前筛选导出资料催收名单（S5-7，API 文档 12.3）：
// 仅 admin / super_admin，与列表同一筛选口径，不分页。
func (s *MaterialCollectionService) Export(ctx context.Context, adminID int64, q model.AdminMissingMaterialQuery) (*ExportFile, error) {
	if err := validateGroupLevelFilter(q.Level); err != nil {
		return nil, err
	}
	if err := validateDanceTypeFilter(q.DanceType); err != nil {
		return nil, err
	}
	if err := validateMissingItemFilter(q.MissingItem); err != nil {
		return nil, err
	}
	if err := validateDeadlineStateFilter(q.DeadlineState); err != nil {
		return nil, err
	}

	rows, err := s.stores.Registrations.ListMissingForExport(ctx, store.AdminMissingMaterialFilter{
		EventID:       q.EventID,
		Level:         q.Level,
		DanceType:     q.DanceType,
		MissingItem:   q.MissingItem,
		DeadlineState: q.DeadlineState,
		Keyword:       q.Keyword,
		Now:           utils.FormatISO(s.now()),
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载资料催收名单失败，请稍后重试")
	}
	content, err := buildMissingMaterialWorkbook(rows)
	if err != nil {
		return nil, httpx.ErrInternal(err, "生成导出文件失败，请稍后重试")
	}

	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		return adminLog(ctx, s.stores, tx, &adminID, adminActionMaterialExport,
			strPtr("registration"), nil, map[string]any{
				"rows":           len(rows),
				"event_id":       q.EventID,
				"missing_item":   q.MissingItem,
				"deadline_state": q.DeadlineState,
			})
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "记录操作日志失败，请稍后重试")
	}

	name := fmt.Sprintf("missing_materials_%s.xlsx", s.now().Format("20060102"))
	return &ExportFile{Name: name, Content: content, Rows: len(rows)}, nil
}
