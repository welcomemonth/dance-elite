package service

import (
	"context"
	"testing"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

const windowTestToken = "WindowToken1"

func lotteryWindowData(start, end time.Time) *model.LotteryData {
	data := &model.LotteryData{
		Version:   2,
		EventName: "窗口测试赛事",
		Genres:    []model.LotteryGenre{{Key: "classical", Label: "古典舞"}},
		Groups:    []model.LotteryGroup{{Genre: "classical", AgeGroup: "U11", Letter: "A", Size: 2}},
		Players: []model.LotteryPlayer{
			{Name: "张三", IDCard: "310101201501011234", Genre: "classical", AgeGroup: "U11", Phone: "13800000001"},
			{Name: "李四", IDCard: "310101201501015678", Genre: "classical", AgeGroup: "U11", Phone: "13800000002"},
		},
	}
	if !start.IsZero() {
		data.StartAt = utils.FormatISO(start)
	}
	if !end.IsZero() {
		data.EndAt = utils.FormatISO(end)
	}
	return data
}

func newLotteryServiceWithData(t *testing.T, data *model.LotteryData) *LotteryService {
	t.Helper()
	st := store.NewLotteryStore(t.TempDir())
	if err := st.Save(context.Background(), windowTestToken, data); err != nil {
		t.Fatalf("save lottery data: %v", err)
	}
	return NewLotteryService(st)
}

func TestLotteryDrawRejectedBeforeStart(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, utils.ChinaLoc)
	svc := newLotteryServiceWithData(t, lotteryWindowData(now.Add(time.Hour), now.Add(24*time.Hour)))
	svc.now = func() time.Time { return now }

	_, err := svc.Draw(context.Background(), windowTestToken, lotteryDrawReq("张三", "310101201501011234"))
	assertLotteryBizCode(t, err, httpx.CodeConflict)
}

func TestLotteryDrawAutoAssignsAfterEnd(t *testing.T) {
	now := time.Date(2026, 11, 27, 22, 0, 0, 0, utils.ChinaLoc)
	svc := newLotteryServiceWithData(t, lotteryWindowData(now.Add(-24*time.Hour), now.Add(-time.Minute)))
	svc.now = func() time.Time { return now }

	result, err := svc.Draw(context.Background(), windowTestToken, lotteryDrawReq("张三", "310101201501011234"))
	if err != nil {
		t.Fatalf("draw after end: %v", err)
	}
	if result.Slot == nil || !result.JustDrawn || !result.AutoDrawn {
		t.Fatalf("result = %+v, want auto-assigned slot", result)
	}

	// 同一轮自动分配应覆盖全部未抽选手。
	other, err := svc.Query(context.Background(), windowTestToken, model.LotteryQueryRequest{
		IDCard: "310101201501015678", Genre: "classical",
	})
	if err != nil {
		t.Fatalf("query after end: %v", err)
	}
	if other.Slot == nil || !other.AutoDrawn {
		t.Fatalf("other = %+v, want auto-assigned slot", other)
	}
	if *other.Slot == *result.Slot {
		t.Fatalf("slots collided: %+v", other.Slot)
	}
}

func TestLotteryMetaExposesWindowAndServerTime(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, utils.ChinaLoc)
	start := now.Add(time.Hour)
	end := now.Add(48 * time.Hour)
	svc := newLotteryServiceWithData(t, lotteryWindowData(start, end))
	svc.now = func() time.Time { return now }

	meta, err := svc.Meta(context.Background(), windowTestToken)
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	if meta.Status != model.LotteryStatusNotStarted {
		t.Fatalf("status = %q", meta.Status)
	}
	if meta.StartAt != utils.FormatISO(start) || meta.EndAt != utils.FormatISO(end) {
		t.Fatalf("window = %q ~ %q", meta.StartAt, meta.EndAt)
	}
	if meta.ServerTime != utils.FormatISO(now) {
		t.Fatalf("server_time = %q", meta.ServerTime)
	}
}

func TestLotteryWindowStatus(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, utils.ChinaLoc)
	window := lotteryWindow{start: now, end: now.Add(time.Hour), hasStart: true, hasEnd: true}

	if got := window.status(now.Add(-time.Second)); got != model.LotteryStatusNotStarted {
		t.Fatalf("before start status = %q", got)
	}
	if got := window.status(now.Add(time.Minute)); got != model.LotteryStatusOngoing {
		t.Fatalf("ongoing status = %q", got)
	}
	if got := window.status(now.Add(time.Hour)); got != model.LotteryStatusEnded {
		t.Fatalf("ended status = %q", got)
	}

	remindWindow := lotteryWindow{
		start: now.Add(-3 * time.Hour), end: now.Add(time.Hour),
		hasStart: true, hasEnd: true,
	}
	if !remindWindow.inRemindWindow(now, 120) {
		t.Fatal("want in reminder window (end-120m ~ end)")
	}
	if remindWindow.inRemindWindow(now.Add(-2*time.Hour), 120) {
		t.Fatal("should not be in reminder window before end-120m")
	}
	if remindWindow.inRemindWindow(now.Add(2*time.Hour), 120) {
		t.Fatal("should not be in reminder window after end")
	}
}
