package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/pkg/sms"
	"github.com/meetmonth/dance-elite/internal/store"
)

// LotterySweeper 是抽签的定时任务（ScheduledJob）：
//   - 截止后：把尚未抽签的选手按同一规则自动分配，并给自动分配者发送结果短信；
//   - 截止前 remind_before_minutes（默认 120 分钟）：给未抽签且未提醒过的手机号发送提醒。
//
// 抽签数据与主业务库解耦，本任务只依赖 LotteryStore 与短信客户端，
// 删除抽签功能时移除本文件与 run.go 中的一行装配即可。
type LotterySweeper struct {
	store   *store.LotteryStore
	sms     sms.Client
	baseURL string
	now     func() time.Time
	// warned 记录各赛事上次告警的缺失项与时间，避免每个 tick 重复刷屏。
	warned map[string]lotterySMSWarn
}

// lotterySMSWarnInterval 是同一赛事同一组缺失配置的最小告警间隔。
const lotterySMSWarnInterval = time.Hour

// lotterySMSWarn 是一条缺失配置告警的去重记录。
type lotterySMSWarn struct {
	at  time.Time
	key string
}

// NewLotterySweeper 构造抽签定时任务；smsClient 为 nil 表示短信未启用。
func NewLotterySweeper(lotteryStore *store.LotteryStore, smsClient sms.Client, baseURL string) *LotterySweeper {
	return &LotterySweeper{
		store: lotteryStore, sms: smsClient, baseURL: baseURL, now: time.Now,
		warned: make(map[string]lotterySMSWarn),
	}
}

// Name 实现 ScheduledJob。
func (s *LotterySweeper) Name() string { return "lottery" }

// RunOnce 遍历全部抽签文件逐个处理；单个文件失败只记日志，不影响其他赛事。
func (s *LotterySweeper) RunOnce(ctx context.Context) error {
	tokens, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, token := range tokens {
		if ctx.Err() != nil {
			return nil
		}
		if err := s.process(ctx, token); err != nil {
			logger.WarnEvent(ctx, "lottery.sweep_failed", "抽签定时任务处理失败", "error", err)
		}
	}
	return nil
}

func (s *LotterySweeper) process(ctx context.Context, token string) error {
	data, err := s.store.Load(ctx, token)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	now := s.now()
	window := lotteryWindowOf(data)
	if window.ended(now) {
		// 先自动分配，再重新读取以拿到最终结果。
		if err := s.autoDraw(ctx, token, now); err != nil {
			return err
		}
		data, err = s.store.Load(ctx, token)
		if err != nil {
			return err
		}
		// 自动分配刚写入 auto_drawn，结果短信缺模板要在同一轮就告警。
		s.warnMissingSMSConfig(ctx, token, data, now)
		return s.sendJobs(ctx, token, lotterySMSKindResult, s.resultJobs(data, token))
	}
	s.warnMissingSMSConfig(ctx, token, data, now)
	if window.inRemindWindow(now, remindMinutes(data)) {
		return s.sendJobs(ctx, token, lotterySMSKindRemind, s.remindJobs(data, token))
	}
	return nil
}

// warnMissingSMSConfig 在短信已启用、却缺少模板或 PUBLIC_BASE_URL 时周期告警，
// 避免到点才发现提醒/结果短信静默不发。同一赛事同一组缺失项每小时最多告警一次；
// 配置补齐后清除记录，再次缺失会立即重新告警。token 是访问凭据，不写入日志。
func (s *LotterySweeper) warnMissingSMSConfig(ctx context.Context, token string, data *model.LotteryData, now time.Time) {
	if s.sms == nil {
		return
	}
	missing := missingLotterySMSConfig(data, s.baseURL, now)
	if len(missing) == 0 {
		delete(s.warned, token)
		return
	}
	key := strings.Join(missing, ",")
	if last, ok := s.warned[token]; ok && last.key == key && now.Sub(last.at) < lotterySMSWarnInterval {
		return
	}
	s.warned[token] = lotterySMSWarn{at: now, key: key}
	logger.WarnEvent(ctx, "lottery.sms_config_missing", "抽签短信缺少配置，到点将不会发送",
		"event_name", data.EventName, "missing", key)
}

// autoDraw 在截止后完成剩余选手的自动分配；没有待抽选手时不写文件。
func (s *LotterySweeper) autoDraw(ctx context.Context, token string, now time.Time) error {
	data, err := s.store.Load(ctx, token)
	if err != nil {
		return err
	}
	if pendingLotteryPlayers(data) == 0 {
		return nil
	}
	drawn, skipped := 0, 0
	_, err = s.store.Mutate(ctx, token, func(data *model.LotteryData) error {
		drawn, skipped = autoDrawPending(data, now)
		return nil
	})
	if err != nil {
		return err
	}
	logger.Event(ctx, "lottery.auto_drawn", "抽签截止，系统自动分配",
		"event", data.EventName, "drawn", drawn, "skipped", skipped)
	if skipped > 0 {
		logger.WarnEvent(ctx, "lottery.auto_drawn_short", "存在无可用槽位的选手，请检查分组数据",
			"event", data.EventName, "skipped", skipped)
	}
	return nil
}
