package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/token"
	"github.com/meetmonth/dance-elite/internal/service"
	"github.com/meetmonth/dance-elite/internal/store"
)

// newRegistrationQueryTestEngine 构造挂载「我的报名」查询路由的测试引擎。
func newRegistrationQueryTestEngine(t *testing.T) (*gin.Engine, *token.Signer, *store.DB) {
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
	players := service.NewPlayerService(stores, "test-hmac-key")
	return New(Deps{
		Stores:              stores,
		AdminAuth:           service.NewAdminAuthService(stores, signer),
		Seasons:             service.NewSeasonService(stores),
		Events:              service.NewEventService(stores),
		PublicEvents:        service.NewPublicEventService(stores),
		Registrations:       service.NewRegistrationService(stores, players),
		RegistrationQueries: service.NewRegistrationQueryService(stores),
		Tokens:              signer,
	}), signer, db
}

// seedRouterRegistration 直接插入一条报名与订单，返回报名单/订单 ID。
func seedRouterRegistration(t *testing.T, db *store.DB, eventID, userID int64,
	playerNo, orderNo, orderStatus, regStatus string) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	var groupID int64
	if err := db.SQL().QueryRowContext(ctx,
		`SELECT id FROM event_groups WHERE event_id = ? AND enabled = 1 ORDER BY id LIMIT 1`,
		eventID).Scan(&groupID); err != nil {
		t.Fatalf("query group: %v", err)
	}
	res, err := db.SQL().ExecContext(ctx,
		`INSERT INTO players (player_no, name, id_card_hash) VALUES (?, ?, ?)`,
		playerNo, "张小雨", "hash-"+playerNo)
	if err != nil {
		t.Fatalf("insert player %s: %v", playerNo, err)
	}
	playerID, _ := res.LastInsertId()
	res, err = db.SQL().ExecContext(ctx,
		`INSERT INTO registrations (event_id, group_id, player_id, user_id, dance_type,
			institution, status) VALUES (?, ?, ?, ?, 'classical', '上海灵舞艺术中心', ?)`,
		eventID, groupID, playerID, userID, regStatus)
	if err != nil {
		t.Fatalf("insert registration %s: %v", orderNo, err)
	}
	regID, _ := res.LastInsertId()
	var paidAt any
	if orderStatus == model.OrderStatusPaid || orderStatus == model.OrderStatusRefunding ||
		orderStatus == model.OrderStatusRefunded {
		paidAt = "2027-01-01T10:00:00+08:00"
	}
	res, err = db.SQL().ExecContext(ctx,
		`INSERT INTO orders (order_no, registration_id, user_id, amount, status, paid_at)
		 VALUES (?, ?, ?, 98000, ?, ?)`, orderNo, regID, userID, orderStatus, paidAt)
	if err != nil {
		t.Fatalf("insert order %s: %v", orderNo, err)
	}
	orderID, _ := res.LastInsertId()
	return regID, orderID
}

// TestRegistrationQueryRoutes 覆盖「我的报名」列表/详情的鉴权、分页包装、
// snake_case 键与本人隔离。
func TestRegistrationQueryRoutes(t *testing.T) {
	engine, signer, db := newRegistrationQueryTestEngine(t)
	auth := map[string]string{"Authorization": "Bearer " + adminToken(t, engine)}
	seasonID := setupSeason(t, engine, auth)
	eventID := mustCreatePublishedEvent(t, engine, auth, seasonID, "我的报名赛事")

	userA := mustInsertRouterUser(t, db, "o-router-reg-query-a")
	userB := mustInsertRouterUser(t, db, "o-router-reg-query-b")
	headersA := map[string]string{"Authorization": "Bearer " + userTokenFor(t, signer, userA)}
	headersB := map[string]string{"Authorization": "Bearer " + userTokenFor(t, signer, userB)}

	regA, _ := seedRouterRegistration(t, db, eventID, userA, "RQ000001", "ORD-Q-A",
		model.OrderStatusPaid, model.RegistrationStatusActive)
	seedRouterRegistration(t, db, eventID, userA, "RQ000002", "ORD-Q-CANCELED",
		model.OrderStatusCanceled, model.RegistrationStatusCanceled)
	seedRouterRegistration(t, db, eventID, userB, "RQ000003", "ORD-Q-B",
		model.OrderStatusPaid, model.RegistrationStatusActive)

	// 未登录 → 40101。
	w := getJSON(t, engine, "/api/v1/registrations", nil)
	assertCode(t, w, 40101)

	// 列表：只返回本人有效报名，分页包装与 snake_case。
	w = getJSON(t, engine, "/api/v1/registrations", headersA)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", w.Code, w.Body)
	}
	var listRaw struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listRaw); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listRaw.Code != 0 {
		t.Fatalf("list code = %d, body = %s", listRaw.Code, w.Body)
	}
	assertSnakeCaseKeys(t, "my registrations", listRaw.Data)
	assertObjectKeys(t, "my registrations", listRaw.Data, "list", "total", "page", "page_size")
	if listRaw.Data["total"].(float64) != 1 {
		t.Fatalf("total = %v, want 1", listRaw.Data["total"])
	}
	list, ok := listRaw.Data["list"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("list = %v, want 1 item", listRaw.Data["list"])
	}
	item, ok := list[0].(map[string]any)
	if !ok {
		t.Fatalf("item = %v", list[0])
	}
	assertSnakeCaseKeys(t, "my registration item", item)
	assertObjectKeys(t, "my registration item", item,
		"registration_id", "event_id", "event_name", "level", "event_date",
		"level_group", "dance_type", "player_name", "player_no", "order_no",
		"amount", "status", "material_status", "material_deadline")
	for _, forbidden := range []string{"user_id", "id_card_no", "id_card_hash", "phone"} {
		if _, exists := item[forbidden]; exists {
			t.Fatalf("forbidden key %q exposed: %v", forbidden, item)
		}
	}

	// 详情：本人可见，字段齐全且 snake_case。
	w = getJSON(t, engine, fmt.Sprintf("/api/v1/registrations/%d", regA), headersA)
	if w.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body = %s", w.Code, w.Body)
	}
	var detailRaw struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &detailRaw); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detailRaw.Code != 0 {
		t.Fatalf("detail code = %d, body = %s", detailRaw.Code, w.Body)
	}
	assertSnakeCaseKeys(t, "my registration detail", detailRaw.Data)
	assertObjectKeys(t, "my registration detail", detailRaw.Data,
		"status", "status_desc", "material_readonly", "event", "level_group",
		"dance_type", "player_name", "player_no", "institution", "order_no",
		"amount", "paid_at", "refund", "materials")
	event, ok := detailRaw.Data["event"].(map[string]any)
	if !ok {
		t.Fatalf("event = %v", detailRaw.Data["event"])
	}
	assertSnakeCaseKeys(t, "my registration event", event)
	assertObjectKeys(t, "my registration event", event, "id", "name", "level", "event_date")
	materials, ok := detailRaw.Data["materials"].(map[string]any)
	if !ok {
		t.Fatalf("materials = %v", detailRaw.Data["materials"])
	}
	assertSnakeCaseKeys(t, "my registration materials", materials)
	assertObjectKeys(t, "my registration materials", materials,
		"photo_url", "audio_url", "audio_filename", "program_name", "teacher_name",
		"contact_phone", "express_address", "work_intro", "special_req", "updated_at")

	// 非本人 → 40401。
	w = getJSON(t, engine, fmt.Sprintf("/api/v1/registrations/%d", regA), headersB)
	assertCode(t, w, 40401)

	// 非法 ID → 40001；不存在 → 40401。
	w = getJSON(t, engine, "/api/v1/registrations/abc", headersA)
	assertCode(t, w, 40001)
	w = getJSON(t, engine, "/api/v1/registrations/999999", headersA)
	assertCode(t, w, 40401)
}
