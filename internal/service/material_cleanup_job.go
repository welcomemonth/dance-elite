package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/pkg/storage"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// adminActionMaterialsCleaned 是赛后媒体清理的审计动作（admin_id 为空）。
const adminActionMaterialsCleaned = "admin.event.materials_cleaned"

// MaterialCleanupJob 是赛后媒体清理任务（S5-3，报名功能 Q22）：
// 赛事 finished 且超过宽限期后，删除 uploads/{event_id}/ 并把报名单的
// photo_path/audio_path 置 NULL；头像目录与文本资料不受影响。
type MaterialCleanupJob struct {
	stores *store.Store
	files  *storage.Store
	now    func() time.Time
	grace  time.Duration
	batch  int
}

// NewMaterialCleanupJob 构造赛后清理任务；delayHours 为结束后宽限期（小时）。
func NewMaterialCleanupJob(stores *store.Store, files *storage.Store, delayHours int) *MaterialCleanupJob {
	return &MaterialCleanupJob{
		stores: stores,
		files:  files,
		now:    time.Now,
		grace:  time.Duration(delayHours) * time.Hour,
		batch:  20,
	}
}

// Name 返回任务名（调度器日志用）。
func (j *MaterialCleanupJob) Name() string { return "material_cleanup" }

// RunOnce 执行一轮清理：删除文件 → 事务内清空路径并标记已清理。
// 只处理结束时刻 + 宽限期已到的赛事；单个赛事失败不中断其他赛事，
// 下一轮会重试（目录不存在视为成功）。
func (j *MaterialCleanupJob) RunOnce(ctx context.Context) error {
	if j.files == nil {
		return errors.New("material cleanup: storage not configured")
	}
	events, err := j.stores.Events.ListFinishedForCleanup(ctx, j.batch)
	if err != nil {
		return fmt.Errorf("查询待清理赛事: %w", err)
	}
	now := j.now()
	var firstErr error
	for i := range events {
		event := &events[i]
		finishAt, ok := eventFinishAt(event)
		if !ok {
			logger.WarnEvent(ctx, "event.materials.cleanup_skipped", "赛事缺少比赛日期，跳过媒体清理",
				"event_id", event.ID)
			continue
		}
		if now.Before(finishAt.Add(j.grace)) {
			continue
		}
		if err := j.cleanup(ctx, event.ID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// cleanup 清理单个赛事的媒体文件与数据库路径。
func (j *MaterialCleanupJob) cleanup(ctx context.Context, eventID int64) error {
	removed, err := j.files.RemoveEventDir(eventID)
	if err != nil {
		return fmt.Errorf("删除赛事 %d 媒体目录: %w", eventID, err)
	}
	cleanedAt := utils.FormatISO(j.now())
	err = j.stores.WithTx(ctx, func(tx *sql.Tx) error {
		if err := j.stores.Registrations.ClearMaterialsTx(ctx, tx, eventID); err != nil {
			return err
		}
		if err := j.stores.Events.MarkMaterialsCleanedTx(ctx, tx, eventID, cleanedAt); err != nil {
			return err
		}
		targetType := "event"
		return adminLog(ctx, j.stores, tx, nil, adminActionMaterialsCleaned,
			&targetType, &eventID, map[string]any{"files_removed": removed})
	})
	if err != nil {
		return fmt.Errorf("记录赛事 %d 媒体清理: %w", eventID, err)
	}
	logger.Event(ctx, "event.materials.cleaned", "赛事媒体已清理",
		"event_id", eventID, "files_removed", removed)
	return nil
}
