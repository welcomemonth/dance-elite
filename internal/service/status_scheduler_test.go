package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeScheduledJob struct {
	name  string
	err   error
	calls int
}

func (f *fakeScheduledJob) Name() string { return f.name }

func (f *fakeScheduledJob) RunOnce(context.Context) error {
	f.calls++
	return f.err
}

func TestStatusSchedulerContinuesAfterJobError(t *testing.T) {
	failing := &fakeScheduledJob{name: "failing", err: errors.New("boom")}
	ok := &fakeScheduledJob{name: "ok"}
	s := NewStatusScheduler(time.Minute, failing, ok)

	s.RunOnce(context.Background())

	if failing.calls != 1 || ok.calls != 1 {
		t.Fatalf("calls = %d/%d, want 1/1（单 job 失败不中断其他 job）", failing.calls, ok.calls)
	}
}

func TestStatusSchedulerRunStopsOnCancel(t *testing.T) {
	job := &fakeScheduledJob{name: "ok"}
	s := NewStatusScheduler(10*time.Millisecond, job)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	// 至少执行两轮（启动即跑一次 + ticker），再取消。
	deadline := time.Now().Add(2 * time.Second)
	for job.calls < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("调度器未在 ctx 取消后退出")
	}
	if job.calls < 2 {
		t.Fatalf("calls = %d, want ≥2", job.calls)
	}
}
