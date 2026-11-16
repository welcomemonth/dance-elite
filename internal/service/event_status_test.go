package service

import (
	"context"
	"testing"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/utils"
)

func strPtrForTest(s string) *string { return &s }

func TestDueEventStatus(t *testing.T) {
	now := time.Date(2027, 3, 15, 10, 0, 0, 0, utils.ChinaLoc)
	event := func(status string, regStart, regEnd, startAt, eventDate string) model.Event {
		e := model.Event{Status: status}
		if regStart != "" {
			e.RegStartAt = strPtrForTest(regStart)
		}
		if regEnd != "" {
			e.RegEndAt = strPtrForTest(regEnd)
		}
		if startAt != "" {
			e.StartAt = strPtrForTest(startAt)
		}
		if eventDate != "" {
			e.EventDate = strPtrForTest(eventDate)
		}
		return e
	}

	cases := []struct {
		name string
		e    model.Event
		want string
	}{
		{
			"报名中且未到截止", event(model.EventStatusRegistering,
				"2027-03-01T09:00:00+08:00", "2027-03-20T18:00:00+08:00",
				"2027-04-01T09:00:00+08:00", "2027-04-01"),
			model.EventStatusRegistering,
		},
		{
			"到报名截止即关闭", event(model.EventStatusRegistering,
				"2027-03-01T09:00:00+08:00", "2027-03-15T09:00:00+08:00",
				"2027-04-01T09:00:00+08:00", "2027-04-01"),
			model.EventStatusRegClosed,
		},
		{
			"到开始时间进入 ongoing", event(model.EventStatusRegClosed,
				"2027-03-01T09:00:00+08:00", "2027-03-10T18:00:00+08:00",
				"2027-03-15T09:00:00+08:00", "2027-03-16"),
			model.EventStatusOngoing,
		},
		{
			"比赛日期次日 00:00 后 finished", event(model.EventStatusOngoing,
				"2027-03-01T09:00:00+08:00", "2027-03-10T18:00:00+08:00",
				"2027-03-14T09:00:00+08:00", "2027-03-14"),
			model.EventStatusFinished,
		},
		{
			"未到开始时间不 finished", event(model.EventStatusOngoing,
				"2027-03-01T09:00:00+08:00", "2027-03-10T18:00:00+08:00",
				"2027-03-14T09:00:00+08:00", "2027-03-15"),
			model.EventStatusOngoing,
		},
		{
			"手动关闭报名不回退为报名中", event(model.EventStatusRegClosed,
				"2027-03-01T09:00:00+08:00", "2027-03-20T18:00:00+08:00",
				"2027-04-01T09:00:00+08:00", "2027-04-01"),
			model.EventStatusRegClosed,
		},
		{
			"草稿不参与自动流转", event(model.EventStatusDraft,
				"2027-03-01T09:00:00+08:00", "2027-03-10T18:00:00+08:00",
				"2027-03-14T09:00:00+08:00", "2027-03-14"),
			model.EventStatusDraft,
		},
		{
			"已下架不参与自动流转", event(model.EventStatusOffline,
				"2027-03-01T09:00:00+08:00", "2027-03-10T18:00:00+08:00",
				"2027-03-14T09:00:00+08:00", "2027-03-14"),
			model.EventStatusOffline,
		},
	}
	for _, tc := range cases {
		if got := dueEventStatus(&tc.e, now); got != tc.want {
			t.Errorf("%s: dueEventStatus = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestDeriveMiniappStatusRespectsManualRegClosed(t *testing.T) {
	now := time.Date(2027, 3, 15, 10, 0, 0, 0, utils.ChinaLoc)
	e := model.Event{
		Status:     model.EventStatusRegClosed,
		RegStartAt: strPtrForTest("2027-03-01T09:00:00+08:00"),
		RegEndAt:   strPtrForTest("2027-03-20T18:00:00+08:00"),
		StartAt:    strPtrForTest("2027-04-01T09:00:00+08:00"),
		EventDate:  strPtrForTest("2027-04-01"),
	}
	if got := deriveMiniappStatus(&e, now); got != model.MiniappStatusRegClosed {
		t.Fatalf("miniapp status = %q, want reg_closed", got)
	}
}

func TestEventStatusAdvancerRunOnce(t *testing.T) {
	f := newRegFixture(t)
	ctx := context.Background()
	eventID := mustCreateRegisteringEvent(t, f, "自动流转赛事", defaultRegGroups)
	// 报名已截止、抽签与比赛仍未来：目标 reg_closed。
	mustExecSQL(t, f.db, `
		UPDATE events SET
			reg_start_at = '2019-01-01T09:00:00+08:00',
			reg_end_at = '2019-02-01T18:00:00+08:00',
			lottery_at = '2099-03-01T10:00:00+08:00',
			material_check_at = '2099-03-02T18:00:00+08:00',
			start_at = '2099-03-03T09:00:00+08:00'
		WHERE id = ?`, eventID)

	job := NewEventStatusAdvancer(f.stores)
	if err := job.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	event, err := f.stores.Events.Get(ctx, eventID)
	if err != nil {
		t.Fatalf("get event: %v", err)
	}
	if event.Status != model.EventStatusRegClosed {
		t.Fatalf("status = %q, want reg_closed", event.Status)
	}
	var n int
	if err := f.db.SQL().QueryRowContext(ctx, `
		SELECT COUNT(1) FROM admin_logs
		WHERE action = 'admin.event.auto_transition' AND target_id = ? AND admin_id IS NULL`,
		eventID).Scan(&n); err != nil {
		t.Fatalf("count admin logs: %v", err)
	}
	if n != 1 {
		t.Fatalf("auto transition logs = %d, want 1", n)
	}
}
