package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/xuri/excelize/v2"
)

// adminActionPrizeExport 是奖金发放表导出的审计动作。
const adminActionPrizeExport = "admin.prize.export"

const prizeExportSheetName = "奖金发放表"

// Export 导出赛季奖金发放表（财务对账）：可按赛事/状态筛选，仅 admin / super_admin。
func (s *PrizeService) Export(ctx context.Context, adminID, seasonID, eventID int64, status string) (*ExportFile, error) {
	if seasonID <= 0 {
		return nil, httpx.ErrBadRequest("season_id 必填")
	}
	if err := validatePrizeStatusFilter(status); err != nil {
		return nil, err
	}
	rows, err := s.stores.Prizes.ListBySeason(ctx, seasonID, status)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载奖金明细失败，请稍后重试")
	}
	if eventID > 0 {
		filtered := rows[:0]
		for _, r := range rows {
			if r.Prize.EventID != nil && *r.Prize.EventID == eventID {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}
	content, err := buildPrizeWorkbook(rows)
	if err != nil {
		return nil, httpx.ErrInternal(err, "生成导出文件失败，请稍后重试")
	}

	targetType := "season"
	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		return adminLog(ctx, s.stores, tx, &adminID, adminActionPrizeExport,
			&targetType, &seasonID, map[string]any{
				"rows": len(rows), "season_id": seasonID, "event_id": eventID, "status": status,
			})
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "记录操作日志失败，请稍后重试")
	}

	name := fmt.Sprintf("prizes_%s.xlsx", time.Now().Format("20060102"))
	return &ExportFile{Name: name, Content: content, Rows: len(rows)}, nil
}

// buildPrizeWorkbook 生成奖金发放表 Excel（列固定，用于财务对账/发放留档）。
func buildPrizeWorkbook(rows []store.PrizeRow) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetSheetName("Sheet1", prizeExportSheetName); err != nil {
		return nil, fmt.Errorf("rename prize export sheet: %w", err)
	}
	headers := []any{"赛事", "组别", "舞种", "选手编号", "选手姓名", "机构", "奖项",
		"金额(元)", "发放状态", "发放时间", "领取人/备注"}
	if err := f.SetSheetRow(prizeExportSheetName, "A1", &headers); err != nil {
		return nil, fmt.Errorf("write prize export header: %w", err)
	}
	for i, row := range rows {
		values := []any{
			ptrValue(row.EventName, ""),
			ptrValue(row.BoardLevel, ""),
			danceTypeLabel(ptrValue(row.DanceType, "")),
			row.PlayerNo,
			row.PlayerName,
			ptrValue(row.Prize.InstitutionName, ""),
			prizeAwardLabel(row.Prize.Award),
			float64(row.Prize.Amount) / 100,
			prizeStatusLabel(row.Prize.Status),
			ptrValue(row.Prize.GrantedAt, ""),
			ptrValue(row.Prize.GranteeNote, ""),
		}
		cell := fmt.Sprintf("A%d", i+2)
		if err := f.SetSheetRow(prizeExportSheetName, cell, &values); err != nil {
			return nil, fmt.Errorf("write prize export row %d: %w", i+1, err)
		}
	}
	widths := map[string]float64{
		"A": 28, "B": 8, "C": 14, "D": 12, "E": 12, "F": 20,
		"G": 12, "H": 12, "I": 10, "J": 22, "K": 28,
	}
	if err := styleExportSheetRange(f, prizeExportSheetName, "K", widths); err != nil {
		return nil, err
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write prize export workbook: %w", err)
	}
	return buf.Bytes(), nil
}

func prizeAwardLabel(award string) string {
	switch award {
	case model.AwardChampion:
		return "冠军"
	case model.AwardRunnerUp:
		return "亚军"
	case model.AwardThird:
		return "季军"
	case model.AwardEliteDancer:
		return "菁英舞者"
	case model.AwardSpecialGold:
		return "特金奖"
	case model.AwardGold:
		return "金奖"
	case model.AwardSilver:
		return "银奖"
	default:
		return award
	}
}

func prizeStatusLabel(status string) string {
	switch status {
	case "pending":
		return "待发放"
	case "granted":
		return "已发放"
	case "skipped":
		return "已跳过"
	default:
		return status
	}
}

// validatePrizeStatusFilter 校验奖金明细状态筛选（空串表示全部）。
func validatePrizeStatusFilter(status string) error {
	if status == "" || status == "pending" || status == "granted" || status == "skipped" {
		return nil
	}
	return httpx.ErrBadRequest("status 取值不合法（pending / granted / skipped）")
}
