package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/store"
)

// adminActionEventAutoTransition 是定时任务推进赛事状态的审计动作（admin_id 为空）。
const adminActionEventAutoTransition = "admin.event.auto_transition"

// EventStatusAdvancer 按时间线自动推进赛事状态（T34）：
// registering → reg_closed → ongoing → finished，只前进不回退。
// 未发布（draft/unpublished/offline）赛事不在候选集内，不会被自动发布。
type EventStatusAdvancer struct {
	stores *store.Store
	now    func() time.Time
	batch  int
}

// NewEventStatusAdvancer 构造赛事状态推进任务。
func NewEventStatusAdvancer(stores *store.Store) *EventStatusAdvancer {
	return &EventStatusAdvancer{stores: stores, now: time.Now, batch: 200}
}

// Name 返回任务名（调度器日志用）。
func (j *EventStatusAdvancer) Name() string { return "event_status" }

// RunOnce 执行一轮推进：推导目标状态 → 单事务 CAS 落库 + 审计日志。
// 并发手动流转（CAS 未命中）跳过，不视为错误。
func (j *EventStatusAdvancer) RunOnce(ctx context.Context) error {
	events, err := j.stores.Events.ListAutoCandidates(ctx, j.batch)
	if err != nil {
		return fmt.Errorf("查询待流转赛事: %w", err)
	}
	now := j.now()
	var firstErr error
	for i := range events {
		event := events[i]
		target := dueEventStatus(&event, now)
		if target == event.Status {
			continue
		}
		if err := j.advance(ctx, event.ID, event.Status, target); err != nil {
			if errors.Is(err, store.ErrConflict) || errors.Is(err, store.ErrNotFound) {
				continue
			}
			if firstErr == nil {
				firstErr = fmt.Errorf("推进赛事 %d 状态: %w", event.ID, err)
			}
		}
	}
	return firstErr
}

// advance 在单事务内按 CAS 更新赛事状态并写审计日志。
func (j *EventStatusAdvancer) advance(ctx context.Context, eventID int64, from, to string) error {
	err := j.stores.WithTx(ctx, func(tx *sql.Tx) error {
		if err := j.stores.Events.TransitionStatus(ctx, tx, eventID, []string{from}, to); err != nil {
			return err
		}
		targetType := "event"
		return adminLog(ctx, j.stores, tx, nil, adminActionEventAutoTransition, &targetType, &eventID,
			map[string]any{"from": from, "to": to})
	})
	if err != nil {
		return err
	}
	logger.Event(ctx, "event.status.advanced", "赛事状态已自动流转",
		"event_id", eventID, "from", from, "to", to)
	return nil
}
