package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
)

// ResultStore 是成绩（results 表）的数据访问层。
type ResultStore struct{ db *DB }

// NewResultStore 构造成绩 store。
func NewResultStore(db *DB) *ResultStore { return &ResultStore{db: db} }

// ResultRow 是成绩查询的一行（成绩 + 选手/赛事摘要）。
type ResultRow struct {
	Result     model.Result
	PlayerNo   string
	PlayerName string
	AvatarPath *string
	EventName  string
	EventLevel string
	EventDate  *string
	BoardLevel string
	DanceType  string
}

const resultSelectColumns = `r.id, r.event_id, r.board_id, r.player_id, r.institution, r.award,
	r.points, r.prize_amount, r.advanced_to_final, r.remark, r.status, r.import_batch,
	r.created_by, r.created_at, r.updated_at,
	pl.player_no, pl.name, pl.avatar_path, e.name, e.level, e.event_date, b.level, b.dance_type`

const resultFrom = `FROM results r
	JOIN players pl ON pl.id = r.player_id
	JOIN events e ON e.id = r.event_id
	JOIN boards b ON b.id = r.board_id`

// ListByEventBoard 返回某赛事某榜单的全部成绩（含已纠错/作废历史，按状态与 id 排序）。
func (s *ResultStore) ListByEventBoard(ctx context.Context, eventID, boardID int64) ([]ResultRow, error) {
	rows, err := s.db.sql.QueryContext(ctx,
		`SELECT `+resultSelectColumns+` `+resultFrom+`
		 WHERE r.event_id = ? AND r.board_id = ?
		 ORDER BY CASE r.status WHEN 'published' THEN 0 WHEN 'corrected' THEN 1 ELSE 2 END, r.id`,
		eventID, boardID)
	if err != nil {
		return nil, fmt.Errorf("list results of event %d board %d: %w", eventID, boardID, err)
	}
	defer rows.Close()
	return scanResultRows(rows)
}

// CountPublishedByEventBoard 统计某赛事某榜单当前有效成绩条数（参赛人数）。
func (s *ResultStore) CountPublishedByEventBoard(ctx context.Context, tx *sql.Tx, eventID, boardID int64) (int, error) {
	var n int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM results WHERE event_id = ? AND board_id = ? AND status = 'published'`,
		eventID, boardID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count results: %w", err)
	}
	return n, nil
}

// GetPublishedByPlayer 查询某赛事某榜单某选手当前有效成绩（不存在返回 nil）。
func (s *ResultStore) GetPublishedByPlayer(ctx context.Context, tx *sql.Tx, eventID, boardID, playerID int64) (*model.Result, error) {
	r, err := scanResult(tx.QueryRowContext(ctx,
		`SELECT id, event_id, board_id, player_id, institution, award, points, prize_amount,
		        advanced_to_final, remark, status, import_batch, created_by, created_at, updated_at
		 FROM results WHERE event_id = ? AND board_id = ? AND player_id = ? AND status = 'published'`,
		eventID, boardID, playerID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get published result: %w", err)
	}
	return r, nil
}

// Insert 写入一条已发布成绩，返回 id。
func (s *ResultStore) Insert(ctx context.Context, tx *sql.Tx, p ResultParams) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO results (event_id, board_id, player_id, institution, award, points,
		        prize_amount, advanced_to_final, remark, status, import_batch, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'published', ?, ?)`,
		p.EventID, p.BoardID, p.PlayerID, p.Institution, p.Award, p.Points,
		p.PrizeAmount, p.AdvancedToFinal, p.Remark, p.ImportBatch, p.CreatedBy)
	if err != nil {
		return 0, fmt.Errorf("insert result: %w", err)
	}
	return res.LastInsertId()
}

// UpdatePublished 就地更新一条有效成绩（重复导入同一选手时）。
func (s *ResultStore) UpdatePublished(ctx context.Context, tx *sql.Tx, id int64, p ResultParams) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE results SET institution = ?, award = ?, points = ?, prize_amount = ?,
		        advanced_to_final = ?, remark = ?, import_batch = ?
		 WHERE id = ? AND status = 'published'`,
		p.Institution, p.Award, p.Points, p.PrizeAmount, p.AdvancedToFinal,
		p.Remark, p.ImportBatch, id); err != nil {
		return fmt.Errorf("update result %d: %w", id, err)
	}
	return nil
}

// MarkCorrected 把一条有效成绩置为已纠错（保留留痕）。
func (s *ResultStore) MarkCorrected(ctx context.Context, tx *sql.Tx, id int64) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE results SET status = 'corrected' WHERE id = ? AND status = 'published'`, id)
	if err != nil {
		return fmt.Errorf("mark result %d corrected: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	return nil
}

// GetPublished 按 id 查询一条有效成绩。
func (s *ResultStore) GetPublished(ctx context.Context, id int64) (*model.Result, error) {
	r, err := scanResult(s.db.sql.QueryRowContext(ctx,
		`SELECT id, event_id, board_id, player_id, institution, award, points, prize_amount,
		        advanced_to_final, remark, status, import_batch, created_by, created_at, updated_at
		 FROM results WHERE id = ? AND status = 'published'`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get result %d: %w", id, err)
	}
	return r, nil
}

// ListPublishedBySeasonBoard 返回赛季某榜单全部有效成绩（重算积分用）。
func (s *ResultStore) ListPublishedBySeasonBoard(ctx context.Context, tx *sql.Tx, seasonID, boardID int64) ([]ResultRow, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT `+resultSelectColumns+` `+resultFrom+`
		 WHERE b.season_id = ? AND r.board_id = ? AND r.status = 'published'
		 ORDER BY r.player_id, r.event_id`, seasonID, boardID)
	if err != nil {
		return nil, fmt.Errorf("list season board results: %w", err)
	}
	defer rows.Close()
	return scanResultRows(rows)
}

// ListPublishedBySeason 返回赛季全部有效成绩（机构榜聚合用，事务内）。
func (s *ResultStore) ListPublishedBySeason(ctx context.Context, tx *sql.Tx, seasonID int64) ([]ResultRow, error) {
	return s.listPublishedBySeason(ctx, tx, seasonID)
}

// ListPublishedBySeasonRead 是事务外只读版本（后台榜单读取）。
func (s *ResultStore) ListPublishedBySeasonRead(ctx context.Context, seasonID int64) ([]ResultRow, error) {
	return s.listPublishedBySeason(ctx, s.db.sql, seasonID)
}

func (s *ResultStore) listPublishedBySeason(ctx context.Context, q querier, seasonID int64) ([]ResultRow, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+resultSelectColumns+` `+resultFrom+`
		 WHERE b.season_id = ? AND r.status = 'published'
		 ORDER BY r.player_id, r.board_id, r.event_id`, seasonID)
	if err != nil {
		return nil, fmt.Errorf("list season results: %w", err)
	}
	defer rows.Close()
	return scanResultRows(rows)
}

// ListPublishedByPlayer 返回选手在赛季内的全部有效成绩（选手详情页）。
func (s *ResultStore) ListPublishedByPlayer(ctx context.Context, seasonID, playerID int64) ([]ResultRow, error) {
	rows, err := s.db.sql.QueryContext(ctx,
		`SELECT `+resultSelectColumns+` `+resultFrom+`
		 WHERE b.season_id = ? AND r.player_id = ? AND r.status = 'published'
		 ORDER BY e.event_date, r.id`, seasonID, playerID)
	if err != nil {
		return nil, fmt.Errorf("list player results: %w", err)
	}
	defer rows.Close()
	return scanResultRows(rows)
}

// ResultParams 是写入成绩的字段。
type ResultParams struct {
	EventID         int64
	BoardID         int64
	PlayerID        int64
	Institution     *string
	Award           string
	Points          int
	PrizeAmount     int
	AdvancedToFinal bool
	Remark          *string
	ImportBatch     *string
	CreatedBy       *int64
}

func scanResult(row rowScanner) (*model.Result, error) {
	var r model.Result
	if err := row.Scan(&r.ID, &r.EventID, &r.BoardID, &r.PlayerID, &r.Institution,
		&r.Award, &r.Points, &r.PrizeAmount, &r.AdvancedToFinal, &r.Remark,
		&r.Status, &r.ImportBatch, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return nil, err
	}
	return &r, nil
}

func scanResultRows(rows *sql.Rows) ([]ResultRow, error) {
	list := make([]ResultRow, 0, 16)
	for rows.Next() {
		var out ResultRow
		r := &out.Result
		if err := rows.Scan(&r.ID, &r.EventID, &r.BoardID, &r.PlayerID, &r.Institution,
			&r.Award, &r.Points, &r.PrizeAmount, &r.AdvancedToFinal, &r.Remark,
			&r.Status, &r.ImportBatch, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
			&out.PlayerNo, &out.PlayerName, &out.AvatarPath,
			&out.EventName, &out.EventLevel, &out.EventDate,
			&out.BoardLevel, &out.DanceType); err != nil {
			return nil, err
		}
		list = append(list, out)
	}
	return list, rows.Err()
}
