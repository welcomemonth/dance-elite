package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/meetmonth/dance-elite/internal/model"
)

// AdminRefundTicketFilter 是退款工单列表的筛选与分页条件（API 文档 11.4.2）。
type AdminRefundTicketFilter struct {
	Status      string
	EventID     int64
	CreatedFrom string
	CreatedTo   string
	Keyword     string
	CreatedBy   int64
	Page        int
	PageSize    int
}

// ListAdmin 分页查询退款工单列表（联订单/赛事/选手/处理人）。
func (s *RefundTicketStore) ListAdmin(ctx context.Context, f AdminRefundTicketFilter) ([]model.AdminRefundTicketItem, int64, error) {
	where := make([]string, 0, 6)
	args := make([]any, 0, 6)
	if f.Status != "" {
		where = append(where, "t.status = ?")
		args = append(args, f.Status)
	}
	if f.EventID > 0 {
		where = append(where, "o.event_id = ?")
		args = append(args, f.EventID)
	}
	if f.CreatedBy > 0 {
		where = append(where, "t.created_by = ?")
		args = append(args, f.CreatedBy)
	}
	if f.CreatedFrom != "" {
		where = append(where, "t.created_at >= ?")
		args = append(args, f.CreatedFrom)
	}
	if f.CreatedTo != "" {
		where = append(where, "t.created_at <= ?")
		args = append(args, f.CreatedTo)
	}
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		where = append(where, "(t.ticket_no LIKE ? OR o.order_no LIKE ? OR pl.name LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like)
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}

	base := `
FROM refund_tickets t
JOIN orders o ON o.id = t.order_id
JOIN registrations r ON r.id = o.registration_id
JOIN events e ON e.id = r.event_id
JOIN players pl ON pl.id = r.player_id`

	var total int64
	if err := s.db.sql.QueryRowContext(ctx,
		`SELECT COUNT(*)`+base+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count refund tickets: %w", err)
	}

	rows, err := s.db.sql.QueryContext(ctx, `
SELECT t.id, t.ticket_no, o.order_no, e.id, e.name, pl.name, t.reason,
       COALESCE(t.applicant, '客服代录'), t.status, h.name, t.created_at, t.handled_at`+base+`
LEFT JOIN admins h ON h.id = t.handler_id`+whereSQL+`
ORDER BY t.id DESC LIMIT ? OFFSET ?`,
		append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list refund tickets: %w", err)
	}
	defer rows.Close()

	list := make([]model.AdminRefundTicketItem, 0, f.PageSize)
	for rows.Next() {
		var item model.AdminRefundTicketItem
		if err := rows.Scan(&item.TicketID, &item.TicketNo, &item.OrderNo, &item.EventID,
			&item.EventName, &item.PlayerName, &item.Reason, &item.Applicant, &item.Status,
			&item.HandlerName, &item.CreatedAt, &item.HandledAt); err != nil {
			return nil, 0, fmt.Errorf("scan refund ticket: %w", err)
		}
		list = append(list, item)
	}
	return list, total, rows.Err()
}

// GetAdminDetail 读取退款工单详情底座：工单 + 订单摘要 + 处理人姓名。
// 不存在返回 ErrNotFound。
func (s *RefundTicketStore) GetAdminDetail(ctx context.Context, ticketID int64) (*model.AdminRefundTicketDetail, error) {
	var d model.AdminRefundTicketDetail
	var applicant *string
	err := s.db.sql.QueryRowContext(ctx, `
SELECT t.id, t.ticket_no, o.order_no, e.id, e.name, pl.name, t.reason,
       t.applicant, t.status, h.name, t.created_at, t.handled_at,
       (SELECT p.transaction_id FROM payments p
         WHERE p.order_id = o.id AND p.type = 'pay' AND p.status = 'success'
         ORDER BY p.id DESC LIMIT 1),
       g.level, g.dance_type, o.amount, o.status, o.source, o.paid_at, o.created_at
FROM refund_tickets t
JOIN orders o ON o.id = t.order_id
JOIN registrations r ON r.id = o.registration_id
JOIN events e ON e.id = r.event_id
JOIN players pl ON pl.id = r.player_id
JOIN event_groups g ON g.id = r.group_id
LEFT JOIN admins h ON h.id = t.handler_id
WHERE t.id = ?`, ticketID).Scan(&d.TicketID, &d.TicketNo, &d.OrderNo, &d.EventID, &d.EventName,
		&d.PlayerName, &d.Reason, &applicant, &d.Status, &d.HandlerName, &d.CreatedAt, &d.HandledAt,
		&d.Order.TransactionID, &d.Order.Level, &d.Order.DanceType, &d.Order.Amount,
		&d.Order.Status, &d.Order.Source, &d.Order.PaidAt, &d.Order.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get refund ticket detail %d: %w", ticketID, err)
	}
	d.Content = d.Reason
	if applicant != nil && *applicant != "" {
		d.Applicant = *applicant
	} else {
		d.Applicant = "客服代录"
	}
	d.PaidAmount = d.Order.Amount
	d.Order.OrderNo = d.OrderNo
	d.Order.EventID = d.EventID
	d.Order.EventName = d.EventName
	d.Order.PlayerName = d.PlayerName
	d.Order.IDCardMasked = nil
	return &d, nil
}
