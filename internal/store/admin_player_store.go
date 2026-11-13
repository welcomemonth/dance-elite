package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
)

// GetPlayerBrief 读取单个选手档案（后台详情页头部，API 文档 13.2）。
// 已合并档案也允许查看（提示 merged_into）。不存在返回 ErrNotFound。
func (s *PlayerStore) GetPlayerBrief(ctx context.Context, playerID int64) (*model.AdminPlayerBriefItem, error) {
	var item model.AdminPlayerBriefItem
	err := s.db.sql.QueryRowContext(ctx, `
SELECT p.id, p.player_no, p.name, p.id_type, p.birth_date, p.gender, p.merged_into,
 (SELECT COUNT(*) FROM registrations r WHERE r.player_id = p.id),
 (SELECT COUNT(*) FROM results res WHERE res.player_id = p.id AND res.status = 'published'),
 COALESCE((SELECT r2.institution FROM registrations r2
   WHERE r2.player_id = p.id ORDER BY r2.id DESC LIMIT 1), '')
FROM players p WHERE p.id = ?`, playerID).Scan(&item.PlayerID, &item.PlayerNo, &item.Name,
		&item.IDType, &item.BirthDate, &item.Gender, &item.MergedInto,
		&item.RegistrationCount, &item.ResultCount, &item.Institution)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get player brief %d: %w", playerID, err)
	}
	item.IDCardMasked = nil
	return &item, nil
}

// ListPlayerRegistrations 查询选手历史报名（详情页 registrations，API 文档 13.2）。
func (s *PlayerStore) ListPlayerRegistrations(ctx context.Context, playerID int64) ([]model.AdminPlayerRegistration, error) {
	rows, err := s.db.sql.QueryContext(ctx, `
SELECT COALESCE(o.order_no, ''), e.name, g.level, g.dance_type, r.status, r.created_at
FROM registrations r
JOIN events e ON e.id = r.event_id
JOIN event_groups g ON g.id = r.group_id
LEFT JOIN orders o ON o.registration_id = r.id
WHERE r.player_id = ?
ORDER BY r.id DESC`, playerID)
	if err != nil {
		return nil, fmt.Errorf("list player registrations %d: %w", playerID, err)
	}
	defer rows.Close()

	list := make([]model.AdminPlayerRegistration, 0, 8)
	for rows.Next() {
		var r model.AdminPlayerRegistration
		if err := rows.Scan(&r.OrderNo, &r.EventName, &r.Level, &r.DanceType, &r.Status, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan player registration: %w", err)
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

// ListPlayerResults 查询选手已发布成绩（详情页 results_summary，API 文档 13.2）。
func (s *PlayerStore) ListPlayerResults(ctx context.Context, playerID int64) ([]model.AdminPlayerResultSummary, error) {
	rows, err := s.db.sql.QueryContext(ctx, `
SELECT res.award, e.name, res.points, res.created_at
FROM results res
JOIN events e ON e.id = res.event_id
WHERE res.player_id = ? AND res.status = 'published'
ORDER BY res.id DESC`, playerID)
	if err != nil {
		return nil, fmt.Errorf("list player results %d: %w", playerID, err)
	}
	defer rows.Close()

	list := make([]model.AdminPlayerResultSummary, 0, 8)
	for rows.Next() {
		var r model.AdminPlayerResultSummary
		if err := rows.Scan(&r.Award, &r.EventName, &r.Points, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan player result: %w", err)
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

// GetPlayerPointsSummary 查询选手最近赛季的积分汇总（详情页 points_summary）。
// 无积分记录时返回零值对象（season_name 为空）。
func (s *PlayerStore) GetPlayerPointsSummary(ctx context.Context, playerID int64) (model.AdminPlayerPointsSummary, error) {
	var out model.AdminPlayerPointsSummary
	row := s.db.sql.QueryRowContext(ctx, `
SELECT COALESCE(s.name, ''), COALESCE(SUM(pp.total_points), 0),
       COUNT(DISTINCT pp.board_id)
FROM player_points pp
JOIN seasons s ON s.id = pp.season_id
WHERE pp.player_id = ?
GROUP BY pp.season_id
ORDER BY pp.season_id DESC
LIMIT 1`, playerID)
	if err := row.Scan(&out.SeasonName, &out.TotalPoints, &out.EventCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, nil
		}
		return out, fmt.Errorf("player points summary %d: %w", playerID, err)
	}
	return out, nil
}

// ListPlayerDuplicates 查询同证件号哈希的重复档案（详情页 duplicates，API 文档 13.2）。
// 无证件号档案（哈希为空）无法判重，返回空列表。
func (s *PlayerStore) ListPlayerDuplicates(ctx context.Context, playerID int64) ([]model.AdminPlayerDuplicate, error) {
	rows, err := s.db.sql.QueryContext(ctx, `
SELECT d.id, d.player_no, d.name, d.created_at
FROM players p
JOIN players d ON d.id_card_hash = p.id_card_hash
WHERE p.id = ? AND d.id <> ? AND d.merged_into IS NULL
ORDER BY d.id`, playerID, playerID)
	if err != nil {
		return nil, fmt.Errorf("list player duplicates %d: %w", playerID, err)
	}
	defer rows.Close()

	list := make([]model.AdminPlayerDuplicate, 0, 4)
	for rows.Next() {
		var d model.AdminPlayerDuplicate
		if err := rows.Scan(&d.PlayerID, &d.PlayerNo, &d.Name, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan player duplicate: %w", err)
		}
		list = append(list, d)
	}
	return list, rows.Err()
}
