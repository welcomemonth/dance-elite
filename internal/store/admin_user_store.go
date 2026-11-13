package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// AdminWxUserFilter 是后台用户列表的筛选与分页条件（API 文档 13.1）。
type AdminWxUserFilter struct {
	Keyword  string // 昵称 / 手机号
	Status   string // active / blocked / 空
	Page     int
	PageSize int
}

// ListWxUsers 分页查询微信用户列表（含报名次数聚合）。
func (s *UserStore) ListWxUsers(ctx context.Context, f AdminWxUserFilter) ([]model.AdminWxUserItem, int64, error) {
	where := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		where = append(where, "(u.nickname LIKE ? OR u.phone LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like)
	}
	if f.Status != "" {
		where = append(where, "u.status = ?")
		args = append(args, f.Status)
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := s.db.sql.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users u`+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count wx users: %w", err)
	}

	rows, err := s.db.sql.QueryContext(ctx, `
SELECT u.id, COALESCE(u.nickname, ''), COALESCE(u.avatar_url, ''), u.phone,
       u.status, u.created_at, u.last_login_at,
 (SELECT COUNT(*) FROM registrations r WHERE r.user_id = u.id)
FROM users u`+whereSQL+` ORDER BY u.id DESC LIMIT ? OFFSET ?`,
		append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list wx users: %w", err)
	}
	defer rows.Close()

	list := make([]model.AdminWxUserItem, 0, f.PageSize)
	for rows.Next() {
		var item model.AdminWxUserItem
		var phone *string
		if err := rows.Scan(&item.ID, &item.Nickname, &item.AvatarURL, &phone,
			&item.Status, &item.CreatedAt, &item.LastLoginAt, &item.RegistrationCount); err != nil {
			return nil, 0, fmt.Errorf("scan wx user: %w", err)
		}
		if phone != nil {
			item.PhoneMasked = utils.MaskPhone(*phone)
		} else {
			item.PhoneMasked = ""
		}
		list = append(list, item)
	}
	return list, total, rows.Err()
}

// AdminPlayerFilter 是后台选手档案列表的筛选与分页条件（API 文档 13.2）。
type AdminPlayerFilter struct {
	Keyword  string // 姓名 / 选手编号
	Page     int
	PageSize int
}

// ListPlayerBriefs 分页查询选手档案列表（含报名/成绩计数与最近机构）。
// 已合并（merged_into 非空）的档案不展示。
func (s *PlayerStore) ListPlayerBriefs(ctx context.Context, f AdminPlayerFilter) ([]model.AdminPlayerBriefItem, int64, error) {
	where := []string{"p.merged_into IS NULL"}
	args := make([]any, 0, 3)
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		where = append(where, "(p.name LIKE ? OR p.player_no LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like)
	}
	whereSQL := " WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := s.db.sql.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM players p`+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count players: %w", err)
	}

	rows, err := s.db.sql.QueryContext(ctx, `
SELECT p.id, p.player_no, p.name, p.id_type, p.birth_date, p.gender, p.merged_into,
 (SELECT COUNT(*) FROM registrations r WHERE r.player_id = p.id),
 (SELECT COUNT(*) FROM results res WHERE res.player_id = p.id AND res.status = 'published'),
 COALESCE((SELECT r2.institution FROM registrations r2
   WHERE r2.player_id = p.id ORDER BY r2.id DESC LIMIT 1), '')
FROM players p`+whereSQL+` ORDER BY p.id DESC LIMIT ? OFFSET ?`,
		append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list player briefs: %w", err)
	}
	defer rows.Close()

	list := make([]model.AdminPlayerBriefItem, 0, f.PageSize)
	for rows.Next() {
		var item model.AdminPlayerBriefItem
		if err := rows.Scan(&item.PlayerID, &item.PlayerNo, &item.Name, &item.IDType,
			&item.BirthDate, &item.Gender, &item.MergedInto, &item.RegistrationCount,
			&item.ResultCount, &item.Institution); err != nil {
			return nil, 0, fmt.Errorf("scan player brief: %w", err)
		}
		// 证件号只存 HMAC 哈希，不可逆推脱敏形态，按文档允许输出 null。
		item.IDCardMasked = nil
		list = append(list, item)
	}
	return list, total, rows.Err()
}
