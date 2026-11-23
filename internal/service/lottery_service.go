package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// LotteryService 承载免登录抽签 H5 的元信息、查询与抽签（API 文档 7.2）。
//
// 抽签数据存 lottery_sessions / lottery_groups / lottery_players；
// 证件号以 HMAC 参与比对，不落明文。
type LotteryService struct {
	stores *store.Store
	idKey  []byte
	// now 可注入，测试固定时钟用；生产为 time.Now。
	now func() time.Time
}

// NewLotteryService 构造抽签服务；idCardKey 为 ID_CARD_HMAC_KEY。
func NewLotteryService(stores *store.Store, idCardKey string) *LotteryService {
	return &LotteryService{stores: stores, idKey: []byte(idCardKey), now: time.Now}
}

// lotteryNotFoundMessage 是证件号未命中时的统一提示。抽签现场大量家长不会
// 区分「没报名」和「填错证件号」，文案必须给出下一步动作，避免直接找客服。
const lotteryNotFoundMessage = "未查询到报名信息：请确认身份证号与报名时填写的一致（18 位，末位 X 用大写）。" +
	"若确认已报名，请把「姓名 + 身份证号」发给赛事客服核对。"

// Meta 返回页面初始化信息（赛事名称、舞种列表、抽签时间窗口与服务端时间）。
func (s *LotteryService) Meta(ctx context.Context, token string) (*model.LotteryMeta, error) {
	session, _, players, err := s.load(ctx, token)
	if err != nil {
		return nil, err
	}
	now := s.now()
	window := lotteryWindowOf(session)
	startAt, endAt := "", ""
	if session.StartAt != nil {
		startAt = *session.StartAt
	}
	if session.EndAt != nil {
		endAt = *session.EndAt
	}
	if window.hasStart {
		startAt = utils.FormatISO(window.start)
	}
	if window.hasEnd {
		endAt = utils.FormatISO(window.end)
	}
	return &model.LotteryMeta{
		EventName:  session.EventName,
		Genres:     lotteryGenresOf(players),
		StartAt:    startAt,
		EndAt:      endAt,
		ServerTime: utils.FormatISO(now),
		Status:     window.status(now),
	}, nil
}

// Query 按证件号查询全部抽签记录（不要求舞种，一次返回该选手所有舞种）。
func (s *LotteryService) Query(ctx context.Context, token string, req model.LotteryLookupRequest) (*model.LotteryLookupResult, error) {
	return s.Lookup(ctx, token, req)
}

// Lookup 按证件号（可选姓名）返回全部命中记录；未命中返回统一的醒目提示。
//
// 已截止的赛事会先完成自动分配，再返回最终结果；未开始也可查询。
func (s *LotteryService) Lookup(ctx context.Context, token string, req model.LotteryLookupRequest) (*model.LotteryLookupResult, error) {
	idCard := utils.NormalizeIDNumber(utils.IDTypeIDCard, req.IDCard)
	hash := s.hashID(idCard)
	name := normalizeLotteryName(req.Name)

	session, _, players, err := s.load(ctx, token)
	if err != nil {
		return nil, err
	}
	now := s.now()
	if lotteryWindowOf(session).ended(now) && pendingLotteryPlayers(players) > 0 {
		err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
			var txErr error
			_, groups, txPlayers, txErr := s.stores.Lottery.GetByTokenTx(ctx, tx, store.NormalizeLotteryToken(token))
			if txErr != nil {
				return txErr
			}
			autoDrawLottery(ctx, s.stores, tx, groups, txPlayers, now)
			players = txPlayers
			return nil
		})
		if err != nil {
			return nil, s.wrapStoreError(err)
		}
	}
	entries := matchingLotteryEntries(players, hash, name)
	if len(entries) == 0 {
		return nil, httpx.ErrNotFound(lotteryNotFoundMessage)
	}
	return &model.LotteryLookupResult{Entries: entries}, nil
}

// drawOutcome 是 Draw 在事务闭包内计算出的结果。
type drawOutcome struct {
	status  string
	choices []model.LotteryGenre
	drawn   []model.LotteryPlayer
}

// Draw 按「姓名 + 证件号」抽签；舞种可选。
//
//   - 只有一条待抽记录：直接抽；
//   - 同一选手有多个舞种都待抽：返回 choose_genre，前端弹窗让家长选择；
//   - 已全部抽完：返回 already_drawn（前端据此提示「已于 X 月 X 日 HH:MM 抽签」）；
//   - 已过截止时间：按系统规则自动分配，不重新随机。
func (s *LotteryService) Draw(ctx context.Context, token string, req model.LotteryDrawRequest) (*model.LotteryDrawResult, error) {
	idCard := utils.NormalizeIDNumber(utils.IDTypeIDCard, req.IDCard)
	hash := s.hashID(idCard)
	name := normalizeLotteryName(req.Name)
	genre := strings.TrimSpace(req.Genre)
	now := s.now()

	var outcome drawOutcome
	var entries []model.LotteryEntry
	err := s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		session, groups, players, err := s.stores.Lottery.GetByTokenTx(ctx, tx, store.NormalizeLotteryToken(token))
		if err != nil {
			return err
		}
		matched := make([]int, 0, len(players))
		for i := range players {
			if players[i].IDCardHash == hash && normalizeLotteryName(players[i].Name) == name {
				matched = append(matched, i)
			}
		}
		if len(matched) == 0 {
			return httpx.ErrNotFound("信息不匹配：请核对姓名和身份证号是否与报名时一致（证件号末位 X 用大写）")
		}
		window := lotteryWindowOf(session)
		if !window.started(now) {
			return httpx.ErrConflict(fmt.Sprintf("抽签尚未开始，开始时间：%s", lotteryTimeText(window.start)))
		}

		// 已截止：所有待抽记录按系统规则自动分配，本次请求不再随机。
		if window.ended(now) {
			pendingBefore := 0
			for _, index := range matched {
				if players[index].SlotNo == nil {
					pendingBefore++
				}
			}
			_, skipped := autoDrawLottery(ctx, s.stores, tx, groups, players, now)
			if skipped > 0 {
				logger.WarnEvent(ctx, "lottery.auto_drawn_short", "存在无可用槽位的选手，请检查分组数据",
					"event_name", session.EventName, "skipped", skipped)
			}
			pendingAfter := 0
			for _, index := range matched {
				if players[index].SlotNo == nil {
					pendingAfter++
				}
			}
			if pendingBefore > pendingAfter {
				outcome.status = model.LotteryOutcomeDrawn
			} else {
				outcome.status = model.LotteryOutcomeAlreadyDrawn
			}
			entries = matchingLotteryEntries(players, hash, name)
			return nil
		}

		// 指定舞种：只抽该舞种。
		if genre != "" {
			target := -1
			for _, index := range matched {
				if players[index].Genre == genre {
					target = index
					break
				}
			}
			if target < 0 {
				return httpx.ErrNotFound("未找到该舞种的报名记录，请核对姓名、证件号和舞种")
			}
			if players[target].SlotNo != nil {
				outcome.status = model.LotteryOutcomeAlreadyDrawn
				entries = matchingLotteryEntries(players, hash, name)
				return nil
			}
			if err := s.assignSlot(ctx, tx, groups, players, &players[target], now); err != nil {
				return err
			}
			outcome.status = model.LotteryOutcomeDrawn
			outcome.drawn = []model.LotteryPlayer{players[target]}
			entries = matchingLotteryEntries(players, hash, name)
			return nil
		}

		pending := make([]int, 0, len(matched))
		for _, index := range matched {
			if players[index].SlotNo == nil {
				pending = append(pending, index)
			}
		}
		switch len(pending) {
		case 0:
			outcome.status = model.LotteryOutcomeAlreadyDrawn
		case 1:
			if err := s.assignSlot(ctx, tx, groups, players, &players[pending[0]], now); err != nil {
				return err
			}
			outcome.status = model.LotteryOutcomeDrawn
			outcome.drawn = []model.LotteryPlayer{players[pending[0]]}
		default:
			outcome.status = model.LotteryOutcomeChooseGenre
			pendingPlayers := make([]model.LotteryPlayer, 0, len(pending))
			for _, index := range pending {
				pendingPlayers = append(pendingPlayers, players[index])
			}
			outcome.choices = lotteryGenreChoices(pendingPlayers)
		}
		entries = matchingLotteryEntries(players, hash, name)
		return nil
	})
	if err != nil {
		return nil, s.wrapStoreError(err)
	}
	for _, player := range outcome.drawn {
		logger.Event(ctx, "lottery.drawn", "抽签完成",
			"genre", player.Genre,
			"age_group", player.AgeGroup,
			"slot", lotterySlotText(*player.Slot()),
		)
	}
	return &model.LotteryDrawResult{
		Status:  outcome.status,
		Entries: entries,
		Choices: outcome.choices,
	}, nil
}

// assignSlot 为一名选手分配随机槽位并写库。
func (s *LotteryService) assignSlot(ctx context.Context, tx *sql.Tx, groups []model.LotteryGroup,
	players []model.LotteryPlayer, player *model.LotteryPlayer, now time.Time) error {
	slots := availableLotterySlots(groups, players, player.Genre, player.AgeGroup)
	if len(slots) == 0 {
		return httpx.ErrConflict("该组别已无可抽签位，请联系赛事客服")
	}
	slot, err := pickLotterySlot(slots)
	if err != nil {
		return httpx.ErrInternal(err, "抽签失败，请稍后重试")
	}
	drawnAt := utils.FormatISO(now)
	ok, err := s.stores.Lottery.UpdatePlayerSlot(ctx, tx, player.ID, slot.Letter, slot.No, drawnAt, false)
	if err != nil {
		return httpx.ErrInternal(err, "抽签失败，请稍后重试")
	}
	if !ok {
		return httpx.ErrConflict("该槽位已被占用，请重试")
	}
	player.GroupLetter = &slot.Letter
	player.SlotNo = &slot.No
	player.DrawnAt = &drawnAt
	player.AutoDrawn = false
	return nil
}

// autoDrawLottery 给所有尚未抽签的选手分配剩余槽位（截止后自动分配）。
// 返回本次分配人数与因槽位不足而跳过的人数。
func autoDrawLottery(ctx context.Context, stores *store.Store, tx *sql.Tx,
	groups []model.LotteryGroup, players []model.LotteryPlayer, now time.Time) (drawn, skipped int) {
	drawnAt := utils.FormatISO(now)
	for i := range players {
		player := &players[i]
		if player.SlotNo != nil {
			continue
		}
		slots := availableLotterySlots(groups, players, player.Genre, player.AgeGroup)
		if len(slots) == 0 {
			skipped++
			continue
		}
		slot, err := pickLotterySlot(slots)
		if err != nil {
			skipped++
			continue
		}
		ok, err := stores.Lottery.UpdatePlayerSlot(ctx, tx, player.ID, slot.Letter, slot.No, drawnAt, true)
		if err != nil || !ok {
			skipped++
			continue
		}
		player.GroupLetter = &slot.Letter
		player.SlotNo = &slot.No
		player.DrawnAt = &drawnAt
		player.AutoDrawn = true
		drawn++
	}
	return drawn, skipped
}

// wrapStoreError 把存储层的「不存在」统一转换为对用户友好的业务错误。
func (s *LotteryService) wrapStoreError(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return httpx.ErrNotFound("抽签链接无效或已失效")
	}
	return err
}

func (s *LotteryService) load(ctx context.Context, token string) (*model.LotterySession, []model.LotteryGroup, []model.LotteryPlayer, error) {
	session, groups, players, err := s.stores.Lottery.GetByToken(ctx, store.NormalizeLotteryToken(token))
	if err != nil {
		return nil, nil, nil, s.wrapStoreError(err)
	}
	return session, groups, players, nil
}

func (s *LotteryService) hashID(idCard string) string {
	return utils.HashIDNumber(s.idKey, utils.IDTypeIDCard, idCard)
}

// matchingLotteryEntries 把命中选手转换为响应记录，按舞种配置顺序排列。
func matchingLotteryEntries(players []model.LotteryPlayer, hash, name string) []model.LotteryEntry {
	matched := make([]model.LotteryPlayer, 0, len(players))
	for _, player := range players {
		if player.IDCardHash != hash {
			continue
		}
		if name != "" && normalizeLotteryName(player.Name) != name {
			continue
		}
		matched = append(matched, player)
	}
	order := make(map[string]int, len(lotteryGenreOrder))
	for i, genre := range lotteryGenreOrder {
		order[genre] = i
	}
	sort.SliceStable(matched, func(i, j int) bool {
		return order[matched[i].Genre] < order[matched[j].Genre]
	})
	entries := make([]model.LotteryEntry, 0, len(matched))
	for _, player := range matched {
		entries = append(entries, lotteryEntry(player))
	}
	return entries
}

// lotteryEntry 把选手记录转换为展示用结构。
func lotteryEntry(player model.LotteryPlayer) model.LotteryEntry {
	entry := model.LotteryEntry{
		Name:       player.Name,
		Genre:      player.Genre,
		GenreLabel: lotteryGenreLabel(player.Genre),
		AgeGroup:   player.AgeGroup,
		Slot:       player.Slot(),
		Drawn:      player.SlotNo != nil,
		AutoDrawn:  player.AutoDrawn,
	}
	if player.Program != nil {
		entry.Program = *player.Program
	}
	if player.Institution != nil {
		entry.Institution = *player.Institution
	}
	if player.DrawnAt != nil {
		entry.DrawnAt = *player.DrawnAt
	}
	return entry
}
