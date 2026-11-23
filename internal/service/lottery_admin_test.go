package service

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strconv"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
	"github.com/xuri/excelize/v2"
)

type lotteryAdminEnv struct {
	svc     *AdminLotteryService
	stores  *store.Store
	eventID int64
	adminID int64
}

// newLotteryAdminEnv 准备内存库 + 赛季 + 赛事，并构造后台抽签服务。
func newLotteryAdminEnv(t *testing.T) *lotteryAdminEnv {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err := store.Migrate(ctx, db.SQL()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Seed(ctx, store.SeedOptions{AdminUsername: "admin", AdminPassword: "Init@12345"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stores := store.New(db)
	seasons := NewSeasonService(stores)
	season, err := seasons.Create(ctx, 1, model.AdminSeasonRequest{
		Name: "2027 赛季", StartDate: "2027-01-01", EndDate: "2027-12-31",
	})
	if err != nil {
		t.Fatalf("create season: %v", err)
	}
	if _, err := seasons.Update(ctx, 1, season.ID, model.AdminSeasonRequest{
		Name: "2027 赛季", StartDate: "2027-01-01", EndDate: "2027-12-31", Status: model.SeasonStatusActive,
	}); err != nil {
		t.Fatalf("activate season: %v", err)
	}
	events := NewEventService(stores)
	created, err := events.Create(ctx, 1, model.AdminCreateEventRequest{
		Name: "2027 上海甲级赛", SeasonID: season.ID, Level: model.EventLevelGradeA,
		Venue: "上海国际舞蹈中心", EventDate: "2027-03-15", Fee: ptrOf[int64](98000),
		Timeline: model.EventTimeline{
			RegStartAt: "2027-01-01T09:00:00+08:00", RegEndAt: "2027-02-01T18:00:00+08:00",
			LotteryAt: "2027-02-05T10:00:00+08:00", MaterialCheckAt: "2027-03-01T18:00:00+08:00",
			StartAt: "2027-03-15T09:00:00+08:00",
		},
		Groups: []model.AdminEventGroupInput{
			{Level: model.GroupLevelU11, DanceType: model.DanceTypeClassical, Quota: 40},
			{Level: model.GroupLevelU11, DanceType: model.DanceTypeFolk, Quota: 40},
		},
		Publish: true,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := events.CloseRegistration(ctx, 1, created.EventID); err != nil {
		t.Fatalf("close registration: %v", err)
	}
	return &lotteryAdminEnv{
		svc:     NewAdminLotteryService(stores, "https://api.example.com", testLotteryIDKey),
		stores:  stores,
		eventID: created.EventID,
		adminID: 1,
	}
}

// lotteryTestExcel 生成报名 Excel（表头 + 三行数据）。
func lotteryTestExcel(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	headers := []any{"姓名", "身份证号", "组别及场次", "舞种", "剧目名称", "选送机构", "手机号"}
	if err := f.SetSheetRow("Sheet1", "A1", &headers); err != nil {
		t.Fatalf("write header: %v", err)
	}
	rows := [][]any{
		{"张三", "310101201501011234", "U11 古典舞组", "古典舞", "小荷", "星光舞蹈", "13800000001"},
		{"张三", "310101201501011234", "U11 民族民间舞组", "民族民间舞", "山泉", "星光舞蹈", "13800000001"},
		{"李四", "310101201501015678", "U11 古典舞组", "古典舞", "春晓", "月光艺术", "13800000002"},
	}
	for i, row := range rows {
		if err := f.SetSheetRow("Sheet1", "A"+strconv.Itoa(i+2), &row); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("write excel: %v", err)
	}
	return buf.Bytes()
}

func (e *lotteryAdminEnv) importExcel(t *testing.T, replace bool) *model.AdminLotteryCreateResult {
	t.Helper()
	result, err := e.svc.Import(context.Background(), e.adminID, AdminLotteryImportInput{
		EventID: e.eventID,
		Source:  model.LotterySourceExcel,
		Excel:   bytes.NewReader(lotteryTestExcel(t)),
		StartAt: "2027-02-05 10:00",
		EndAt:   "2027-02-06 22:00",
		Remind:  120,
		Replace: replace,
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	return result
}

func TestAdminLotteryImportListAndExport(t *testing.T) {
	env := newLotteryAdminEnv(t)
	created := env.importExcel(t, false)
	if created.PlayerCount != 3 || created.DrawnCount != 0 || created.PendingCount != 3 {
		t.Fatalf("created = %+v", created)
	}
	if created.EventID == nil || *created.EventID != env.eventID || created.EventName != "2027 上海甲级赛" {
		t.Fatalf("event link = %+v", created)
	}
	if created.Link != "https://api.example.com/lottery/"+created.Token {
		t.Fatalf("link = %q", created.Link)
	}
	if created.StartAt == "" || created.EndAt == "" {
		t.Fatalf("window missing: %+v", created)
	}

	list, err := env.svc.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.List) != 1 || list.List[0].Token != created.Token {
		t.Fatalf("list = %+v", list.List)
	}

	detail, err := env.svc.Get(context.Background(), created.Token)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(detail.Groups) != 2 {
		t.Fatalf("groups = %+v", detail.Groups)
	}

	file, err := env.svc.Export(context.Background(), env.adminID, created.Token)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if file.Rows != 3 {
		t.Fatalf("export rows = %d", file.Rows)
	}
	xl, err := excelize.OpenReader(bytes.NewReader(file.Content))
	if err != nil {
		t.Fatalf("open exported workbook: %v", err)
	}
	defer func() { _ = xl.Close() }()
	rows, err := xl.GetRows(lotteryExportSheet)
	if err != nil {
		t.Fatalf("read export sheet: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("export lines = %d, want 4", len(rows))
	}
	if rows[0][0] != "序号" || rows[0][2] != "身份证号" {
		t.Fatalf("header = %v", rows[0])
	}
	if rows[1][2] != "310101201501011234" {
		t.Fatalf("id card = %q, want text", rows[1][2])
	}
	if len(rows[1]) < 8 || rows[1][7] != "未抽签" {
		t.Fatalf("draw order = %v", rows[1])
	}
}

func TestAdminLotteryImportRequiresReplaceForSameEvent(t *testing.T) {
	env := newLotteryAdminEnv(t)
	first := env.importExcel(t, false)

	_, err := env.svc.Import(context.Background(), env.adminID, AdminLotteryImportInput{
		EventID: env.eventID,
		Source:  model.LotterySourceExcel,
		Excel:   bytes.NewReader(lotteryTestExcel(t)),
	})
	assertLotteryBizCode(t, err, httpx.CodeConflict)

	second := env.importExcel(t, true)
	if second.Token != first.Token {
		t.Fatalf("replace token = %q, want %q", second.Token, first.Token)
	}
	if second.CreatedAt != first.CreatedAt {
		t.Fatalf("created_at = %q, want %q", second.CreatedAt, first.CreatedAt)
	}
}

func TestAdminLotteryUpdateSettings(t *testing.T) {
	env := newLotteryAdminEnv(t)
	created := env.importExcel(t, false)

	updated, err := env.svc.UpdateSettings(context.Background(), env.adminID, created.Token,
		model.AdminLotterySettingsRequest{
			StartAt:             "2027-02-04 08:30",
			EndAt:               "2027-02-07 20:00",
			RemindBeforeMinutes: 60,
			SMSRemindTemplate:   "SMS_1",
			SMSResultTemplate:   "SMS_2",
		})
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if updated.RemindBeforeMinutes != 60 || updated.SMSRemindTemplate != "SMS_1" {
		t.Fatalf("updated = %+v", updated)
	}
	if updated.StartAt != "2027-02-04T08:30:00+08:00" {
		t.Fatalf("start_at = %q", updated.StartAt)
	}

	_, err = env.svc.UpdateSettings(context.Background(), env.adminID, created.Token,
		model.AdminLotterySettingsRequest{StartAt: "2027-02-08 10:00", EndAt: "2027-02-07 10:00"})
	if err == nil {
		t.Fatal("want error for start after end")
	}
	var biz *httpx.BizError
	if !errors.As(err, &biz) || biz.Code != httpx.CodeBadRequest {
		t.Fatalf("err = %v, want bad request", err)
	}
}

// seedRegistrations 直接写入两笔有效报名（古典舞/民族民间舞各一），
// 模拟赛事报名库，用于「从赛事报名生成抽签」用例。
func (e *lotteryAdminEnv) seedRegistrations(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	groups, err := e.stores.Groups.ListByEvent(ctx, e.eventID)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	groupID := map[string]int64{}
	for _, group := range groups {
		groupID[group.DanceType] = group.ID
	}
	if err := e.stores.WithTx(ctx, func(tx *sql.Tx) error {
		userRes, err := tx.ExecContext(ctx, `INSERT INTO users (openid, nickname) VALUES (?, ?)`,
			"lottery-openid-1", "抽签测试家长")
		if err != nil {
			return err
		}
		userID, _ := userRes.LastInsertId()
		rows := []struct {
			playerNo, name, idCard, genre string
		}{
			{"LT000001", "张三", "310101201501011234", model.DanceTypeClassical},
			{"LT000002", "李四", "310101201501015678", model.DanceTypeFolk},
		}
		for _, row := range rows {
			hash := utils.HashIDNumber([]byte(testLotteryIDKey), utils.IDTypeIDCard, row.idCard)
			res, err := tx.ExecContext(ctx, `INSERT INTO players (player_no, name, id_card_hash) VALUES (?, ?, ?)`,
				row.playerNo, row.name, hash)
			if err != nil {
				return err
			}
			playerID, _ := res.LastInsertId()
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO registrations (event_id, group_id, player_id, user_id, dance_type, status,
					institution, program_name, contact_phone)
				VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?)`,
				e.eventID, groupID[row.genre], playerID, userID, row.genre,
				"星光舞蹈", "测试剧目", "13800000001"); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed registrations: %v", err)
	}
}

func TestAdminLotteryImportFromEventRegistrations(t *testing.T) {
	env := newLotteryAdminEnv(t)
	env.seedRegistrations(t)

	result, err := env.svc.Import(context.Background(), env.adminID, AdminLotteryImportInput{
		EventID: env.eventID,
		Source:  model.LotterySourceEvent,
		StartAt: "2026-11-01 10:00",
		EndAt:   "2026-12-31 22:00",
	})
	if err != nil {
		t.Fatalf("import from event: %v", err)
	}
	if result.Source != model.LotterySourceEvent || result.PlayerCount != 2 || result.PendingCount != 2 {
		t.Fatalf("result = %+v", result)
	}
	detail, err := env.svc.Get(context.Background(), result.Token)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(detail.Groups) != 2 {
		t.Fatalf("groups = %+v", detail.Groups)
	}

	file, err := env.svc.Export(context.Background(), env.adminID, result.Token)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	xl, err := excelize.OpenReader(bytes.NewReader(file.Content))
	if err != nil {
		t.Fatalf("open export: %v", err)
	}
	defer func() { _ = xl.Close() }()
	rows, err := xl.GetRows(lotteryExportSheet)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("export rows = %d, want 3", len(rows))
	}
	// 赛事报名库只有证件号哈希，导出时明确标注无明文。
	if rows[1][2] != "（报名库无明文）" {
		t.Fatalf("event-source id card = %q", rows[1][2])
	}
}

func TestAdminLotteryImportExcelWithoutEvent(t *testing.T) {
	env := newLotteryAdminEnv(t)
	ctx := context.Background()

	first, err := env.svc.Import(ctx, env.adminID, AdminLotteryImportInput{
		Source: model.LotterySourceExcel,
		Title:  "外部机构汇报抽签",
		Excel:  bytes.NewReader(lotteryTestExcel(t)),
	})
	if err != nil {
		t.Fatalf("import without event: %v", err)
	}
	if first.EventID != nil || first.EventName != "外部机构汇报抽签" {
		t.Fatalf("session = %+v", first)
	}
	if first.PlayerCount != 3 || first.Link == "" {
		t.Fatalf("result = %+v", first)
	}

	// 不关联赛事时每次导入都是新链接。
	second, err := env.svc.Import(ctx, env.adminID, AdminLotteryImportInput{
		Source: model.LotterySourceExcel,
		Title:  "外部机构汇报抽签",
		Excel:  bytes.NewReader(lotteryTestExcel(t)),
	})
	if err != nil {
		t.Fatalf("second import without event: %v", err)
	}
	if second.Token == first.Token {
		t.Fatal("unbound import should create a new lottery, not replace")
	}

	list, err := env.svc.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.List) != 2 {
		t.Fatalf("list = %+v", list.List)
	}
}

func TestAdminLotteryImportValidation(t *testing.T) {
	env := newLotteryAdminEnv(t)
	ctx := context.Background()

	_, err := env.svc.Import(ctx, env.adminID, AdminLotteryImportInput{
		Source: model.LotterySourceExcel,
		Excel:  bytes.NewReader(lotteryTestExcel(t)),
	})
	assertLotteryBizCode(t, err, httpx.CodeBadRequest)

	_, err = env.svc.Import(ctx, env.adminID, AdminLotteryImportInput{
		Source: model.LotterySourceEvent,
	})
	assertLotteryBizCode(t, err, httpx.CodeBadRequest)
}
