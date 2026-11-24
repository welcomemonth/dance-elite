package service

import (
	"fmt"

	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/xuri/excelize/v2"
)

// waitlistExportSheetName 是候补队列导出文件的工作表名。
const waitlistExportSheetName = "候补队列"

// buildWaitlistWorkbook 生成候补队列 Excel（按队列顺序，对账/电话联系用）。
// 证件号只导出脱敏值（waitlists.id_card_masked），不含明文。
func buildWaitlistWorkbook(rows []store.AdminWaitlistRow) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetSheetName("Sheet1", waitlistExportSheetName); err != nil {
		return nil, fmt.Errorf("rename waitlist export sheet: %w", err)
	}

	headers := []any{"队列序号", "姓名", "证件号(脱敏)", "机构", "联系电话", "状态",
		"赛事", "组别", "舞种", "提交时间", "通知时间", "处理时间"}
	if err := f.SetSheetRow(waitlistExportSheetName, "A1", &headers); err != nil {
		return nil, fmt.Errorf("write waitlist export header: %w", err)
	}
	for i, row := range rows {
		values := []any{
			row.PositionNo,
			row.Name,
			ptrValue(row.IDCardMasked, ""),
			ptrValue(row.Institution, ""),
			row.Phone,
			waitlistStatusLabel(row.Status),
			row.EventName,
			row.Level,
			danceTypeLabel(row.DanceType),
			row.CreatedAt,
			ptrValue(row.NotifiedAt, ""),
			ptrValue(row.HandledAt, ""),
		}
		cell := fmt.Sprintf("A%d", i+2)
		if err := f.SetSheetRow(waitlistExportSheetName, cell, &values); err != nil {
			return nil, fmt.Errorf("write waitlist export row %d: %w", i+1, err)
		}
	}

	widths := map[string]float64{
		"A": 10, "B": 12, "C": 20, "D": 20, "E": 14, "F": 10,
		"G": 24, "H": 8, "I": 14, "J": 22, "K": 22, "L": 22,
	}
	if err := styleExportSheetRange(f, waitlistExportSheetName, "L", widths); err != nil {
		return nil, err
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write waitlist export workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// waitlistStatusLabel 把候补状态枚举转换为中文展示。
func waitlistStatusLabel(status string) string {
	switch status {
	case "waiting":
		return "排队中"
	case "notified":
		return "已通知"
	case "converted":
		return "已转报名"
	case "given_up":
		return "已放弃"
	case "removed":
		return "已移出"
	default:
		return status
	}
}
