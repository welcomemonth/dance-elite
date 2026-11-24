package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
)

// 候补队列导出审计动作（admin_logs.action）。
const adminActionWaitlistExport = "admin.waitlist.export"

// Export 按当前筛选导出候补队列（S5-7，API 文档 12.1）：
// 仅 admin / super_admin，与列表同一筛选口径，不分页。
func (s *WaitlistService) Export(ctx context.Context, adminID int64, q model.AdminWaitlistListQuery) (*ExportFile, error) {
	if err := validateWaitlistStatusFilter(q.Status); err != nil {
		return nil, err
	}
	if q.Level != "" && q.Level != model.GroupLevelU11 && q.Level != model.GroupLevelU13 && q.Level != model.GroupLevelU15 {
		return nil, httpx.ErrBadRequest("level 取值不合法")
	}
	if q.DanceType != "" && q.DanceType != model.DanceTypeClassical && q.DanceType != model.DanceTypeFolk {
		return nil, httpx.ErrBadRequest("dance_type 取值不合法")
	}

	rows, err := s.stores.Waitlists.ListForExport(ctx, store.AdminWaitlistFilter{
		EventID:   q.EventID,
		Level:     q.Level,
		DanceType: q.DanceType,
		Status:    q.Status,
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载候补队列失败，请稍后重试")
	}
	content, err := buildWaitlistWorkbook(rows)
	if err != nil {
		return nil, httpx.ErrInternal(err, "生成导出文件失败，请稍后重试")
	}

	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		return adminLog(ctx, s.stores, tx, &adminID, adminActionWaitlistExport,
			strPtr("waitlist"), nil, map[string]any{
				"rows":       len(rows),
				"event_id":   q.EventID,
				"status":     q.Status,
				"level":      q.Level,
				"dance_type": q.DanceType,
			})
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "记录操作日志失败，请稍后重试")
	}

	name := fmt.Sprintf("waitlists_%s.xlsx", s.now().Format("20060102"))
	return &ExportFile{Name: name, Content: content, Rows: len(rows)}, nil
}
