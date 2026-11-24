package store

import (
	"context"
	"fmt"
	"strings"
)

// MaterialArchiveRow 是媒体归档导出所需的一条报名资料（按报名单粒度）。
// 同一选手可能有多条（多舞种），由 service 层按 player_no 归并到选手目录。
type MaterialArchiveRow struct {
	PlayerName  string
	PlayerNo    string
	Level       string
	DanceType   string
	PhotoPath   *string
	AudioPath   *string
	ProgramName *string
}

// ListMaterialsForArchive 返回赛事媒体归档所需的报名资料（不分页）。
// 默认仅 active 报名；includeCanceled 时含已取消留档。playerNos 非空时
// 只返回指定选手编号的记录。按组别/舞种/姓名排序，输出稳定。
func (s *RegistrationStore) ListMaterialsForArchive(ctx context.Context, eventID int64,
	playerNos []string, includeCanceled bool) ([]MaterialArchiveRow, error) {
	where := []string{"r.event_id = ?"}
	args := []any{eventID}
	if includeCanceled {
		where = append(where, "r.status IN ('active', 'canceled')")
	} else {
		where = append(where, "r.status = 'active'")
	}
	if len(playerNos) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(playerNos)), ",")
		where = append(where, "pl.player_no IN ("+placeholders+")")
		for _, no := range playerNos {
			args = append(args, no)
		}
	}
	rows, err := s.db.sql.QueryContext(ctx, `
		SELECT pl.name, pl.player_no, g.level, r.dance_type,
		       r.photo_path, r.audio_path, r.program_name
		FROM registrations r
		JOIN event_groups g ON g.id = r.group_id
		JOIN players pl ON pl.id = r.player_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY g.level, r.dance_type, pl.name, r.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("query event %d materials for archive: %w", eventID, err)
	}
	defer rows.Close()

	list := make([]MaterialArchiveRow, 0, 50)
	for rows.Next() {
		var r MaterialArchiveRow
		if err := rows.Scan(&r.PlayerName, &r.PlayerNo, &r.Level, &r.DanceType,
			&r.PhotoPath, &r.AudioPath, &r.ProgramName); err != nil {
			return nil, fmt.Errorf("scan event %d material row: %w", eventID, err)
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate event %d material rows: %w", eventID, err)
	}
	return list, nil
}
