package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
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
// 抽签数据来自 store.LotteryStore 管理的 JSON 文件，与报名库无关。
type LotteryService struct {
	store *store.LotteryStore
	// now 可注入，测试固定时钟用；生产为 time.Now。
	now func() time.Time
}

// NewLotteryService 构造抽签服务。
func NewLotteryService(lotteryStore *store.LotteryStore) *LotteryService {
	return &LotteryService{store: lotteryStore, now: time.Now}
}

// Meta 返回页面初始化信息（赛事名称、舞种列表、抽签时间窗口与服务端时间）。
func (s *LotteryService) Meta(ctx context.Context, token string) (*model.LotteryMeta, error) {
	data, err := s.load(ctx, token)
	if err != nil {
		return nil, err
	}
	now := s.now()
	window := lotteryWindowOf(data)
	startAt, endAt := data.StartAt, data.EndAt
	if window.hasStart {
		startAt = utils.FormatISO(window.start)
	}
	if window.hasEnd {
		endAt = utils.FormatISO(window.end)
	}
	return &model.LotteryMeta{
		EventName:  data.EventName,
		Genres:     data.Genres,
		StartAt:    startAt,
		EndAt:      endAt,
		ServerTime: utils.FormatISO(now),
		Status:     window.status(now),
	}, nil
}

// Query 按「身份证号 + 舞种」查询抽签结果；未抽签时 Slot 为 nil。
//
// 已过截止时间且仍有未抽选手时，顺带完成自动分配，保证查询到的是最终结果。
func (s *LotteryService) Query(ctx context.Context, token string, req model.LotteryQueryRequest) (*model.LotteryResult, error) {
	now := s.now()
	data, err := s.load(ctx, token)
	if err != nil {
		return nil, err
	}
	if lotteryWindowOf(data).ended(now) && pendingLotteryPlayers(data) > 0 {
		data, err = s.store.Mutate(ctx, token, func(data *model.LotteryData) error {
			autoDrawPending(data, now)
			return nil
		})
		if err != nil {
			return nil, s.wrapStoreError(err)
		}
	}
	player := findLotteryPlayer(data, req.IDCard, req.Genre)
	if player == nil {
		return nil, httpx.ErrNotFound("信息不匹配，请核对证件号和舞种")
	}
	return lotteryResult(data, player, false), nil
}

// Draw 按「姓名 + 身份证号 + 舞种」抽签。
//
// 已抽过的人直接返回原结果（幂等），不会重新分配；首次抽签在互斥锁内
// 从同「组别 × 舞种」尚未占用的槽位里均匀随机取一个并写回文件。
// 未到开始时间拒绝抽签；已过截止时间则按自动分配规则补抽。
func (s *LotteryService) Draw(ctx context.Context, token string, req model.LotteryDrawRequest) (*model.LotteryResult, error) {
	name := normalizeLotteryName(req.Name)
	idCard := utils.NormalizeIDNumber(utils.IDTypeIDCard, req.IDCard)
	genre := strings.TrimSpace(req.Genre)
	now := s.now()

	var result *model.LotteryResult
	justDrawn := false
	_, err := s.store.Mutate(ctx, token, func(data *model.LotteryData) error {
		window := lotteryWindowOf(data)
		if !window.started(now) {
			return httpx.ErrConflict(fmt.Sprintf("抽签尚未开始，开始时间：%s", lotteryTimeText(window.start)))
		}
		player := findLotteryPlayer(data, idCard, genre)
		if player == nil || normalizeLotteryName(player.Name) != name {
			return httpx.ErrNotFound("信息不匹配，请核对姓名、证件号和舞种")
		}
		if player.Slot == nil {
			if window.ended(now) {
				// 已截止：按系统自动分配规则补抽（与定时任务同一逻辑）。
				autoDrawPending(data, now)
				if player.Slot == nil {
					return httpx.ErrConflict("该组别已无可抽签位，请联系赛事客服")
				}
				justDrawn = true
			} else {
				slots := availableLotterySlots(data, player.Genre, player.AgeGroup)
				if len(slots) == 0 {
					return httpx.ErrConflict("该组别已无可抽签位，请联系赛事客服")
				}
				slot, err := pickLotterySlot(slots)
				if err != nil {
					return httpx.ErrInternal(err, "抽签失败，请稍后重试")
				}
				player.Slot = &slot
				player.DrawnAt = utils.NowISO()
				justDrawn = true
			}
		}
		result = lotteryResult(data, player, justDrawn)
		return nil
	})
	if err != nil {
		return nil, s.wrapStoreError(err)
	}
	if justDrawn {
		// 只记录分组与槽位，不记录姓名/证件号（PII）。
		logger.Event(ctx, "lottery.drawn", "抽签完成",
			"genre", result.Genre,
			"age_group", result.AgeGroup,
			"slot", lotterySlotText(*result.Slot),
			"auto", result.AutoDrawn,
		)
	}
	return result, nil
}

// wrapStoreError 把文件存储的「不存在」统一转换为对用户友好的业务错误。
func (s *LotteryService) wrapStoreError(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return httpx.ErrNotFound("抽签链接无效或已失效")
	}
	return err
}

func (s *LotteryService) load(ctx context.Context, token string) (*model.LotteryData, error) {
	data, err := s.store.Load(ctx, token)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("抽签链接无效或已失效")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载抽签数据失败，请稍后重试")
	}
	return data, nil
}

// findLotteryPlayer 按规范化后的证件号与舞种定位选手；不存在返回 nil。
func findLotteryPlayer(data *model.LotteryData, idCard, genre string) *model.LotteryPlayer {
	idCard = utils.NormalizeIDNumber(utils.IDTypeIDCard, idCard)
	genre = strings.TrimSpace(genre)
	for i := range data.Players {
		p := &data.Players[i]
		if p.Genre != genre {
			continue
		}
		if utils.NormalizeIDNumber(utils.IDTypeIDCard, p.IDCard) == idCard {
			return p
		}
	}
	return nil
}

// availableLotterySlots 返回同「组别 × 舞种」下尚未被占用的全部槽位。
func availableLotterySlots(data *model.LotteryData, genre, ageGroup string) []model.LotterySlot {
	taken := make(map[model.LotterySlot]bool, len(data.Players))
	for _, p := range data.Players {
		if p.Genre == genre && p.AgeGroup == ageGroup && p.Slot != nil {
			taken[*p.Slot] = true
		}
	}
	slots := make([]model.LotterySlot, 0)
	for _, g := range data.Groups {
		if g.Genre != genre || g.AgeGroup != ageGroup {
			continue
		}
		for no := 1; no <= g.Size; no++ {
			slot := model.LotterySlot{Letter: g.Letter, No: no}
			if !taken[slot] {
				slots = append(slots, slot)
			}
		}
	}
	return slots
}

// pickLotterySlot 用 crypto/rand 从候选槽位中均匀随机取一个。
func pickLotterySlot(slots []model.LotterySlot) (model.LotterySlot, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(slots))))
	if err != nil {
		return model.LotterySlot{}, fmt.Errorf("pick random slot: %w", err)
	}
	return slots[n.Int64()], nil
}

func lotteryResult(data *model.LotteryData, player *model.LotteryPlayer, justDrawn bool) *model.LotteryResult {
	label := player.Genre
	for _, genre := range data.Genres {
		if genre.Key == player.Genre {
			label = genre.Label
			break
		}
	}
	return &model.LotteryResult{
		Name:        player.Name,
		Genre:       player.Genre,
		GenreLabel:  label,
		AgeGroup:    player.AgeGroup,
		Program:     player.Program,
		Institution: player.Institution,
		Slot:        player.Slot,
		DrawnAt:     player.DrawnAt,
		JustDrawn:   justDrawn,
		AutoDrawn:   player.AutoDrawn,
	}
}

// normalizeLotteryName 去掉姓名中的全部空白，兼容输入时多打的空格。
func normalizeLotteryName(name string) string {
	return strings.Join(strings.Fields(name), "")
}

// lotterySlotText 把槽位格式化为日志可读的 A3 形式。
func lotterySlotText(slot model.LotterySlot) string {
	return fmt.Sprintf("%s%d", slot.Letter, slot.No)
}
