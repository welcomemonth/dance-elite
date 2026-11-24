package service

import (
	"fmt"

	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/xuri/excelize/v2"
)

// orderExportSheetName 是订单导出文件的工作表名。
const orderExportSheetName = "订单列表"

// buildOrderWorkbook 生成订单列表 Excel（与列表筛选一致，对账/存档用）。
// 列固定，不含证件号任何形态（系统只存 HMAC 哈希）。
func buildOrderWorkbook(rows []store.AdminOrderRow) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetSheetName("Sheet1", orderExportSheetName); err != nil {
		return nil, fmt.Errorf("rename order export sheet: %w", err)
	}

	headers := []any{"订单号", "微信支付单号", "赛事", "组别", "舞种", "选手编号",
		"选手姓名", "证件类型", "金额(元)", "状态", "来源", "支付时间", "创建时间"}
	if err := f.SetSheetRow(orderExportSheetName, "A1", &headers); err != nil {
		return nil, fmt.Errorf("write order export header: %w", err)
	}
	for i, row := range rows {
		values := []any{
			row.OrderNo,
			ptrValue(row.TransactionID, ""),
			row.EventName,
			row.Level,
			danceTypeLabel(row.DanceType),
			row.PlayerNo,
			row.PlayerName,
			idTypeLabel(row.IDType),
			float64(row.Amount) / 100,
			orderStatusLabel(row.Status),
			orderSourceLabel(row.Source),
			ptrValue(row.PaidAt, ""),
			row.CreatedAt,
		}
		cell := fmt.Sprintf("A%d", i+2)
		if err := f.SetSheetRow(orderExportSheetName, cell, &values); err != nil {
			return nil, fmt.Errorf("write order export row %d: %w", i+1, err)
		}
	}

	widths := map[string]float64{
		"A": 24, "B": 40, "C": 24, "D": 8, "E": 14, "F": 12, "G": 12,
		"H": 12, "I": 12, "J": 10, "K": 10, "L": 22, "M": 22,
	}
	if err := styleExportSheetRange(f, orderExportSheetName, "M", widths); err != nil {
		return nil, err
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write order export workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// orderStatusLabel 把订单状态枚举转换为中文展示。
func orderStatusLabel(status string) string {
	switch status {
	case "pending_payment":
		return "待支付"
	case "paid":
		return "已支付"
	case "canceled":
		return "已取消"
	case "refunding":
		return "退款中"
	case "refunded":
		return "已退款"
	case "pay_failed":
		return "支付失败"
	default:
		return status
	}
}

// orderSourceLabel 把订单来源枚举转换为中文展示。
func orderSourceLabel(source string) string {
	switch source {
	case "miniapp":
		return "小程序"
	case "manual":
		return "后台人工"
	default:
		return source
	}
}

// idTypeLabel 把证件类型枚举转换为中文展示。
func idTypeLabel(idType string) string {
	switch idType {
	case "id_card":
		return "身份证"
	case "passport":
		return "护照"
	case "hk_macau":
		return "港澳通行证"
	case "other":
		return "其他"
	default:
		return idType
	}
}
