package service

import (
	"fmt"
	"strings"

	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/xuri/excelize/v2"
)

// materialExportSheetName 是资料催收导出文件的工作表名。
const materialExportSheetName = "资料催收"

// buildMissingMaterialWorkbook 生成资料催收名单 Excel（缺失项/联系状态一目了然）。
func buildMissingMaterialWorkbook(rows []store.AdminMissingMaterialRow) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetSheetName("Sheet1", materialExportSheetName); err != nil {
		return nil, fmt.Errorf("rename material export sheet: %w", err)
	}

	headers := []any{"赛事", "组别", "舞种", "选手姓名", "选手编号", "联系电话",
		"资料完成度", "缺失项", "资料截止", "最后上传", "已联系", "联系时间", "订单号"}
	if err := f.SetSheetRow(materialExportSheetName, "A1", &headers); err != nil {
		return nil, fmt.Errorf("write material export header: %w", err)
	}
	for i, row := range rows {
		status := MaterialStatusOf(&row.Registration)
		values := []any{
			row.EventName,
			row.Level,
			danceTypeLabel(row.Registration.DanceType),
			row.PlayerName,
			row.PlayerNo,
			ptrValue(row.ContactPhone, ""),
			fmt.Sprintf("%d/%d", status.Completed, status.Total),
			strings.Join(missingItemLabels(status.Missing), "、"),
			ptrValue(row.MaterialCheckAt, ""),
			ptrValue(row.Registration.MaterialUpdatedAt, ""),
			contactedLabel(row.ContactedAt != nil),
			ptrValue(row.ContactedAt, ""),
			ptrValue(row.OrderNo, ""),
		}
		cell := fmt.Sprintf("A%d", i+2)
		if err := f.SetSheetRow(materialExportSheetName, cell, &values); err != nil {
			return nil, fmt.Errorf("write material export row %d: %w", i+1, err)
		}
	}

	widths := map[string]float64{
		"A": 24, "B": 8, "C": 14, "D": 12, "E": 12, "F": 14, "G": 12,
		"H": 24, "I": 22, "J": 22, "K": 10, "L": 22, "M": 24,
	}
	if err := styleExportSheetRange(f, materialExportSheetName, "M", widths); err != nil {
		return nil, err
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write material export workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// missingItemLabels 把缺失资料项字段名列表转换为中文列表。
func missingItemLabels(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, materialFieldLabel(item))
	}
	return out
}

// contactedLabel 返回是否已联系的中文展示。
func contactedLabel(contacted bool) string {
	if contacted {
		return "已联系"
	}
	return "未联系"
}
