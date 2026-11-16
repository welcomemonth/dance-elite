package service

import (
	"context"
	"time"

	"github.com/meetmonth/dance-elite/internal/pkg/logger"
)

// ScheduledJob 是可由调度器周期执行的后台任务（T34）。
// RunOnce 必须幂等：同一轮重复执行不应产生副作用（靠条件更新/CAS 保证）。
type ScheduledJob interface {
	Name() string
	RunOnce(ctx context.Context) error
}

// StatusScheduler 是应用级单调度器：单 goroutine 串行执行全部 Job，
// 避免每个 service 各自起定时器（SQLite 单连接池下并发 tick 只会互相等锁）。
type StatusScheduler struct {
	interval   time.Duration
	jobTimeout time.Duration
	jobs       []ScheduledJob
}

// NewStatusScheduler 构造调度器；interval ≤ 0 时按 60s 兜底。
func NewStatusScheduler(interval time.Duration, jobs ...ScheduledJob) *StatusScheduler {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	timeout := interval
	if timeout > 30*time.Second {
		timeout = 30 * time.Second
	}
	return &StatusScheduler{interval: interval, jobTimeout: timeout, jobs: jobs}
}

// Run 启动调度循环并阻塞，直到 ctx 取消；启动时先立即执行一轮，
// 补齐停机期间到期的状态。ctx 取消后不再开始新一轮，在途一轮执行完再返回。
func (s *StatusScheduler) Run(ctx context.Context) {
	s.RunOnce(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.RunOnce(ctx)
		}
	}
}

// RunOnce 顺序执行全部 Job；单个 Job 失败只记日志，不中断其他 Job。
// 测试直接调用本方法，不依赖真实时钟。
func (s *StatusScheduler) RunOnce(ctx context.Context) {
	log := logger.FromCtx(ctx)
	for _, job := range s.jobs {
		if ctx.Err() != nil {
			return
		}
		jobCtx, cancel := context.WithTimeout(ctx, s.jobTimeout)
		start := time.Now()
		err := job.RunOnce(jobCtx)
		cancel()
		if err != nil {
			if ctx.Err() == nil {
				log.Warnw("定时任务执行失败", "job", job.Name(), "error", err,
					"elapsed_ms", time.Since(start).Milliseconds())
			}
			continue
		}
		log.Debugw("定时任务执行完成", "job", job.Name(), "elapsed_ms", time.Since(start).Milliseconds())
	}
}
