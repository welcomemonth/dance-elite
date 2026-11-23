package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/meetmonth/dance-elite/internal/lotteryimport"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/pkg/secureid"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// 抽签后台审计动作（admin_logs.action）。
const (
	adminActionLotteryImport   = "admin.lottery.import"
	adminActionLotteryExport   = "admin.lottery.export"
	adminActionLotterySettings = "admin.lottery.settings"
)

// AdminLotteryService 承载后台「赛事抽签」页：导入 Excel / 从赛事报名生成、
// 维护时间窗口与短信模板、导出抽签结果。
type AdminLotteryService struct {
	stores  *store.Store
	baseURL string
	idKey   []byte
	encKey  []byte
	now     func() time.Time
}

// NewAdminLotteryService 构造后台抽签服务；idCardKey 为 ID_CARD_HMAC_KEY。
func NewAdminLotteryService(stores *store.Store, baseURL, idCardKey string) *AdminLotteryService {
	return &AdminLotteryService{
		stores:  stores,
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		idKey:   []byte(idCardKey),
		encKey:  secureid.DeriveKey(idCardKey),
		now:     time.Now,
	}
}

// AdminLotteryImportInput 是后台生成抽签的参数。
type AdminLotteryImportInput struct {
	// EventID 为 0 表示 Excel 抽签不关联赛事（此时 Title 必填）。
	EventID int64
	// Title 是自定义抽签名称；为空时使用赛事名称（不关联赛事时必填）。
	Title string
	// Source 为 excel（上传报名表）或 event（读取赛事已截止的有效报名）。
	Source string
	// Excel 仅在 Source=excel 时必填。
	Excel     io.Reader
	StartAt   string
	EndAt     string
	Remind    int
	SMSRemind string
	SMSResult string
	// Replace 为 true 时允许覆盖该赛事已有抽签（清空结果，沿用原 token）。
	Replace bool
}

// List 返回全部赛事抽签（按更新时间倒序）。
func (s *AdminLotteryService) List(ctx context.Context) (*model.AdminLotteryListResponse, error) {
	sessions, err := s.stores.Lottery.List(ctx)
	if err != nil {
		return nil, httpx.ErrInternal(err, "读取抽签列表失败，请稍后重试")
	}
	list := make([]model.AdminLotterySession, 0, len(sessions))
	for i := range sessions {
		list = append(list, s.summary(&sessions[i]))
	}
	return &model.AdminLotteryListResponse{List: list}, nil
}

// Get 返回单场抽签详情（含分组进度）。
func (s *AdminLotteryService) Get(ctx context.Context, token string) (*model.AdminLotteryDetail, error) {
	session, groups, players, err := s.load(ctx, token)
	if err != nil {
		return nil, err
	}
	detail := &model.AdminLotteryDetail{
		AdminLotterySession: s.summary(session),
		Groups:              lotteryGroupStats(groups, players),
	}
	return detail, nil
}

// Import 生成（或覆盖）赛事抽签。
//
// Excel 模式可以不关联赛事（EventID=0，用 Title 作为抽签名称），此时每次
// 导入都生成新链接；关联赛事时同一赛事只允许一场，重复导入需 Replace。
// 赛事报名模式必须关联赛事。
func (s *AdminLotteryService) Import(ctx context.Context, adminID int64, in AdminLotteryImportInput) (*model.AdminLotteryCreateResult, error) {
	startAt, endAt, err := lotteryWindowInput(in.StartAt, in.EndAt)
	if err != nil {
		return nil, httpx.ErrBadRequest(err.Error())
	}

	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = model.LotterySourceExcel
	}
	if source != model.LotterySourceExcel && source != model.LotterySourceEvent {
		return nil, httpx.ErrBadRequest("抽签来源不合法")
	}

	var event *model.Event
	if in.EventID > 0 {
		event, err = s.stores.Events.Get(ctx, in.EventID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, httpx.ErrNotFound("赛事不存在")
			}
			return nil, httpx.ErrInternal(err, "加载赛事失败，请稍后重试")
		}
	}
	if source == model.LotterySourceEvent && event == nil {
		return nil, httpx.ErrBadRequest("从赛事报名生成必须选择赛事")
	}

	title := strings.TrimSpace(in.Title)
	if len([]rune(title)) > 100 {
		return nil, httpx.ErrBadRequest("抽签名称不能超过 100 个字")
	}
	eventName := ""
	var eventID *int64
	if event != nil {
		eventID = &event.ID
		eventName = event.Name
	}
	if title != "" {
		eventName = title
	}
	if eventName == "" {
		return nil, httpx.ErrBadRequest("未选择赛事时请填写抽签名称")
	}

	groups, players, warnings, err := s.buildRoster(ctx, event, eventName, source, in.Excel)
	if err != nil {
		return nil, err
	}

	params := store.LotterySessionParams{
		EventID:             eventID,
		EventName:           eventName,
		Source:              source,
		StartAt:             startAt,
		EndAt:               endAt,
		RemindBeforeMinutes: in.Remind,
		SMSRemindTemplate:   trimmedPtr(in.SMSRemind),
		SMSResultTemplate:   trimmedPtr(in.SMSResult),
		CreatedBy:           &adminID,
	}

	var existing *model.LotterySession
	if eventID != nil {
		existing, err = s.stores.Lottery.GetByEventID(ctx, *eventID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return nil, httpx.ErrInternal(err, "读取赛事抽签失败，请稍后重试")
		}
		if existing != nil && !in.Replace {
			return nil, httpx.ErrConflict("该赛事已有抽签，如需重新生成请勾选「覆盖已有抽签」（会清空已有结果）")
		}
	}

	var sessionID int64
	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		if existing != nil {
			sessionID = existing.ID
			params.Token = existing.Token
			return s.stores.Lottery.Replace(ctx, tx, existing.ID, params, groups, players)
		}
		token, err := s.newToken(ctx, tx)
		if err != nil {
			return err
		}
		params.Token = token
		id, createErr := s.stores.Lottery.Create(ctx, tx, params, groups, players)
		sessionID = id
		return createErr
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "写入抽签数据失败，请稍后重试")
	}
	detailEventID := int64(0)
	if eventID != nil {
		detailEventID = *eventID
	}
	if err := s.audit(ctx, adminID, adminActionLotteryImport, eventID, map[string]any{
		"players": len(players), "source": source, "replace": existing != nil,
	}); err != nil {
		return nil, err
	}
	logger.Event(ctx, "lottery.imported", "生成赛事抽签",
		"session_id", sessionID, "event_id", detailEventID, "players", len(players),
		"source", source, "replace", existing != nil)

	session, _, _, err := s.loadByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return &model.AdminLotteryCreateResult{
		AdminLotterySession: s.summary(session),
		Warnings:            warnings,
	}, nil
}

// buildRoster 按来源构造分组与选手参数。
func (s *AdminLotteryService) buildRoster(ctx context.Context, event *model.Event, eventName, source string,
	excel io.Reader) ([]model.LotteryGroup, []store.LotteryPlayerParams, []string, error) {
	switch source {
	case model.LotterySourceExcel:
		return s.buildRosterFromExcel(excel, eventName)
	case model.LotterySourceEvent:
		return s.buildRosterFromEvent(ctx, event)
	default:
		return nil, nil, nil, httpx.ErrBadRequest("抽签来源不合法")
	}
}

func (s *AdminLotteryService) buildRosterFromExcel(excel io.Reader, eventName string) (
	[]model.LotteryGroup, []store.LotteryPlayerParams, []string, error) {
	if excel == nil {
		return nil, nil, nil, httpx.ErrBadRequest("请上传报名 Excel 文件（.xlsx）")
	}
	result, warnings, err := lotteryimport.BuildReader(excel, eventName)
	if err != nil {
		return nil, nil, nil, httpx.ErrBadRequest(err.Error())
	}
	players := make([]store.LotteryPlayerParams, 0, len(result.Players))
	for _, player := range result.Players {
		idCard := utils.NormalizeIDNumber(utils.IDTypeIDCard, player.IDCard)
		encrypted, err := secureid.Encrypt(s.encKey, idCard)
		if err != nil {
			return nil, nil, nil, httpx.ErrInternal(err, "加密证件号失败，请稍后重试")
		}
		players = append(players, store.LotteryPlayerParams{
			Name:         player.Name,
			IDCardHash:   utils.HashIDNumber(s.idKey, utils.IDTypeIDCard, idCard),
			IDCardMasked: strPtr(utils.MaskIDNumber(idCard)),
			IDCardEnc:    &encrypted,
			Genre:        player.Genre,
			AgeGroup:     player.AgeGroup,
			Institution:  trimmedPtr(player.Institution),
			Program:      trimmedPtr(player.Program),
			Phone:        trimmedPtr(player.Phone),
		})
	}
	return result.Groups, players, warnings, nil
}

func (s *AdminLotteryService) buildRosterFromEvent(ctx context.Context, event *model.Event) (
	[]model.LotteryGroup, []store.LotteryPlayerParams, []string, error) {
	if !lotteryRegistrationClosed(event, s.now()) {
		return nil, nil, nil, httpx.ErrConflict("该赛事报名尚未结束，暂不能从报名结果生成抽签")
	}
	seeds, skipped, err := s.stores.Lottery.ListEventRegistrations(ctx, event.ID)
	if err != nil {
		return nil, nil, nil, httpx.ErrInternal(err, "读取赛事报名失败，请稍后重试")
	}
	if len(seeds) == 0 {
		return nil, nil, nil, httpx.ErrConflict("该赛事暂无带证件号的有效报名记录，无法生成抽签")
	}
	warnings := []string{}
	if skipped > 0 {
		warnings = append(warnings, fmt.Sprintf("有 %d 条报名缺少证件号哈希，未纳入抽签", skipped))
	}
	grouping := make([]lotteryimport.Player, 0, len(seeds))
	players := make([]store.LotteryPlayerParams, 0, len(seeds))
	seen := make(map[string]bool, len(seeds))
	for _, seed := range seeds {
		key := seed.IDCardHash + "|" + seed.Genre
		if seen[key] {
			return nil, nil, nil, httpx.ErrBadRequest("赛事报名数据存在重复记录（同一选手同一舞种），请先修正报名数据")
		}
		seen[key] = true
		grouping = append(grouping, lotteryimport.Player{
			Name: seed.Name, Genre: seed.Genre, AgeGroup: seed.AgeGroup,
		})
		players = append(players, store.LotteryPlayerParams{
			RegistrationID: &seed.RegistrationID,
			Name:           seed.Name,
			IDCardHash:     seed.IDCardHash,
			Genre:          seed.Genre,
			AgeGroup:       seed.AgeGroup,
			Institution:    seed.Institution,
			Program:        seed.Program,
			Phone:          seed.Phone,
		})
	}
	groups, groupWarnings := lotteryimport.Group(grouping)
	return groups, players, append(warnings, groupWarnings...), nil
}

// UpdateSettings 修改已有抽签的时间窗口/短信模板（可重新关联赛事）。
func (s *AdminLotteryService) UpdateSettings(ctx context.Context, adminID int64, token string, req model.AdminLotterySettingsRequest) (*model.AdminLotterySession, error) {
	startAt, endAt, err := lotteryWindowInput(req.StartAt, req.EndAt)
	if err != nil {
		return nil, httpx.ErrBadRequest(err.Error())
	}
	session, _, _, err := s.load(ctx, token)
	if err != nil {
		return nil, err
	}
	params := store.LotterySessionParams{
		EventID:             session.EventID,
		EventName:           session.EventName,
		StartAt:             startAt,
		EndAt:               endAt,
		RemindBeforeMinutes: req.RemindBeforeMinutes,
		SMSRemindTemplate:   trimmedPtr(req.SMSRemindTemplate),
		SMSResultTemplate:   trimmedPtr(req.SMSResultTemplate),
	}
	if req.EventID != nil && *req.EventID > 0 {
		event, err := s.stores.Events.Get(ctx, *req.EventID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, httpx.ErrNotFound("赛事不存在")
			}
			return nil, httpx.ErrInternal(err, "加载赛事失败，请稍后重试")
		}
		params.EventID = &event.ID
		params.EventName = event.Name
	}
	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		return s.stores.Lottery.UpdateSettings(ctx, tx, session.ID, params)
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "保存抽签设置失败，请稍后重试")
	}
	if err := s.audit(ctx, adminID, adminActionLotterySettings, params.EventID, map[string]any{
		"start_at": derefString(params.StartAt), "end_at": derefString(params.EndAt),
	}); err != nil {
		return nil, err
	}
	updated, _, _, err := s.load(ctx, token)
	if err != nil {
		return nil, err
	}
	summary := s.summary(updated)
	return &summary, nil
}

// summary 由抽签会话生成后台列表项。
func (s *AdminLotteryService) summary(session *model.LotterySession) model.AdminLotterySession {
	return model.AdminLotterySession{
		ID:                  session.ID,
		Token:               session.Token,
		EventID:             session.EventID,
		EventName:           session.EventName,
		Source:              session.Source,
		StartAt:             derefString(session.StartAt),
		EndAt:               derefString(session.EndAt),
		RemindBeforeMinutes: session.RemindBeforeMinutes,
		SMSRemindTemplate:   derefString(session.SMSRemindTemplate),
		SMSResultTemplate:   derefString(session.SMSResultTemplate),
		PlayerCount:         session.PlayerCount,
		DrawnCount:          session.DrawnCount,
		PendingCount:        session.PlayerCount - session.DrawnCount,
		CreatedAt:           session.CreatedAt,
		UpdatedAt:           session.UpdatedAt,
		Link:                s.link(session.Token),
	}
}

// link 拼接免登录抽签页地址；未配置 public_base_url 时返回相对路径。
func (s *AdminLotteryService) link(token string) string {
	return s.baseURL + "/lottery/" + token
}

func (s *AdminLotteryService) load(ctx context.Context, token string) (*model.LotterySession, []model.LotteryGroup, []model.LotteryPlayer, error) {
	session, groups, players, err := s.stores.Lottery.GetByToken(ctx, store.NormalizeLotteryToken(token))
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil, nil, httpx.ErrNotFound("抽签不存在或链接无效")
	}
	if err != nil {
		return nil, nil, nil, httpx.ErrInternal(err, "读取抽签数据失败，请稍后重试")
	}
	return session, groups, players, nil
}

func (s *AdminLotteryService) loadByID(ctx context.Context, id int64) (*model.LotterySession, []model.LotteryGroup, []model.LotteryPlayer, error) {
	sessions, err := s.stores.Lottery.List(ctx)
	if err != nil {
		return nil, nil, nil, httpx.ErrInternal(err, "读取抽签数据失败，请稍后重试")
	}
	for i := range sessions {
		if sessions[i].ID == id {
			return s.load(ctx, sessions[i].Token)
		}
	}
	return nil, nil, nil, httpx.ErrNotFound("抽签不存在或链接无效")
}

// newToken 生成未被占用的 16 位 token。
func (s *AdminLotteryService) newToken(ctx context.Context, tx *sql.Tx) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		token, err := randomLotteryToken()
		if err != nil {
			return "", err
		}
		var count int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM lottery_sessions WHERE token = ?`, token).Scan(&count); err != nil {
			return "", fmt.Errorf("check lottery token: %w", err)
		}
		if count == 0 {
			return token, nil
		}
	}
	return "", errors.New("generate lottery token: exhausted retries")
}

func (s *AdminLotteryService) audit(ctx context.Context, adminID int64, action string, eventID *int64, detail map[string]any) error {
	err := s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		return adminLog(ctx, s.stores, tx, &adminID, action, strPtr("event"), eventID, detail)
	})
	if err != nil {
		return httpx.ErrInternal(err, "记录操作日志失败，请稍后重试")
	}
	return nil
}

// lotteryWindowInput 解析后台表单中的开始/截止时间，空值表示不限制。
func lotteryWindowInput(startRaw, endRaw string) (startAt, endAt *string, err error) {
	var start, end time.Time
	var hasStart, hasEnd bool
	if strings.TrimSpace(startRaw) != "" {
		start, err = utils.ParseFlexibleTime(startRaw)
		if err != nil {
			return nil, nil, fmt.Errorf("开始时间格式不正确：%w", err)
		}
		hasStart = true
	}
	if strings.TrimSpace(endRaw) != "" {
		end, err = utils.ParseFlexibleTime(endRaw)
		if err != nil {
			return nil, nil, fmt.Errorf("截止时间格式不正确：%w", err)
		}
		hasEnd = true
	}
	if hasStart && hasEnd && !start.Before(end) {
		return nil, nil, errors.New("开始时间必须早于截止时间")
	}
	if hasStart {
		value := utils.FormatISO(start)
		startAt = &value
	}
	if hasEnd {
		value := utils.FormatISO(end)
		endAt = &value
	}
	return startAt, endAt, nil
}

// lotteryRegistrationClosed 判断赛事报名是否已结束（可从中生成抽签）。
func lotteryRegistrationClosed(event *model.Event, now time.Time) bool {
	switch event.Status {
	case model.EventStatusRegClosed, model.EventStatusOngoing, model.EventStatusFinished:
		return true
	}
	if event.RegEndAt != nil {
		if end, err := utils.ParseISO(*event.RegEndAt); err == nil && !now.Before(end) {
			return true
		}
	}
	return false
}

// lotteryGroupStats 按「组别 × 舞种 × 字母组」统计已抽人数。
func lotteryGroupStats(groups []model.LotteryGroup, players []model.LotteryPlayer) []model.AdminLotteryGroupStat {
	type key struct{ genre, ageGroup, letter string }
	drawn := make(map[key]int, len(groups))
	for _, player := range players {
		if player.SlotNo == nil {
			continue
		}
		drawn[key{player.Genre, player.AgeGroup, *player.GroupLetter}]++
	}
	stats := make([]model.AdminLotteryGroupStat, 0, len(groups))
	for _, group := range groups {
		stats = append(stats, model.AdminLotteryGroupStat{
			Genre:      group.Genre,
			GenreLabel: lotteryGenreLabel(group.Genre),
			AgeGroup:   group.AgeGroup,
			Letter:     group.Letter,
			Size:       group.Size,
			Drawn:      drawn[key{group.Genre, group.AgeGroup, group.Letter}],
		})
	}
	return stats
}

// lotterySortPlayers 按舞种顺序、年龄组、字母组、组内序号排列，
// 未抽签的记录排在已抽签之后。
func lotterySortPlayers(players []model.LotteryPlayer) []model.LotteryPlayer {
	genreOrder := make(map[string]int, len(lotteryGenreOrder))
	for i, genre := range lotteryGenreOrder {
		genreOrder[genre] = i
	}
	ageOrder := map[string]int{"U11": 0, "U13": 1, "U15": 2}
	sorted := make([]model.LotteryPlayer, len(players))
	copy(sorted, players)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if genreOrder[a.Genre] != genreOrder[b.Genre] {
			return genreOrder[a.Genre] < genreOrder[b.Genre]
		}
		if ageOrder[a.AgeGroup] != ageOrder[b.AgeGroup] {
			return ageOrder[a.AgeGroup] < ageOrder[b.AgeGroup]
		}
		aHas, bHas := a.SlotNo != nil, b.SlotNo != nil
		if aHas != bHas {
			return aHas
		}
		if aHas && bHas {
			if *a.GroupLetter != *b.GroupLetter {
				return *a.GroupLetter < *b.GroupLetter
			}
			if *a.SlotNo != *b.SlotNo {
				return *a.SlotNo < *b.SlotNo
			}
		}
		return a.Name < b.Name
	})
	return sorted
}

// randomLotteryToken 生成 16 位 crypto/rand 字母数字 token。
func randomLotteryToken() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate lottery token: %w", err)
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf), nil
}

func trimmedPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
