package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
)

// AudioReviewRow 是音频审核 H5 查询的原始行（音频路径由 service 映射为 URL）。
type AudioReviewRow struct {
	PlayerName  string
	ProgramName string
	AudioPath   string
	UpdatedAt   string
}

// SetAudioReviewToken 写入赛事音频审核 token（重新生成即覆盖旧值，旧链接立即失效）。
func (s *EventStore) SetAudioReviewToken(ctx context.Context, tx *sql.Tx, eventID int64, token string) error {
	res, err := tx.ExecContext(ctx, `UPDATE events SET audio_review_token = ? WHERE id = ?`, token, eventID)
	if err != nil {
		return fmt.Errorf("set audio review token of event %d: %w", eventID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected of event %d token: %w", eventID, err)
	}
	if n == 0 {
		var cnt int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM events WHERE id = ?`, eventID).Scan(&cnt); err != nil {
			return fmt.Errorf("check existence of event %d: %w", eventID, err)
		}
		if cnt == 0 {
			return ErrNotFound
		}
	}
	return nil
}

// GetByAudioReviewToken 按 token 查询赛事；不存在返回 ErrNotFound。
func (s *EventStore) GetByAudioReviewToken(ctx context.Context, token string) (*model.Event, error) {
	row := s.db.sql.QueryRowContext(ctx, `SELECT `+eventColumns+` FROM events WHERE audio_review_token = ?`, token)
	e, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get event by audio review token: %w", err)
	}
	return e, nil
}

// ListAudioReview 返回赛事下已上传音频的 active 报名（可按姓名过滤）。
func (s *EventStore) ListAudioReview(ctx context.Context, eventID int64, keyword string) ([]AudioReviewRow, error) {
	query := `SELECT pl.name, COALESCE(r.program_name, ''), r.audio_path,
		COALESCE(r.material_updated_at, r.updated_at)
FROM registrations r
JOIN players pl ON pl.id = r.player_id
WHERE r.event_id = ? AND r.status = 'active'
  AND r.audio_path IS NOT NULL AND r.audio_path <> ''`
	args := []any{eventID}
	if keyword != "" {
		query += ` AND pl.name LIKE ?`
		args = append(args, "%"+keyword+"%")
	}
	query += ` ORDER BY pl.name, r.id`

	rows, err := s.db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audio review of event %d: %w", eventID, err)
	}
	defer rows.Close()

	list := make([]AudioReviewRow, 0, 16)
	for rows.Next() {
		var item AudioReviewRow
		if err := rows.Scan(&item.PlayerName, &item.ProgramName, &item.AudioPath, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan audio review row: %w", err)
		}
		list = append(list, item)
	}
	return list, rows.Err()
}
