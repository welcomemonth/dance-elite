package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
)

// PrizeStore 是奖金明细（prizes）的数据访问层。
type PrizeStore struct{ db *DB }

// NewPrizeStore 构造奖金 store。
func NewPrizeStore(db *DB) *PrizeStore { return &PrizeStore{db: db} }

// PrizeRow 是奖金明细 + 选手/赛事摘要。
type PrizeRow struct {
	Prize      model.Prize
	PlayerNo   string
	PlayerName string
	EventName  *string
	BoardLevel *string
	DanceType  *string
}

const prizeSelect = `pz.id, pz.season_id, pz.event_id, pz.board_id, pz.player_id,
	pz.institution_name, pz.award, pz.amount, pz.status, pz.granted_at, pz.grantee_note,
	pz.created_at, pz.updated_at, pl.player_no, pl.name, e.name, b.level, b.dance_type`

const prizeFrom = `FROM prizes pz
	LEFT JOIN players pl ON pl.id = pz.player_id
	LEFT JOIN events e ON e.id = pz.event_id
	LEFT JOIN boards b ON b.id = pz.board_id`

// ReplacePendingForEventBoard 事务内重建某赛事某榜单的未发放奖金（保留已发放/已跳过）。
func (s *PrizeStore) ReplacePendingForEventBoard(ctx context.Context, tx *sql.Tx,
	eventID, boardID int64, rows []model.Prize) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM prizes WHERE event_id = ? AND board_id = ? AND status = 'pending'`,
		eventID, boardID); err != nil {
		return fmt.Errorf("clear pending prizes: %w", err)
	}
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO prizes (season_id, event_id, board_id, player_id, institution_name,
			        award, amount, status) VALUES (?, ?, ?, ?, ?, ?, ?, 'pending')`,
			r.SeasonID, r.EventID, r.BoardID, r.PlayerID, r.InstitutionName,
			r.Award, r.Amount); err != nil {
			return fmt.Errorf("insert prize: %w", err)
		}
	}
	return nil
}

// ListBySeason 返回赛季奖金明细。
func (s *PrizeStore) ListBySeason(ctx context.Context, seasonID int64, status string) ([]PrizeRow, error) {
	where := "pz.season_id = ?"
	args := []any{seasonID}
	if status != "" {
		where += " AND pz.status = ?"
		args = append(args, status)
	}
	rows, err := s.db.sql.QueryContext(ctx,
		`SELECT `+prizeSelect+` `+prizeFrom+` WHERE `+where+` ORDER BY pz.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list prizes: %w", err)
	}
	defer rows.Close()
	return scanPrizeRows(rows)
}

// Get 按 id 查询奖金。
func (s *PrizeStore) Get(ctx context.Context, id int64) (*model.Prize, error) {
	var p model.Prize
	err := s.db.sql.QueryRowContext(ctx,
		`SELECT id, season_id, event_id, board_id, player_id, institution_name, award, amount,
		        status, granted_at, grantee_note, created_at, updated_at
		 FROM prizes WHERE id = ?`, id).
		Scan(&p.ID, &p.SeasonID, &p.EventID, &p.BoardID, &p.PlayerID, &p.InstitutionName,
			&p.Award, &p.Amount, &p.Status, &p.GrantedAt, &p.GranteeNote, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get prize %d: %w", id, err)
	}
	return &p, nil
}

// UpdateAmount 调整未发放奖金金额。
func (s *PrizeStore) UpdateAmount(ctx context.Context, tx *sql.Tx, id int64, amount int) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE prizes SET amount = ? WHERE id = ? AND status = 'pending'`, amount, id)
	if err != nil {
		return fmt.Errorf("update prize amount: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	return nil
}

// MarkGranted 登记发放（pending → granted）。
func (s *PrizeStore) MarkGranted(ctx context.Context, tx *sql.Tx, id int64, at, note string) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE prizes SET status = 'granted', granted_at = ?, grantee_note = ?
		 WHERE id = ? AND status = 'pending'`, at, note, id)
	if err != nil {
		return fmt.Errorf("grant prize %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	return nil
}

func scanPrizeRows(rows *sql.Rows) ([]PrizeRow, error) {
	list := make([]PrizeRow, 0, 32)
	for rows.Next() {
		var out PrizeRow
		p := &out.Prize
		if err := rows.Scan(&p.ID, &p.SeasonID, &p.EventID, &p.BoardID, &p.PlayerID,
			&p.InstitutionName, &p.Award, &p.Amount, &p.Status, &p.GrantedAt, &p.GranteeNote,
			&p.CreatedAt, &p.UpdatedAt, &out.PlayerNo, &out.PlayerName,
			&out.EventName, &out.BoardLevel, &out.DanceType); err != nil {
			return nil, err
		}
		list = append(list, out)
	}
	return list, rows.Err()
}
