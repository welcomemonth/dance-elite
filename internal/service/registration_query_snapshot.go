package service

import (
	"fmt"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// miniappRegistrationItemOf 把查询行映射为「我的报名」列表项（API 文档 3.3）。
func miniappRegistrationItemOf(row *store.MiniappRegistrationRow) (model.MiniappRegistrationItem, error) {
	status, ok := miniappRegistrationStatusOf(row.Order.Status, row.Registration.Status)
	if !ok {
		return model.MiniappRegistrationItem{}, httpx.ErrInternal(
			unexpectedStatusError(row.Order.Status), "加载报名列表失败，请稍后重试")
	}
	return model.MiniappRegistrationItem{
		RegistrationID:   row.Registration.ID,
		EventID:          row.Registration.EventID,
		EventName:        row.EventName,
		Level:            row.EventLevel,
		EventDate:        row.EventDate,
		LevelGroup:       row.Level,
		DanceType:        row.Registration.DanceType,
		PlayerName:       row.PlayerName,
		PlayerNo:         row.PlayerNo,
		OrderNo:          row.Order.OrderNo,
		Amount:           row.Order.Amount,
		Status:           status,
		MaterialStatus:   MaterialStatusOf(&row.Registration),
		MaterialDeadline: row.MaterialCheckAt,
	}, nil
}

// miniappRefundOf 把退款工单映射为小程序展示态（API 文档 3.4）。
// 小程序只区分处理中/已退款，不暴露内部工单状态机（approved/refund_failed 等）。
func miniappRefundOf(ticket *model.RefundTicket) *model.MiniappRegistrationRefund {
	status := "processing"
	if ticket.Status == model.RefundTicketStatusRefunded {
		status = model.MiniappRegistrationStatusRefunded
	}
	return &model.MiniappRegistrationRefund{
		TicketNo: ticket.TicketNo,
		Status:   status,
		Reason:   ticket.Reason,
	}
}

// miniappRegistrationStatusDesc 生成报名详情状态说明文案（API 文档 3.4）。
func miniappRegistrationStatusDesc(status string, material model.MaterialStatus,
	deadline *string, now time.Time) string {
	switch status {
	case model.MiniappRegistrationStatusRefunding:
		return "退款处理中，请等待工作人员处理，结果将通过客服会话告知"
	case model.MiniappRegistrationStatusRefunded:
		return "退款已完成，报名已退赛"
	case model.MiniappRegistrationStatusWithdrawn:
		return "该报名已退赛，名额已释放；如需退款请联系客服登记"
	}

	if material.Completed == material.Total {
		return "报名已支付，资料已提交；请按时参赛"
	}
	if deadline != nil && *deadline != "" {
		if utils.FormatISO(now) < *deadline {
			return fmt.Sprintf("请在 %s 前上传资料；不上传不影响参赛", formatCNDate(*deadline))
		}
		return "报名已支付；资料上传已截止"
	}
	return "报名已支付；请按时参赛"
}

// formatCNDate 把统一 ISO 时间格式化为中文日期（如 2027 年 3 月 1 日）；
// 解析失败时回退原始字符串，避免状态文案导致整个详情接口失败。
func formatCNDate(iso string) string {
	t, err := utils.ParseISO(iso)
	if err != nil {
		return iso
	}
	return t.In(utils.ChinaLoc).Format("2006 年 1 月 2 日")
}
