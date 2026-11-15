package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
)

// MiniappRegistrationRow 是小程序「我的报名」查询的一行：
// 报名单 + 订单 + 赛事/组别/选手摘要（T35）。
type MiniappRegistrationRow struct {
	Registration    model.Registration
	Order           model.Order
	EventName       string
	EventLevel      string
	EventDate       *string
	Level           string // event_groups.level
	PlayerName      string
	PlayerNo        string
	MaterialCheckAt *string
}

// miniappRegistrationSelectColumns 是小程序「我的报名」固定查询列，
// 顺序与 scanMiniappRegistration 一致（报名单 21 列 + 订单 14 列 + 摘要 7 列）。
const miniappRegistrationSelectColumns = `r.id, r.event_id, r.group_id, r.player_id, r.user_id,
	r.dance_type, r.institution, r.status, r.source, r.cs_ticket_id, r.photo_path,
	r.audio_path, r.program_name, r.teacher_name, r.contact_phone, r.express_address,
	r.work_intro, r.special_req, r.material_updated_at, r.created_at, r.updated_at,
	o.id, o.order_no, o.registration_id, o.user_id, o.amount, o.status,
	o.source, o.request_id, o.prepay_id, o.paid_at, o.canceled_at, o.cancel_reason,
	o.created_at, o.updated_at,
	e.name, e.level, e.event_date, g.level, p.name, p.player_no, e.material_check_at`

// miniappRegistrationFrom 是「我的报名」固定 JOIN。订单与报名单 1:1，
// 用 INNER JOIN 保证数据异常（无订单）不会把半条报名展示给用户。
const miniappRegistrationFrom = `FROM registrations r
	JOIN orders o ON o.registration_id = r.id
	JOIN events e ON e.id = r.event_id
	JOIN event_groups g ON g.id = r.group_id
	JOIN players p ON p.id = r.player_id`

// miniappRegistrationVisible 是「我的报名」可见范围：仅本人、有效/已退赛报名，
// 订单为已支付/退款中/已退款；待支付与已取消订单不进入小程序页面（API 文档 3.3）。
const miniappRegistrationVisible = `r.user_id = ?
	AND r.status IN ('active', 'withdrawn')
	AND o.status IN ('paid', 'refunding', 'refunded')`

// ListMine 分页查询用户「我的报名」（T35，API 文档 3.3）。
// 按报名单创建时间倒序返回，结果稳定；无记录返回空切片。
func (s *RegistrationStore) ListMine(ctx context.Context, userID int64, page, pageSize int) ([]MiniappRegistrationRow, int64, error) {
	var total int64
	if err := s.db.sql.QueryRowContext(ctx,
		`SELECT COUNT(1) `+miniappRegistrationFrom+` WHERE `+miniappRegistrationVisible,
		userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count miniapp registrations of user %d: %w", userID, err)
	}
	if total == 0 {
		return []MiniappRegistrationRow{}, 0, nil
	}

	rows, err := s.db.sql.QueryContext(ctx,
		`SELECT `+miniappRegistrationSelectColumns+` `+miniappRegistrationFrom+`
		 WHERE `+miniappRegistrationVisible+`
		 ORDER BY r.created_at DESC, r.id DESC
		 LIMIT ? OFFSET ?`, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("query miniapp registrations of user %d: %w", userID, err)
	}
	defer rows.Close()

	list := make([]MiniappRegistrationRow, 0, pageSize)
	for rows.Next() {
		row, err := scanMiniappRegistration(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan miniapp registration of user %d: %w", userID, err)
		}
		list = append(list, *row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate miniapp registrations of user %d: %w", userID, err)
	}
	return list, total, nil
}

// GetMine 查询用户的一条「我的报名」详情（T35，API 文档 3.4）。
// 不区分不存在/非本人/不可见状态，统一返回 ErrNotFound 防枚举。
func (s *RegistrationStore) GetMine(ctx context.Context, userID, registrationID int64) (*MiniappRegistrationRow, error) {
	row, err := scanMiniappRegistration(s.db.sql.QueryRowContext(ctx,
		`SELECT `+miniappRegistrationSelectColumns+` `+miniappRegistrationFrom+`
		 WHERE r.id = ? AND `+miniappRegistrationVisible,
		registrationID, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query miniapp registration %d of user %d: %w", registrationID, userID, err)
	}
	return row, nil
}

// scanMiniappRegistration 扫描「我的报名」一行（列顺序与常量一致）。
func scanMiniappRegistration(row rowScanner) (*MiniappRegistrationRow, error) {
	var out MiniappRegistrationRow
	r := &out.Registration
	o := &out.Order
	if err := row.Scan(&r.ID, &r.EventID, &r.GroupID, &r.PlayerID, &r.UserID,
		&r.DanceType, &r.Institution, &r.Status, &r.Source, &r.CSTicketID,
		&r.PhotoPath, &r.AudioPath, &r.ProgramName, &r.TeacherName,
		&r.ContactPhone, &r.ExpressAddress, &r.WorkIntro, &r.SpecialReq,
		&r.MaterialUpdatedAt, &r.CreatedAt, &r.UpdatedAt,
		&o.ID, &o.OrderNo, &o.RegistrationID, &o.UserID, &o.Amount, &o.Status,
		&o.Source, &o.RequestID, &o.PrepayID, &o.PaidAt, &o.CanceledAt, &o.CancelReason,
		&o.CreatedAt, &o.UpdatedAt,
		&out.EventName, &out.EventLevel, &out.EventDate, &out.Level,
		&out.PlayerName, &out.PlayerNo, &out.MaterialCheckAt); err != nil {
		return nil, err
	}
	return &out, nil
}
