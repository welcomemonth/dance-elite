package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// newSeasonFixture 打开内存库、跑迁移并种子超管与系统配置。
func newSeasonFixture(t *testing.T) (*SeasonService, *store.Store) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db.SQL()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Seed(context.Background(), store.SeedOptions{
		AdminUsername: "admin", AdminPassword: "Init@12345",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stores := store.New(db)
	return NewSeasonService(stores), stores
}

// mustActiveSeason 创建一个日期不限的赛季（创建即按日期判定为 active），供赛事测试使用。
func mustActiveSeason(t *testing.T, svc *SeasonService) *model.AdminSeasonItem {
	t.Helper()
	season, err := svc.Create(context.Background(), 1, model.AdminSeasonRequest{Name: "2027 赛季"})
	if err != nil {
		t.Fatalf("create season: %v", err)
	}
	if season.Status != model.SeasonStatusActive {
		t.Fatalf("season status = %q, want active", season.Status)
	}
	return season
}

func TestSeasonCreateCopiesGroupRules(t *testing.T) {
	svc, stores := newSeasonFixture(t)
	ctx := context.Background()

	season, err := svc.Create(ctx, 1, model.AdminSeasonRequest{
		Name: "2027 赛季", StartDate: "2027-01-01", EndDate: "2027-12-31",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// group_rules 必须从系统设置复制（迁移 0007 种子）。
	template, err := stores.Settings.Get(ctx, "group_rules")
	if err != nil {
		t.Fatalf("get settings group_rules: %v", err)
	}
	if season.GroupRules != template {
		t.Fatalf("group_rules = %q, want settings template %q", season.GroupRules, template)
	}
}

func TestSeasonCreateStatusByToday(t *testing.T) {
	svc, _ := newSeasonFixture(t)
	ctx := context.Background()
	today := utils.TodayDate()

	// 今天落在区间内 → 创建即 active，无需再手工激活。
	inRange, err := svc.Create(ctx, 1, model.AdminSeasonRequest{
		Name: "当前赛季", StartDate: today, EndDate: today,
	})
	if err != nil {
		t.Fatalf("create in-range season: %v", err)
	}
	if inRange.Status != model.SeasonStatusActive {
		t.Fatalf("status = %q, want active", inRange.Status)
	}

	// 已有进行中赛季且时间重叠 → 40901，提示时间不能重复。
	if _, err := svc.Create(ctx, 1, model.AdminSeasonRequest{
		Name: "重叠赛季", StartDate: today, EndDate: today,
	}); err == nil {
		t.Fatal("时间重叠的 active 赛季应被拒绝")
	} else {
		var biz *httpx.BizError
		if !errors.As(err, &biz) || biz.Code != httpx.CodeConflict {
			t.Fatalf("err = %v, want 40901", err)
		}
		if !strings.Contains(biz.Message, "时间不能重复") {
			t.Fatalf("message = %q, want 时间不能重复", biz.Message)
		}
	}

	// 时间不重叠的未来赛季 → upcoming，不影响已有 active 赛季。
	future, err := svc.Create(ctx, 1, model.AdminSeasonRequest{
		Name: "未来赛季", StartDate: "2099-01-01", EndDate: "2099-12-31",
	})
	if err != nil {
		t.Fatalf("create future season: %v", err)
	}
	if future.Status != model.SeasonStatusUpcoming {
		t.Fatalf("future status = %q, want upcoming", future.Status)
	}
}

func TestSeasonCreateInvalidDates(t *testing.T) {
	svc, _ := newSeasonFixture(t)
	ctx := context.Background()

	cases := []model.AdminSeasonRequest{
		{Name: "s", StartDate: "2027-13-01"},                        // 格式非法
		{Name: "s", EndDate: "not-a-date"},                          // 格式非法
		{Name: "s", StartDate: "2027-06-01", EndDate: "2027-01-01"}, // 起始晚于结束
	}
	for _, req := range cases {
		if _, err := svc.Create(ctx, 1, req); err == nil {
			t.Fatalf("Create(%+v) 应被拒绝", req)
		} else {
			var biz *httpx.BizError
			if !errors.As(err, &biz) || biz.Code != httpx.CodeBadRequest {
				t.Fatalf("Create(%+v) err = %v, want 40001", req, err)
			}
		}
	}
}

func TestSeasonUpdateStatusFlow(t *testing.T) {
	svc, _ := newSeasonFixture(t)
	ctx := context.Background()

	season, err := svc.Create(ctx, 1, model.AdminSeasonRequest{
		Name: "S1", StartDate: "2099-01-01", EndDate: "2099-12-31",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// upcoming → active 成功。
	activated, err := svc.Update(ctx, 1, season.ID, model.AdminSeasonRequest{Name: "S1", Status: model.SeasonStatusActive})
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if activated.Status != model.SeasonStatusActive || activated.ArchivedAt != nil {
		t.Fatalf("activated = %+v", activated)
	}

	// 同一赛季重复设置 active（幂等，不算流转）。
	same, err := svc.Update(ctx, 1, season.ID, model.AdminSeasonRequest{Name: "S1", Status: model.SeasonStatusActive})
	if err != nil {
		t.Fatalf("re-activate same status: %v", err)
	}
	if same.Status != model.SeasonStatusActive {
		t.Fatalf("status = %q", same.Status)
	}

	// active → upcoming 回退拒绝（40901）。
	if _, err := svc.Update(ctx, 1, season.ID, model.AdminSeasonRequest{Name: "S1", Status: model.SeasonStatusUpcoming}); err == nil {
		t.Fatal("状态回退应被拒绝")
	} else {
		var biz *httpx.BizError
		if !errors.As(err, &biz) || biz.Code != httpx.CodeConflict {
			t.Fatalf("err = %v, want 40901", err)
		}
	}

	// active → archived 成功且写 archived_at。
	archived, err := svc.Update(ctx, 1, season.ID, model.AdminSeasonRequest{Name: "S1", Status: model.SeasonStatusArchived})
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if archived.Status != model.SeasonStatusArchived || archived.ArchivedAt == nil {
		t.Fatalf("archived = %+v", archived)
	}

	// archived 之后不可再变更（40901）。
	if _, err := svc.Update(ctx, 1, season.ID, model.AdminSeasonRequest{Name: "S1", Status: model.SeasonStatusActive}); err == nil {
		t.Fatal("归档后变更应被拒绝")
	} else {
		var biz *httpx.BizError
		if !errors.As(err, &biz) || biz.Code != httpx.CodeConflict {
			t.Fatalf("err = %v, want 40901", err)
		}
	}
}

func TestSeasonUpdateKeepsDatesWhenOmitted(t *testing.T) {
	svc, _ := newSeasonFixture(t)
	ctx := context.Background()

	season, err := svc.Create(ctx, 1, model.AdminSeasonRequest{
		Name: "2027 赛季", StartDate: "2027-01-01", EndDate: "2027-12-31",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 只改状态、不传日期 → 原有日期必须保留（回归：曾把空值当清空写入 NULL）。
	activated, err := svc.Update(ctx, 1, season.ID, model.AdminSeasonRequest{
		Name: season.Name, Status: model.SeasonStatusActive,
	})
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if activated.StartDate == nil || *activated.StartDate != "2027-01-01" ||
		activated.EndDate == nil || *activated.EndDate != "2027-12-31" {
		t.Fatalf("dates cleared by status-only update: start=%v end=%v",
			activated.StartDate, activated.EndDate)
	}

	// 显式传日期 → 更新为新值。
	updated, err := svc.Update(ctx, 1, season.ID, model.AdminSeasonRequest{
		Name: season.Name, StartDate: "2027-02-01", EndDate: "2027-11-30",
	})
	if err != nil {
		t.Fatalf("update dates: %v", err)
	}
	if updated.StartDate == nil || *updated.StartDate != "2027-02-01" ||
		updated.EndDate == nil || *updated.EndDate != "2027-11-30" {
		t.Fatalf("dates not updated: start=%v end=%v", updated.StartDate, updated.EndDate)
	}
}

func TestSeasonUniqueActive(t *testing.T) {
	svc, _ := newSeasonFixture(t)
	ctx := context.Background()

	mustActiveSeason(t, svc)

	other, err := svc.Create(ctx, 1, model.AdminSeasonRequest{
		Name: "2028 赛季", StartDate: "2099-01-01", EndDate: "2099-12-31",
	})
	if err != nil {
		t.Fatalf("create other: %v", err)
	}
	// 已有 active 赛季时激活另一个 → 40901。
	if _, err := svc.Update(ctx, 1, other.ID, model.AdminSeasonRequest{
		Name: other.Name, Status: model.SeasonStatusActive,
	}); err == nil {
		t.Fatal("同时存在两个 active 赛季应被拒绝")
	} else {
		var biz *httpx.BizError
		if !errors.As(err, &biz) || biz.Code != httpx.CodeConflict {
			t.Fatalf("err = %v, want 40901", err)
		}
	}

	// 赛季不存在 → 40401。
	if _, err := svc.Update(ctx, 1, 9999, model.AdminSeasonRequest{Name: "x", Status: model.SeasonStatusActive}); err == nil {
		t.Fatal("更新不存在赛季应 40401")
	} else {
		var biz *httpx.BizError
		if !errors.As(err, &biz) || biz.Code != httpx.CodeNotFound {
			t.Fatalf("err = %v, want 40401", err)
		}
	}
}

func TestSeasonArchivedRejectsAnyChange(t *testing.T) {
	svc, _ := newSeasonFixture(t)
	ctx := context.Background()

	season, err := svc.Create(ctx, 1, model.AdminSeasonRequest{Name: "S1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Update(ctx, 1, season.ID, model.AdminSeasonRequest{
		Name: season.Name, Status: model.SeasonStatusArchived,
	}); err != nil {
		t.Fatalf("archive: %v", err)
	}

	// 归档后仅改名（不带 status）也必须 40901（回归：拦截只在状态变化时生效）。
	if _, err := svc.Update(ctx, 1, season.ID, model.AdminSeasonRequest{Name: "S1（改）"}); err == nil {
		t.Fatal("归档后改名应被拒绝")
	} else {
		var biz *httpx.BizError
		if !errors.As(err, &biz) || biz.Code != httpx.CodeConflict {
			t.Fatalf("err = %v, want 40901", err)
		}
	}
}

func TestSeasonUpdateInvalidStatusValue(t *testing.T) {
	svc, _ := newSeasonFixture(t)
	ctx := context.Background()

	season, err := svc.Create(ctx, 1, model.AdminSeasonRequest{Name: "S1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Update(ctx, 1, season.ID, model.AdminSeasonRequest{
		Name: season.Name, Status: "deleted",
	}); err == nil {
		t.Fatal("非法状态值应被拒绝")
	} else {
		var biz *httpx.BizError
		if !errors.As(err, &biz) || biz.Code != httpx.CodeBadRequest {
			t.Fatalf("err = %v, want 40001", err)
		}
	}
}
