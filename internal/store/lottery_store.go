package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/meetmonth/dance-elite/internal/model"
)

// LotteryStore 是赛事抽签的数据访问层（lottery_sessions / groups / players）。
//
// 抽签写入集中在单个事务内（创建/覆盖、抽签分配、短信状态），
// SQLite 单连接池保证「读取剩余槽位 → 写入槽位」不会被并发交错。
type LotteryStore struct{ db *DB }

// NewLotteryStore 构造抽签 store。
func NewLotteryStore(db *DB) *LotteryStore { return &LotteryStore{db: db} }

// LotterySessionParams 是新建/修改抽签的字段。
type LotterySessionParams struct {
	EventID             *int64
	EventName           string
	Token               string
	Source              string
	StartAt             *string
	EndAt               *string
	RemindBeforeMinutes int
	SMSRemindTemplate   *string
	SMSResultTemplate   *string
	CreatedBy           *int64
}

// LotteryPlayerParams 是写入抽签选手的字段（证件号只给哈希与密文）。
type LotteryPlayerParams struct {
	RegistrationID *int64
	Name           string
	IDCardHash     string
	IDCardMasked   *string
	IDCardEnc      *string
	Genre          string
	AgeGroup       string
	Institution    *string
	Program        *string
	Phone          *string
}

// LotterySeedPlayer 是从赛事报名生成抽签时的候选选手。
type LotterySeedPlayer struct {
	RegistrationID int64
	Name           string
	IDCardHash     string
	Genre          string
	AgeGroup       string
	Institution    *string
	Program        *string
	Phone          *string
}

const lotterySessionColumns = `id, event_id, event_name, token, source, start_at, end_at,
	remind_before_minutes, sms_remind_template, sms_result_template, created_by, created_at, updated_at`

// Create 新建抽签会话并写入分组与选手，返回 session id。
func (s *LotteryStore) Create(ctx context.Context, tx *sql.Tx, params LotterySessionParams,
	groups []model.LotteryGroup, players []LotteryPlayerParams) (int64, error) {
	res, err := tx.ExecContext(ctx, `
		INSERT INTO lottery_sessions (event_id, event_name, token, source, start_at, end_at,
			remind_before_minutes, sms_remind_template, sms_result_template, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		params.EventID, params.EventName, params.Token, params.Source, params.StartAt, params.EndAt,
		params.RemindBeforeMinutes, params.SMSRemindTemplate, params.SMSResultTemplate, params.CreatedBy)
	if err != nil {
		return 0, fmt.Errorf("insert lottery session: %w", err)
	}
	sessionID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("lottery session id: %w", err)
	}
	if err := s.insertGroups(ctx, tx, sessionID, groups); err != nil {
		return 0, err
	}
	if err := s.insertPlayers(ctx, tx, sessionID, players); err != nil {
		return 0, err
	}
	return sessionID, nil
}

// Replace 覆盖一场抽签的分组与选手，并更新会话设置（沿用原 token）。
func (s *LotteryStore) Replace(ctx context.Context, tx *sql.Tx, sessionID int64,
	params LotterySessionParams, groups []model.LotteryGroup, players []LotteryPlayerParams) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM lottery_players WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("delete lottery players: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM lottery_groups WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("delete lottery groups: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE lottery_sessions
		SET event_id = ?, event_name = ?, source = ?, start_at = ?, end_at = ?,
			remind_before_minutes = ?, sms_remind_template = ?, sms_result_template = ?
		WHERE id = ?`,
		params.EventID, params.EventName, params.Source, params.StartAt, params.EndAt,
		params.RemindBeforeMinutes, params.SMSRemindTemplate, params.SMSResultTemplate, sessionID); err != nil {
		return fmt.Errorf("update lottery session: %w", err)
	}
	if err := s.insertGroups(ctx, tx, sessionID, groups); err != nil {
		return err
	}
	return s.insertPlayers(ctx, tx, sessionID, players)
}

// UpdateSettings 修改抽签设置（时间窗口/短信模板/关联赛事）。
func (s *LotteryStore) UpdateSettings(ctx context.Context, tx *sql.Tx, sessionID int64, params LotterySessionParams) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE lottery_sessions
		SET event_id = ?, event_name = ?, start_at = ?, end_at = ?,
			remind_before_minutes = ?, sms_remind_template = ?, sms_result_template = ?
		WHERE id = ?`,
		params.EventID, params.EventName, params.StartAt, params.EndAt,
		params.RemindBeforeMinutes, params.SMSRemindTemplate, params.SMSResultTemplate, sessionID)
	if err != nil {
		return fmt.Errorf("update lottery settings: %w", err)
	}
	return nil
}

func (s *LotteryStore) insertGroups(ctx context.Context, tx *sql.Tx, sessionID int64, groups []model.LotteryGroup) error {
	for _, group := range groups {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO lottery_groups (session_id, genre, age_group, letter, size)
			VALUES (?, ?, ?, ?, ?)`,
			sessionID, group.Genre, group.AgeGroup, group.Letter, group.Size); err != nil {
			return fmt.Errorf("insert lottery group %s/%s/%s: %w", group.Genre, group.AgeGroup, group.Letter, err)
		}
	}
	return nil
}

func (s *LotteryStore) insertPlayers(ctx context.Context, tx *sql.Tx, sessionID int64, players []LotteryPlayerParams) error {
	for _, player := range players {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO lottery_players (session_id, registration_id, name, id_card_hash,
				id_card_masked, id_card_enc, genre, age_group, institution, program, phone)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sessionID, player.RegistrationID, player.Name, player.IDCardHash,
			player.IDCardMasked, player.IDCardEnc, player.Genre, player.AgeGroup,
			player.Institution, player.Program, player.Phone); err != nil {
			return fmt.Errorf("insert lottery player %q: %w", player.Name, err)
		}
	}
	return nil
}

// List 返回全部抽签（含人数统计），按更新时间倒序。
func (s *LotteryStore) List(ctx context.Context) ([]model.LotterySession, error) {
	rows, err := s.db.sql.QueryContext(ctx, `
		SELECT `+lotterySessionColumns+`,
			(SELECT COUNT(*) FROM lottery_players p WHERE p.session_id = s.id),
			(SELECT COUNT(*) FROM lottery_players p WHERE p.session_id = s.id AND p.slot_no IS NOT NULL)
		FROM lottery_sessions s
		ORDER BY s.updated_at DESC, s.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query lottery sessions: %w", err)
	}
	defer rows.Close()
	var sessions []model.LotterySession
	for rows.Next() {
		session, err := scanLotterySession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lottery sessions: %w", err)
	}
	return sessions, nil
}

// GetByEventID 查找关联某赛事的抽签；不存在返回 ErrNotFound。
func (s *LotteryStore) GetByEventID(ctx context.Context, eventID int64) (*model.LotterySession, error) {
	row := s.db.sql.QueryRowContext(ctx, `
		SELECT `+lotterySessionColumns+`,
			(SELECT COUNT(*) FROM lottery_players p WHERE p.session_id = s.id),
			(SELECT COUNT(*) FROM lottery_players p WHERE p.session_id = s.id AND p.slot_no IS NOT NULL)
		FROM lottery_sessions s WHERE s.event_id = ? ORDER BY s.id DESC LIMIT 1`, eventID)
	session, err := scanLotterySession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return session, err
}

// ExistsToken 判断 token 是否已被占用。
func (s *LotteryStore) ExistsToken(ctx context.Context, token string) (bool, error) {
	var count int
	if err := s.db.sql.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM lottery_sessions WHERE token = ?`, token).Scan(&count); err != nil {
		return false, fmt.Errorf("count lottery token: %w", err)
	}
	return count > 0, nil
}

// GetByToken 读取抽签会话、分组与选手（只读）。
func (s *LotteryStore) GetByToken(ctx context.Context, token string) (*model.LotterySession, []model.LotteryGroup, []model.LotteryPlayer, error) {
	session, err := s.getSessionByToken(ctx, s.db.sql, token)
	if err != nil {
		return nil, nil, nil, err
	}
	groups, players, err := s.loadContent(ctx, s.db.sql, session.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	session.PlayerCount = len(players)
	for _, player := range players {
		if player.SlotNo != nil {
			session.DrawnCount++
		}
	}
	return session, groups, players, nil
}

// GetByTokenTx 在事务内读取抽签会话、分组与选手。
func (s *LotteryStore) GetByTokenTx(ctx context.Context, tx *sql.Tx, token string) (*model.LotterySession, []model.LotteryGroup, []model.LotteryPlayer, error) {
	session, err := s.getSessionByToken(ctx, tx, token)
	if err != nil {
		return nil, nil, nil, err
	}
	groups, players, err := s.loadContent(ctx, tx, session.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	session.PlayerCount = len(players)
	for _, player := range players {
		if player.SlotNo != nil {
			session.DrawnCount++
		}
	}
	return session, groups, players, nil
}

// GetPlayers 读取某场抽签的全部选手（定时任务用）。
func (s *LotteryStore) GetPlayers(ctx context.Context, sessionID int64) ([]model.LotteryPlayer, error) {
	players, err := s.loadPlayers(ctx, s.db.sql, sessionID)
	if err != nil {
		return nil, err
	}
	return players, nil
}

// UpdatePlayerSlot 原子分配槽位（无操作人，系统自动分配用）；仅当该选手尚未抽签时生效。
// 返回 false 表示槽位已被并发请求占用。
func (s *LotteryStore) UpdatePlayerSlot(ctx context.Context, tx *sql.Tx, playerID int64,
	letter string, no int, drawnAt string, auto bool) (bool, error) {
	return s.updatePlayerSlot(ctx, tx, playerID, letter, no, drawnAt, auto, "", "")
}

// UpdatePlayerSlotWithIdentity 分配槽位并记录公众号抽签身份（手动抽签用）；
// openid/unionid 为空时写 NULL。
func (s *LotteryStore) UpdatePlayerSlotWithIdentity(ctx context.Context, tx *sql.Tx, playerID int64,
	letter string, no int, drawnAt string, openid, unionid string) (bool, error) {
	return s.updatePlayerSlot(ctx, tx, playerID, letter, no, drawnAt, false, openid, unionid)
}

func (s *LotteryStore) updatePlayerSlot(ctx context.Context, tx *sql.Tx, playerID int64,
	letter string, no int, drawnAt string, auto bool, openid, unionid string) (bool, error) {
	res, err := tx.ExecContext(ctx, `
		UPDATE lottery_players
		SET group_letter = ?, slot_no = ?, drawn_at = ?, auto_drawn = ?,
			drawn_openid = NULLIF(?, ''), drawn_unionid = NULLIF(?, '')
		WHERE id = ? AND slot_no IS NULL`,
		letter, no, drawnAt, boolInt(auto), openid, unionid, playerID)
	if err != nil {
		return false, fmt.Errorf("update lottery slot: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("lottery slot affected: %w", err)
	}
	return affected == 1, nil
}

// UpdatePlayerSMS 记录一次短信发送结果：尝试数 +1，成功时写发送时间。
func (s *LotteryStore) UpdatePlayerSMS(ctx context.Context, tx *sql.Tx, playerID int64,
	kind string, sentAt string, success bool) error {
	var query string
	switch kind {
	case LotterySMSRemind:
		query = `UPDATE lottery_players
			SET remind_attempts = remind_attempts + 1,
				remind_sent_at = CASE WHEN ? = 1 THEN ? ELSE remind_sent_at END
			WHERE id = ?`
	case LotterySMSResult:
		query = `UPDATE lottery_players
			SET result_attempts = result_attempts + 1,
				result_sent_at = CASE WHEN ? = 1 THEN ? ELSE result_sent_at END
			WHERE id = ?`
	default:
		return fmt.Errorf("unknown lottery sms kind %q", kind)
	}
	if _, err := tx.ExecContext(ctx, query, boolInt(success), sentAt, playerID); err != nil {
		return fmt.Errorf("update lottery sms: %w", err)
	}
	return nil
}

// 短信类型（与 lottery_notify 的 kind 常量一致）。
const (
	LotterySMSRemind = "remind"
	LotterySMSResult = "result"
)

func (s *LotteryStore) getSessionByToken(ctx context.Context, q lotteryQueryer, token string) (*model.LotterySession, error) {
	row := q.QueryRowContext(ctx, `
		SELECT `+lotterySessionColumns+`, 0, 0
		FROM lottery_sessions WHERE token = ?`, token)
	session, err := scanLotterySession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return session, err
}

// loadContent 读取分组与选手。
func (s *LotteryStore) loadContent(ctx context.Context, q lotteryQueryer, sessionID int64) ([]model.LotteryGroup, []model.LotteryPlayer, error) {
	groups, err := s.loadGroups(ctx, q, sessionID)
	if err != nil {
		return nil, nil, err
	}
	players, err := s.loadPlayers(ctx, q, sessionID)
	if err != nil {
		return nil, nil, err
	}
	return groups, players, nil
}

func (s *LotteryStore) loadGroups(ctx context.Context, q lotteryQueryer, sessionID int64) ([]model.LotteryGroup, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, session_id, genre, age_group, letter, size
		FROM lottery_groups WHERE session_id = ? ORDER BY id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query lottery groups: %w", err)
	}
	defer rows.Close()
	var groups []model.LotteryGroup
	for rows.Next() {
		var group model.LotteryGroup
		if err := rows.Scan(&group.ID, &group.SessionID, &group.Genre, &group.AgeGroup,
			&group.Letter, &group.Size); err != nil {
			return nil, fmt.Errorf("scan lottery group: %w", err)
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (s *LotteryStore) loadPlayers(ctx context.Context, q lotteryQueryer, sessionID int64) ([]model.LotteryPlayer, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, session_id, registration_id, name, id_card_hash, id_card_masked, id_card_enc,
			genre, age_group, institution, program, phone, group_letter, slot_no, drawn_at,
			auto_drawn, drawn_openid, drawn_unionid,
			remind_sent_at, remind_attempts, result_sent_at, result_attempts,
			created_at, updated_at
		FROM lottery_players WHERE session_id = ? ORDER BY id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query lottery players: %w", err)
	}
	defer rows.Close()
	var players []model.LotteryPlayer
	for rows.Next() {
		var (
			player       model.LotteryPlayer
			autoDrawn    int
			registration sql.NullInt64
			masked       sql.NullString
			enc          sql.NullString
			institution  sql.NullString
			program      sql.NullString
			phone        sql.NullString
			letter       sql.NullString
			slotNo       sql.NullInt64
			drawnAt      sql.NullString
			drawnOpenID  sql.NullString
			drawnUnionID sql.NullString
			remindSent   sql.NullString
			resultSent   sql.NullString
		)
		if err := rows.Scan(&player.ID, &player.SessionID, &registration, &player.Name,
			&player.IDCardHash, &masked, &enc, &player.Genre, &player.AgeGroup,
			&institution, &program, &phone, &letter, &slotNo, &drawnAt, &autoDrawn,
			&drawnOpenID, &drawnUnionID,
			&remindSent, &player.SMS.RemindAttempts, &resultSent, &player.SMS.ResultAttempts,
			&player.CreatedAt, &player.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan lottery player: %w", err)
		}
		player.RegistrationID = nullInt64Ptr(registration)
		player.IDCardMasked = nullStringPtr(masked)
		player.IDCardEnc = nullStringPtr(enc)
		player.Institution = nullStringPtr(institution)
		player.Program = nullStringPtr(program)
		player.Phone = nullStringPtr(phone)
		player.GroupLetter = nullStringPtr(letter)
		player.SlotNo = nullIntPtr(slotNo)
		player.DrawnAt = nullStringPtr(drawnAt)
		player.AutoDrawn = autoDrawn == 1
		player.DrawnOpenID = nullStringPtr(drawnOpenID)
		player.DrawnUnionID = nullStringPtr(drawnUnionID)
		player.SMS.RemindSentAt = nullStringPtr(remindSent)
		player.SMS.ResultSentAt = nullStringPtr(resultSent)
		players = append(players, player)
	}
	return players, rows.Err()
}

// ListEventRegistrations 返回某赛事全部有效报名（抽签从赛事生成时使用）。
// skipped 是没有证件号哈希、无法参与抽签的报名数（人工登记/护照等情况）。
func (s *LotteryStore) ListEventRegistrations(ctx context.Context, eventID int64) ([]LotterySeedPlayer, int, error) {
	rows, err := s.db.sql.QueryContext(ctx, `
		SELECT r.id, p.name, p.id_card_hash, r.dance_type, g.level,
			r.institution, r.program_name, r.contact_phone
		FROM registrations r
		JOIN players p ON p.id = r.player_id
		JOIN event_groups g ON g.id = r.group_id
		WHERE r.event_id = ? AND r.status = 'active'
		ORDER BY r.id`, eventID)
	if err != nil {
		return nil, 0, fmt.Errorf("query event registrations for lottery: %w", err)
	}
	defer rows.Close()
	var players []LotterySeedPlayer
	skipped := 0
	for rows.Next() {
		var (
			player      LotterySeedPlayer
			idCardHash  sql.NullString
			institution sql.NullString
			program     sql.NullString
			phone       sql.NullString
		)
		if err := rows.Scan(&player.RegistrationID, &player.Name, &idCardHash,
			&player.Genre, &player.AgeGroup, &institution, &program, &phone); err != nil {
			return nil, 0, fmt.Errorf("scan lottery seed player: %w", err)
		}
		if !idCardHash.Valid || idCardHash.String == "" {
			skipped++
			continue
		}
		player.IDCardHash = idCardHash.String
		player.Institution = nullStringPtr(institution)
		player.Program = nullStringPtr(program)
		player.Phone = nullStringPtr(phone)
		players = append(players, player)
	}
	return players, skipped, rows.Err()
}

// lotteryQueryer 是 *sql.DB / *sql.Tx 的公共查询接口。
type lotteryQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func scanLotterySession(row rowScanner) (*model.LotterySession, error) {
	var (
		session   model.LotterySession
		eventID   sql.NullInt64
		startAt   sql.NullString
		endAt     sql.NullString
		smsRemind sql.NullString
		smsResult sql.NullString
		createdBy sql.NullInt64
	)
	if err := row.Scan(&session.ID, &eventID, &session.EventName, &session.Token, &session.Source,
		&startAt, &endAt, &session.RemindBeforeMinutes, &smsRemind, &smsResult,
		&createdBy, &session.CreatedAt, &session.UpdatedAt,
		&session.PlayerCount, &session.DrawnCount); err != nil {
		return nil, err
	}
	session.EventID = nullInt64Ptr(eventID)
	session.StartAt = nullStringPtr(startAt)
	session.EndAt = nullStringPtr(endAt)
	session.SMSRemindTemplate = nullStringPtr(smsRemind)
	session.SMSResultTemplate = nullStringPtr(smsResult)
	session.CreatedBy = nullInt64Ptr(createdBy)
	return &session, nil
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	value := v.String
	return &value
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	value := v.Int64
	return &value
}

func nullIntPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	value := int(v.Int64)
	return &value
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// lotteryTokenPattern 限定 token 只能是 8-64 位字母数字。
var lotteryTokenPattern = regexp.MustCompile(`^[A-Za-z0-9]{8,64}$`)

// ValidLotteryToken 判断 token 是否形如抽签链接凭据。
func ValidLotteryToken(token string) bool {
	return lotteryTokenPattern.MatchString(token)
}

// NormalizeLotteryToken 清理 token 首尾空白，防止数据库里出现带空格的凭据。
func NormalizeLotteryToken(token string) string {
	return strings.TrimSpace(token)
}
