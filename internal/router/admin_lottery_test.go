package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/token"
	"github.com/meetmonth/dance-elite/internal/service"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/xuri/excelize/v2"
)

// newAdminLotteryTestEngine 装配后台认证 + 赛季/赛事 + 后台抽签 + 抽签 H5 路由。
func newAdminLotteryTestEngine(t *testing.T) (*gin.Engine, *token.Signer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
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
	signer := token.NewSigner("test-secret", time.Hour)
	return New(Deps{
		Stores:       stores,
		AdminAuth:    service.NewAdminAuthService(stores, signer),
		Seasons:      service.NewSeasonService(stores),
		Events:       service.NewEventService(stores),
		Lottery:      service.NewLotteryService(stores, testLotteryRouteKey),
		LotteryAdmin: service.NewAdminLotteryService(stores, "https://api.example.com", testLotteryRouteKey),
		Tokens:       signer,
	}), signer
}

// lotteryRouteExcel 生成后台导入用 Excel。
func lotteryRouteExcel(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	header := []any{"姓名", "身份证号", "组别及场次", "舞种"}
	if err := f.SetSheetRow("Sheet1", "A1", &header); err != nil {
		t.Fatalf("write header: %v", err)
	}
	rows := [][]any{
		{"张三", "310101201501011234", "U11 古典舞组", "古典舞"},
		{"张三", "310101201501011234", "U11 民族民间舞组", "民族民间舞"},
		{"李四", "310101201501015678", "U11 古典舞组", "古典舞"},
	}
	for i, row := range rows {
		if err := f.SetSheetRow("Sheet1", fmt.Sprintf("A%d", i+2), &row); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("write excel: %v", err)
	}
	return buf.Bytes()
}

// postLotteryImport 用 multipart 上传 Excel 创建/覆盖抽签。
func postLotteryImport(t *testing.T, engine *gin.Engine, auth map[string]string, eventID int64, replace bool) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"event_id":              fmt.Sprint(eventID),
		"source":                "excel",
		"start_at":              "2026-11-01 10:00",
		"end_at":                "2026-12-31 22:00",
		"replace":               fmt.Sprint(replace),
		"remind_before_minutes": "120",
		"sms_remind_template":   "SMS_remind",
		"sms_result_template":   "SMS_result",
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field %s: %v", key, err)
		}
	}
	part, err := writer.CreateFormFile("file", "lottery.xlsx")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(lotteryRouteExcel(t)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/lottery/sessions", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for key, value := range auth {
		req.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestAdminLotteryRoutesImportDrawExport(t *testing.T) {
	engine, _ := newAdminLotteryTestEngine(t)
	bearer := adminToken(t, engine)
	auth := map[string]string{"Authorization": "Bearer " + bearer}

	seasonID := setupSeason(t, engine, auth)
	createEvent := postJSON(t, engine, http.MethodPost, "/api/v1/admin/events",
		fmt.Sprintf(adminEventBody, seasonID), auth)
	if createEvent.Code != http.StatusOK {
		t.Fatalf("create event status = %d, body = %s", createEvent.Code, createEvent.Body)
	}
	var eventResp struct {
		Data struct {
			EventID int64 `json:"event_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createEvent.Body.Bytes(), &eventResp); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	eventID := eventResp.Data.EventID
	if eventID == 0 {
		t.Fatalf("event id = 0, body = %s", createEvent.Body)
	}

	w := postLotteryImport(t, engine, auth, eventID, false)
	if w.Code != http.StatusOK {
		t.Fatalf("import status = %d, body = %s", w.Code, w.Body)
	}
	var imported struct {
		Data struct {
			Token       string `json:"token"`
			Link        string `json:"link"`
			PlayerCount int    `json:"player_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &imported); err != nil {
		t.Fatalf("decode import: %v", err)
	}
	if imported.Data.Token == "" || imported.Data.PlayerCount != 3 {
		t.Fatalf("import = %+v", imported.Data)
	}
	if imported.Data.Link != "https://api.example.com/lottery/"+imported.Data.Token {
		t.Fatalf("link = %q", imported.Data.Link)
	}

	// 同赛事重复导入未勾选覆盖 → 40901。
	w = postLotteryImport(t, engine, auth, eventID, false)
	code, _ := decodeLotteryBody(t, w)
	if w.Code != http.StatusConflict || code != httpx.CodeConflict {
		t.Fatalf("duplicate import: status=%d code=%d", w.Code, code)
	}

	// H5 抽签：选择舞种后写入槽位。
	drawPath := "/h5/lottery/" + imported.Data.Token + "/draw"
	w = lotteryPost(t, engine, drawPath, `{"name":"张三","id_card":"310101201501011234","genre":"classical"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("draw status = %d, body = %s", w.Code, w.Body)
	}

	// 后台导出：包含表头 + 3 行，身份证号为文本。
	w = getJSON(t, engine, "/api/v1/admin/lottery/sessions/"+imported.Data.Token+"/export", auth)
	if w.Code != http.StatusOK {
		t.Fatalf("export status = %d, body = %s", w.Code, w.Body)
	}
	if contentType := w.Header().Get("Content-Type"); contentType != exportMIME {
		t.Fatalf("content-type = %q", contentType)
	}
	xl, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("open export: %v", err)
	}
	defer func() { _ = xl.Close() }()
	rows, err := xl.GetRows("抽签结果")
	if err != nil {
		t.Fatalf("read export rows: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("export rows = %d, want 4", len(rows))
	}
	if rows[1][2] != "310101201501011234" {
		t.Fatalf("id card = %q", rows[1][2])
	}
}

// postLotteryImportUnbound 上传 Excel 且不关联赛事（title 作为抽签名称）。
func postLotteryImportUnbound(t *testing.T, engine *gin.Engine, auth map[string]string, title string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range map[string]string{"source": "excel", "title": title} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field %s: %v", key, err)
		}
	}
	part, err := writer.CreateFormFile("file", "lottery.xlsx")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(lotteryRouteExcel(t)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/lottery/sessions", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for key, value := range auth {
		req.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestAdminLotteryRouteImportWithoutEvent(t *testing.T) {
	engine, _ := newAdminLotteryTestEngine(t)
	bearer := adminToken(t, engine)
	auth := map[string]string{"Authorization": "Bearer " + bearer}

	w := postLotteryImportUnbound(t, engine, auth, "外部机构抽签")
	if w.Code != http.StatusOK {
		t.Fatalf("import without event status = %d, body = %s", w.Code, w.Body)
	}
	var imported struct {
		Data struct {
			Token     string `json:"token"`
			EventID   *int64 `json:"event_id"`
			EventName string `json:"event_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &imported); err != nil {
		t.Fatalf("decode import: %v", err)
	}
	if imported.Data.EventID != nil || imported.Data.EventName != "外部机构抽签" {
		t.Fatalf("imported = %+v", imported.Data)
	}
}
