package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
)

// BoardStore 是榜单（boards 表）的数据访问层。
type BoardStore struct{ db *DB }

// NewBoardStore 构造榜单 store。
func NewBoardStore(db *DB) *BoardStore { return &BoardStore{db: db} }

const boardColumns = `id, season_id, level, dance_type, status, published_at, created_at, updated_at`

// EnsureForSeason 幂等写入一个赛季的 6 个榜单（U11/U13/U15 × 古典/民族民间）。
// 已存在的组合不覆盖，返回当前榜单列表。
func (s *BoardStore) EnsureForSeason(ctx context.Context, tx *sql.Tx, seasonID int64) error {
	for _, level := range []string{"U11", "U13", "U15"} {
		for _, dance := range []string{"classical", "folk"} {
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO boards (season_id, level, dance_type) VALUES (?, ?, ?)`,
				seasonID, level, dance); err != nil {
				return fmt.Errorf("ensure board %s/%s of season %d: %w", level, dance, seasonID, err)
			}
		}
	}
	return nil
}

// ListBySeason 返回赛季全部榜单（按组别、舞种排序）。
func (s *BoardStore) ListBySeason(ctx context.Context, seasonID int64) ([]model.Board, error) {
	return s.listBySeason(ctx, s.db.sql, seasonID)
}

// ListBySeasonTx 是事务内版本（单连接池下事务内必须走 tx，否则自死锁）。
func (s *BoardStore) ListBySeasonTx(ctx context.Context, tx *sql.Tx, seasonID int64) ([]model.Board, error) {
	return s.listBySeason(ctx, tx, seasonID)
}

func (s *BoardStore) listBySeason(ctx context.Context, q querier, seasonID int64) ([]model.Board, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+boardColumns+` FROM boards WHERE season_id = ?
		 ORDER BY CASE level WHEN 'U11' THEN 1 WHEN 'U13' THEN 2 ELSE 3 END, dance_type`,
		seasonID)
	if err != nil {
		return nil, fmt.Errorf("list boards of season %d: %w", seasonID, err)
	}
	defer rows.Close()
	return scanBoards(rows)
}

// Get 按 id 查询榜单。
func (s *BoardStore) Get(ctx context.Context, id int64) (*model.Board, error) {
	b, err := scanBoard(s.db.sql.QueryRowContext(ctx,
		`SELECT `+boardColumns+` FROM boards WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get board %d: %w", id, err)
	}
	return b, nil
}

// GetByKey 按赛季 × 组别 × 舞种查询榜单。
func (s *BoardStore) GetByKey(ctx context.Context, seasonID int64, level, danceType string) (*model.Board, error) {
	b, err := scanBoard(s.db.sql.QueryRowContext(ctx,
		`SELECT `+boardColumns+` FROM boards WHERE season_id = ? AND level = ? AND dance_type = ?`,
		seasonID, level, danceType))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get board %d/%s/%s: %w", seasonID, level, danceType, err)
	}
	return b, nil
}

// MarkPublished 记录榜单发布时刻（成绩发布后调用）。
func (s *BoardStore) MarkPublished(ctx context.Context, tx *sql.Tx, id int64, at string) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE boards SET published_at = ? WHERE id = ?`, at, id); err != nil {
		return fmt.Errorf("mark board %d published: %w", id, err)
	}
	return nil
}

func scanBoard(row rowScanner) (*model.Board, error) {
	var b model.Board
	if err := row.Scan(&b.ID, &b.SeasonID, &b.Level, &b.DanceType, &b.Status,
		&b.PublishedAt, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return nil, err
	}
	return &b, nil
}

func scanBoards(rows *sql.Rows) ([]model.Board, error) {
	list := make([]model.Board, 0, 6)
	for rows.Next() {
		b, err := scanBoard(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *b)
	}
	return list, rows.Err()
}
