package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
)

// QualificationStore 是总决赛资格（final_qualifications）的数据访问层。
type QualificationStore struct{ db *DB }

// NewQualificationStore 构造资格 store。
func NewQualificationStore(db *DB) *QualificationStore { return &QualificationStore{db: db} }

// QualificationRow 是资格记录 + 选手信息。
type QualificationRow struct {
	Qualification model.FinalQualification
	PlayerNo      string
	PlayerName    string
	BoardLevel    string
	DanceType     string
}

const qualificationSelect = `q.id, q.season_id, q.board_id, q.player_id, q.source, q.reason,
	q.status, q.created_by, q.created_at, q.updated_at,
	pl.player_no, pl.name, b.level, b.dance_type`

const qualificationFrom = `FROM final_qualifications q
	JOIN players pl ON pl.id = q.player_id
	JOIN boards b ON b.id = q.board_id`

// ReplaceAuto 事务内重建自动来源的资格（auto_direct / points），保留手动来源。
func (s *QualificationStore) ReplaceAuto(ctx context.Context, tx *sql.Tx, seasonID, boardID int64, rows []model.FinalQualification) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM final_qualifications
		 WHERE season_id = ? AND board_id = ? AND source IN ('auto_direct', 'points')`,
		seasonID, boardID); err != nil {
		return fmt.Errorf("clear auto qualifications: %w", err)
	}
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO final_qualifications (season_id, board_id, player_id, source, reason, status)
			 VALUES (?, ?, ?, ?, ?, 'valid')`,
			seasonID, boardID, r.PlayerID, r.Source, r.Reason); err != nil {
			return fmt.Errorf("insert qualification: %w", err)
		}
	}
	return nil
}

// ListValidByBoard 返回榜单有效资格。
func (s *QualificationStore) ListValidByBoard(ctx context.Context, boardID int64) ([]QualificationRow, error) {
	return s.listValidByBoard(ctx, s.db.sql, boardID)
}

// ListValidByBoardTx 是事务内版本（单连接池下必须走 tx）。
func (s *QualificationStore) ListValidByBoardTx(ctx context.Context, tx *sql.Tx, boardID int64) ([]QualificationRow, error) {
	return s.listValidByBoard(ctx, tx, boardID)
}

func (s *QualificationStore) listValidByBoard(ctx context.Context, q querier, boardID int64) ([]QualificationRow, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+qualificationSelect+` `+qualificationFrom+`
		 WHERE q.board_id = ? AND q.status = 'valid'
		 ORDER BY CASE q.source WHEN 'auto_direct' THEN 0 WHEN 'manual_direct' THEN 1
		         WHEN 'points' THEN 2 ELSE 3 END, q.id`, boardID)
	if err != nil {
		return nil, fmt.Errorf("list qualifications of board %d: %w", boardID, err)
	}
	defer rows.Close()
	return scanQualificationRows(rows)
}

// ListValidBySeason 返回赛季有效资格。
func (s *QualificationStore) ListValidBySeason(ctx context.Context, seasonID int64) ([]QualificationRow, error) {
	rows, err := s.db.sql.QueryContext(ctx,
		`SELECT `+qualificationSelect+` `+qualificationFrom+`
		 WHERE q.season_id = ? AND q.status = 'valid'
		 ORDER BY b.level, b.dance_type, CASE q.source WHEN 'auto_direct' THEN 0 ELSE 1 END, q.id`,
		seasonID)
	if err != nil {
		return nil, fmt.Errorf("list qualifications of season %d: %w", seasonID, err)
	}
	defer rows.Close()
	return scanQualificationRows(rows)
}

// Insert 写入一条手动资格（valid）。
func (s *QualificationStore) Insert(ctx context.Context, tx *sql.Tx, q model.FinalQualification) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO final_qualifications (season_id, board_id, player_id, source, reason, status, created_by)
		 VALUES (?, ?, ?, ?, ?, 'valid', ?)`,
		q.SeasonID, q.BoardID, q.PlayerID, q.Source, q.Reason, q.CreatedBy)
	if err != nil {
		return 0, fmt.Errorf("insert qualification: %w", err)
	}
	return res.LastInsertId()
}

// Cancel 把一条有效资格置为 canceled（手动取消）。
func (s *QualificationStore) Cancel(ctx context.Context, tx *sql.Tx, id int64) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE final_qualifications SET status = 'canceled' WHERE id = ? AND status = 'valid'`, id)
	if err != nil {
		return fmt.Errorf("cancel qualification %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Get 按 id 查询资格。
func (s *QualificationStore) Get(ctx context.Context, id int64) (*model.FinalQualification, error) {
	var q model.FinalQualification
	err := s.db.sql.QueryRowContext(ctx,
		`SELECT id, season_id, board_id, player_id, source, reason, status, created_by, created_at, updated_at
		 FROM final_qualifications WHERE id = ?`, id).
		Scan(&q.ID, &q.SeasonID, &q.BoardID, &q.PlayerID, &q.Source, &q.Reason,
			&q.Status, &q.CreatedBy, &q.CreatedAt, &q.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get qualification %d: %w", id, err)
	}
	return &q, nil
}

func scanQualificationRows(rows *sql.Rows) ([]QualificationRow, error) {
	list := make([]QualificationRow, 0, 16)
	for rows.Next() {
		var out QualificationRow
		q := &out.Qualification
		if err := rows.Scan(&q.ID, &q.SeasonID, &q.BoardID, &q.PlayerID, &q.Source, &q.Reason,
			&q.Status, &q.CreatedBy, &q.CreatedAt, &q.UpdatedAt,
			&out.PlayerNo, &out.PlayerName, &out.BoardLevel, &out.DanceType); err != nil {
			return nil, err
		}
		list = append(list, out)
	}
	return list, rows.Err()
}
