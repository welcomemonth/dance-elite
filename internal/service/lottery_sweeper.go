package service

import (
	"context"
	"database/sql"
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
type LotterySweeper struct {
	stores  *store.Store
	sms     sms.Client
	baseURL string
	now     func() time.Time
	// warned 记录各赛事上次告警的缺失项与时间，避免每个 tick 重复刷屏。
	warned map[int64]lotterySMSWarn
}

// lotterySMSWarnInterval 是同一赛事同一组缺失配置的最小告警间隔。
const lotterySMSWarnInterval = time.Hour

// lotterySMSWarn 是一条缺失配置告警的去重记录。
type lotterySMSWarn struct {
	at  time.Time
	key string
}

// NewLotterySweeper 构造抽签定时任务；smsClient 为 nil 表示短信未启用。
func NewLotterySweeper(stores *store.Store, smsClient sms.Client, baseURL string) *LotterySweeper {
	return &LotterySweeper{
		stores: stores, sms: smsClient, baseURL: baseURL, now: time.Now,
		warned: make(map[int64]lotterySMSWarn),
	}
}

// Name 实现 ScheduledJob。
func (s *LotterySweeper) Name() string { return "lottery" }

// RunOnce 遍历全部赛事抽签逐个处理；单个失败只记日志，不影响其他赛事。
func (s *LotterySweeper) RunOnce(ctx context.Context) error {
	sessions, err := s.stores.Lottery.List(ctx)
	if err != nil {
		return err
	}
	for i := range sessions {
		if ctx.Err() != nil {
			return nil
		}
		if err := s.process(ctx, &sessions[i]); err != nil {
			logger.WarnEvent(ctx, "lottery.sweep_failed", "抽签定时任务处理失败", "error", err)
		}
	}
	return nil
}

func (s *LotterySweeper) process(ctx context.Context, session *model.LotterySession) error {
	now := s.now()
	window := lotteryWindowOf(session)

	_, _, players, err := s.stores.Lottery.GetByToken(ctx, session.Token)
	if err != nil {
		return err
	}
	if window.ended(now) {
		if pendingLotteryPlayers(players) > 0 {
			if err := s.autoDraw(ctx, session, now); err != nil {
				return err
			}
			// 重新读取以拿到最终结果。
			updated, _, reloaded, err := s.stores.Lottery.GetByToken(ctx, session.Token)
			if err != nil {
				return err
			}
			session, players = updated, reloaded
		}
		s.warnMissingSMSConfig(ctx, session, players, now)
		// 结果短信每轮都重试未成功/未发送的选手，避免一次超时后永久漏发。
		return s.sendJobs(ctx, lotterySMSKindResult, s.resultJobs(session, players, session.Token))
	}

	s.warnMissingSMSConfig(ctx, session, players, now)
	if window.inRemindWindow(now, remindMinutes(session)) {
		return s.sendJobs(ctx, lotterySMSKindRemind, s.remindJobs(session, players, session.Token))
	}
	return nil
}

// warnMissingSMSConfig 在短信已启用、却缺少模板或 public_base_url 时周期告警。
// 同一赛事同一组缺失项每小时最多告警一次；配置补齐后清除记录。
func (s *LotterySweeper) warnMissingSMSConfig(ctx context.Context, session *model.LotterySession,
	players []model.LotteryPlayer, now time.Time) {
	if s.sms == nil {
		return
	}
	missing := missingLotterySMSConfig(session, players, s.baseURL, now)
	if len(missing) == 0 {
		delete(s.warned, session.ID)
		return
	}
	key := strings.Join(missing, ",")
	if last, ok := s.warned[session.ID]; ok && last.key == key && now.Sub(last.at) < lotterySMSWarnInterval {
		return
	}
	s.warned[session.ID] = lotterySMSWarn{at: now, key: key}
	logger.WarnEvent(ctx, "lottery.sms_config_missing", "抽签短信缺少配置，到点将不会发送",
		"event_name", session.EventName, "missing", key)
}

// autoDraw 在截止后完成剩余选手的自动分配。
func (s *LotterySweeper) autoDraw(ctx context.Context, session *model.LotterySession, now time.Time) error {
	drawn, skipped := 0, 0
	err := s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		_, groups, players, err := s.stores.Lottery.GetByTokenTx(ctx, tx, session.Token)
		if err != nil {
			return err
		}
		drawn, skipped = autoDrawLottery(ctx, s.stores, tx, groups, players, now)
		return nil
	})
	if err != nil {
		return err
	}
	if drawn > 0 {
		logger.Event(ctx, "lottery.auto_drawn", "抽签截止，系统自动分配",
			"event_name", session.EventName, "drawn", drawn, "skipped", skipped)
	}
	if skipped > 0 {
		logger.WarnEvent(ctx, "lottery.auto_drawn_short", "存在无可用槽位的选手，请检查分组数据",
			"event_name", session.EventName, "skipped", skipped)
	}
	return nil
}
