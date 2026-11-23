package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
)

// PointsStore 是积分快照（player_points / institution_points）的数据访问层。
type PointsStore struct{ db *DB }

// NewPointsStore 构造积分快照 store。
func NewPointsStore(db *DB) *PointsStore { return &PointsStore{db: db} }

// PlayerPointRow 是选手积分快照 + 选手信息（榜单展示用）。
type PlayerPointRow struct {
	PlayerPoint model.PlayerPoint
	PlayerNo    string
	PlayerName  string
	AvatarPath  *string
}

// ReplacePlayerPoints 事务内替换某榜单的快照（先删后插）。
// 保留已设置的 display_name / hidden：调用方传入的前值由本方法从旧行合并。
func (s *PointsStore) ReplacePlayerPoints(ctx context.Context, tx *sql.Tx, seasonID, boardID int64, rows []model.PlayerPoint) error {
	// 保留人工设置（展示名/隐藏）。
	keep := map[int64]struct {
		name   *string
		hidden bool
	}{}
	old, err := tx.QueryContext(ctx,
		`SELECT player_id, display_name, hidden FROM player_points WHERE season_id = ? AND board_id = ?`,
		seasonID, boardID)
	if err != nil {
		return fmt.Errorf("load existing player points: %w", err)
	}
	for old.Next() {
		var pid int64
		var name *string
		var hidden int
		if err := old.Scan(&pid, &name, &hidden); err != nil {
			old.Close()
			return fmt.Errorf("scan existing player points: %w", err)
		}
		keep[pid] = struct {
			name   *string
			hidden bool
		}{name, hidden == 1}
	}
	old.Close()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM player_points WHERE season_id = ? AND board_id = ?`, seasonID, boardID); err != nil {
		return fmt.Errorf("clear player points: %w", err)
	}
	for _, r := range rows {
		var name *string
		hidden := 0
		if k, ok := keep[r.PlayerID]; ok {
			name, hidden = k.name, boolToInt(k.hidden)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO player_points (season_id, board_id, player_id, total_points,
			        used_event_ids, "rank", display_name, hidden, computed_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			seasonID, boardID, r.PlayerID, r.TotalPoints, r.UsedEventIDs,
			r.Rank, name, hidden, r.ComputedAt); err != nil {
			return fmt.Errorf("insert player point: %w", err)
		}
	}
	return nil
}

// ListPlayerPointsByBoard 返回榜单快照（按 rank 升序，rank 为空的排在后）。
func (s *PointsStore) ListPlayerPointsByBoard(ctx context.Context, boardID int64) ([]PlayerPointRow, error) {
	return s.listPlayerPointsByBoard(ctx, s.db.sql, boardID)
}

// ListPlayerPointsByBoardTx 是事务内版本（单连接池下必须走 tx）。
func (s *PointsStore) ListPlayerPointsByBoardTx(ctx context.Context, tx *sql.Tx, boardID int64) ([]PlayerPointRow, error) {
	return s.listPlayerPointsByBoard(ctx, tx, boardID)
}

func (s *PointsStore) listPlayerPointsByBoard(ctx context.Context, q querier, boardID int64) ([]PlayerPointRow, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT pp.id, pp.season_id, pp.board_id, pp.player_id, pp.total_points,
		        pp.used_event_ids, pp."rank", pp.display_name, pp.hidden, pp.computed_at,
		        pl.player_no, pl.name, pl.avatar_path
		 FROM player_points pp
		 JOIN players pl ON pl.id = pp.player_id
		 WHERE pp.board_id = ?
		 ORDER BY pp."rank" IS NULL, pp."rank", pp.total_points DESC, pp.player_id`,
		boardID)
	if err != nil {
		return nil, fmt.Errorf("list player points of board %d: %w", boardID, err)
	}
	defer rows.Close()

	list := make([]PlayerPointRow, 0, 32)
	for rows.Next() {
		var out PlayerPointRow
		p := &out.PlayerPoint
		var hidden int
		if err := rows.Scan(&p.ID, &p.SeasonID, &p.BoardID, &p.PlayerID, &p.TotalPoints,
			&p.UsedEventIDs, &p.Rank, &p.DisplayName, &hidden, &p.ComputedAt,
			&out.PlayerNo, &out.PlayerName, &out.AvatarPath); err != nil {
			return nil, err
		}
		p.Hidden = hidden == 1
		list = append(list, out)
	}
	return list, rows.Err()
}

// GetPlayerPoint 查询某榜单某选手的快照。
func (s *PointsStore) GetPlayerPoint(ctx context.Context, boardID, playerID int64) (*PlayerPointRow, error) {
	var out PlayerPointRow
	p := &out.PlayerPoint
	var hidden int
	err := s.db.sql.QueryRowContext(ctx,
		`SELECT pp.id, pp.season_id, pp.board_id, pp.player_id, pp.total_points,
		        pp.used_event_ids, pp."rank", pp.display_name, pp.hidden, pp.computed_at,
		        pl.player_no, pl.name, pl.avatar_path
		 FROM player_points pp JOIN players pl ON pl.id = pp.player_id
		 WHERE pp.board_id = ? AND pp.player_id = ?`, boardID, playerID).
		Scan(&p.ID, &p.SeasonID, &p.BoardID, &p.PlayerID, &p.TotalPoints,
			&p.UsedEventIDs, &p.Rank, &p.DisplayName, &hidden, &p.ComputedAt,
			&out.PlayerNo, &out.PlayerName, &out.AvatarPath)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get player point: %w", err)
	}
	p.Hidden = hidden == 1
	return &out, nil
}

// UpdateDisplayName 设置展示名（空串表示恢复真名）。
func (s *PointsStore) UpdateDisplayName(ctx context.Context, tx *sql.Tx, boardID, playerID int64, name *string) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE player_points SET display_name = ? WHERE board_id = ? AND player_id = ?`,
		name, boardID, playerID)
	if err != nil {
		return fmt.Errorf("update display name: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateHidden 设置隐藏上榜。
func (s *PointsStore) UpdateHidden(ctx context.Context, tx *sql.Tx, boardID, playerID int64, hidden bool) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE player_points SET hidden = ? WHERE board_id = ? AND player_id = ?`,
		boolToInt(hidden), boardID, playerID)
	if err != nil {
		return fmt.Errorf("update hidden: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ReplaceInstitutionPoints 事务内替换赛季机构快照。
func (s *PointsStore) ReplaceInstitutionPoints(ctx context.Context, tx *sql.Tx, seasonID int64, rows []model.InstitutionPoint) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM institution_points WHERE season_id = ?`, seasonID); err != nil {
		return fmt.Errorf("clear institution points: %w", err)
	}
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO institution_points (season_id, institution_name, total_points,
			        player_count, "rank", computed_at) VALUES (?, ?, ?, ?, ?, ?)`,
			seasonID, r.InstitutionName, r.TotalPoints, r.PlayerCount, r.Rank, r.ComputedAt); err != nil {
			return fmt.Errorf("insert institution point: %w", err)
		}
	}
	return nil
}

// ListInstitutionPoints 返回赛季机构榜快照（按 rank 升序）。
func (s *PointsStore) ListInstitutionPoints(ctx context.Context, seasonID int64) ([]model.InstitutionPoint, error) {
	rows, err := s.db.sql.QueryContext(ctx,
		`SELECT id, season_id, institution_name, total_points, player_count, "rank", computed_at
		 FROM institution_points WHERE season_id = ?
		 ORDER BY "rank" IS NULL, "rank", total_points DESC`, seasonID)
	if err != nil {
		return nil, fmt.Errorf("list institution points: %w", err)
	}
	defer rows.Close()
	list := make([]model.InstitutionPoint, 0, 16)
	for rows.Next() {
		var p model.InstitutionPoint
		if err := rows.Scan(&p.ID, &p.SeasonID, &p.InstitutionName, &p.TotalPoints,
			&p.PlayerCount, &p.Rank, &p.ComputedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// MaxComputedAt 返回赛季内最新快照时间（榜单更新时间）。
func (s *PointsStore) MaxComputedAt(ctx context.Context, seasonID int64) (*string, error) {
	var at *string
	if err := s.db.sql.QueryRowContext(ctx,
		`SELECT MAX(computed_at) FROM player_points WHERE season_id = ?`, seasonID).Scan(&at); err != nil {
		return nil, fmt.Errorf("max computed_at: %w", err)
	}
	return at, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// GetPlayerPointByID 按快照 id 查询（小程序选手详情以条目 id 定位）。
func (s *PointsStore) GetPlayerPointByID(ctx context.Context, id int64) (*PlayerPointRow, error) {
	var out PlayerPointRow
	p := &out.PlayerPoint
	var hidden int
	err := s.db.sql.QueryRowContext(ctx,
		`SELECT pp.id, pp.season_id, pp.board_id, pp.player_id, pp.total_points,
		        pp.used_event_ids, pp."rank", pp.display_name, pp.hidden, pp.computed_at,
		        pl.player_no, pl.name, pl.avatar_path
		 FROM player_points pp JOIN players pl ON pl.id = pp.player_id
		 WHERE pp.id = ?`, id).
		Scan(&p.ID, &p.SeasonID, &p.BoardID, &p.PlayerID, &p.TotalPoints,
			&p.UsedEventIDs, &p.Rank, &p.DisplayName, &hidden, &p.ComputedAt,
			&out.PlayerNo, &out.PlayerName, &out.AvatarPath)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get player point %d: %w", id, err)
	}
	p.Hidden = hidden == 1
	return &out, nil
}
