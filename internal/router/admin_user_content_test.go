package router

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/meetmonth/dance-elite/internal/store"
)

// TestAdminUsersHTTP 覆盖微信用户列表的筛选、脱敏与字段（API 文档 13.1）。
func TestAdminUsersHTTP(t *testing.T) {
	engine, signer, db := newRefundTestEngine(t)
	adminHeaders := map[string]string{"Authorization": "Bearer " + adminToken(t, engine)}
	csHeaders := map[string]string{"Authorization": "Bearer " + csAdminToken(t, signer)}

	userID := mustInsertRouterUserWithPhone(t, db, "o-admin-user", "13812345678", "小雨妈妈")
	eventID, groupID := mustCreateWaitlistEvent(t, engine, db, adminHeaders,
		setupSeason(t, engine, adminHeaders), "用户列表赛事", 10)
	mustHTTPOrder(t, db, eventID, groupID, userID, "User0001", "ORD-USER-1", "paid", "miniapp")

	// cs 只读可用；未登录 → 40101。
	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/admin/users", "", nil), 40101)
	w := postJSON(t, engine, http.MethodGet, "/api/v1/admin/users", "", csHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("cs users status = %d, body = %s", w.Code, w.Body)
	}

	// 字段收敛：手机号脱敏、报名次数、状态。
	w = postJSON(t, engine, http.MethodGet, "/api/v1/admin/users?page=1&page_size=20", "", adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("users status = %d, body = %s", w.Code, w.Body)
	}
	data := decodeData(t, w.Body.Bytes())
	if data["total"] != float64(1) {
		t.Fatalf("users total = %v, want 1", data["total"])
	}
	items, _ := data["list"].([]any)
	item, _ := items[0].(map[string]any)
	assertObjectKeys(t, "wx user", item,
		"id", "nickname", "avatar_url", "phone_masked", "registration_count", "status",
		"created_at", "last_login_at")
	if item["phone_masked"] != "138****5678" {
		t.Fatalf("phone_masked = %v, want masked", item["phone_masked"])
	}
	if item["registration_count"] != float64(1) {
		t.Fatalf("registration_count = %v, want 1", item["registration_count"])
	}

	// 关键字与状态筛选。
	w = postJSON(t, engine, http.MethodGet, "/api/v1/admin/users?keyword=1381234", "", adminHeaders)
	if total := decodeData(t, w.Body.Bytes())["total"]; total != float64(1) {
		t.Fatalf("keyword total = %v, want 1", total)
	}
	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/admin/users?status=locked", "", adminHeaders), 40001)
}

// TestAdminPlayersHTTP 覆盖选手档案列表/详情与内容管理（API 文档 13.2/13.4）。
func TestAdminPlayersHTTP(t *testing.T) {
	engine, signer, db := newRefundTestEngine(t)
	adminHeaders := map[string]string{"Authorization": "Bearer " + adminToken(t, engine)}
	csHeaders := map[string]string{"Authorization": "Bearer " + csAdminToken(t, signer)}
	seasonID := setupSeason(t, engine, adminHeaders)
	eventID, groupID := mustCreateWaitlistEvent(t, engine, db, adminHeaders, seasonID, "选手档案赛事", 10)

	// 造第二个档案（无证件号哈希；players.id_card_hash 有 UNIQUE 约束，
	// 同哈希档案在库中不可能存在，重复档案场景仅出现在无证件号登记）。
	userID := mustInsertRouterUser(t, db, "o-admin-player")
	mustHTTPOrder(t, db, eventID, groupID, userID, "Plyr0001", "ORD-PLY-1", "paid", "miniapp")
	if _, err := db.SQL().ExecContext(t.Context(),
		`INSERT INTO players (player_no, name, id_card_hash, id_type, birth_date, gender)
		 VALUES ('Plyr0002', '张雨涵', NULL, 'id_card', '2015-03-02', 'female')`); err != nil {
		t.Fatalf("insert second player: %v", err)
	}

	// 列表：cs 可读，字段收敛，不含证件号明文/哈希。
	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/admin/players", "", nil), 40101)
	w := postJSON(t, engine, http.MethodGet, "/api/v1/admin/players", "", csHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("cs players status = %d, body = %s", w.Code, w.Body)
	}
	data := decodeData(t, w.Body.Bytes())
	if data["total"] != float64(2) {
		t.Fatalf("players total = %v, want 2", data["total"])
	}
	items, _ := data["list"].([]any)
	var target map[string]any
	for _, raw := range items {
		candidate, _ := raw.(map[string]any)
		assertObjectKeys(t, "player brief", candidate,
			"player_id", "player_no", "name", "id_type", "id_card_masked", "birth_date", "gender",
			"institution", "merged_into", "registration_count", "result_count")
		if _, leaked := candidate["id_card_hash"]; leaked {
			t.Fatal("player list leaks id_card_hash")
		}
		if candidate["player_no"] == "Plyr0001" {
			target = candidate
		}
	}
	if target == nil {
		t.Fatal("Plyr0001 not found in player list")
	}
	if target["registration_count"] != float64(1) {
		t.Fatalf("registration_count = %v, want 1", target["registration_count"])
	}
	playerID := int64(target["player_id"].(float64))

	// 详情：历史报名 + 重复档案提示。
	w = postJSON(t, engine, http.MethodGet, fmt.Sprintf("/api/v1/admin/players/%d", playerID), "", adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("player detail status = %d, body = %s", w.Code, w.Body)
	}
	detail := decodeData(t, w.Body.Bytes())
	assertObjectKeys(t, "player detail", detail,
		"player_id", "player_no", "name", "id_type", "id_card_masked", "birth_date", "gender",
		"institution", "merged_into", "registration_count", "result_count",
		"registrations", "results_summary", "points_summary", "duplicates")
	regs, _ := detail["registrations"].([]any)
	if len(regs) != 1 {
		t.Fatalf("registrations = %v, want 1", regs)
	}
	reg, _ := regs[0].(map[string]any)
	assertObjectKeys(t, "player registration", reg,
		"order_no", "event_name", "level", "dance_type", "status", "created_at")
	if reg["order_no"] != "ORD-PLY-1" {
		t.Fatalf("reg order_no = %v", reg["order_no"])
	}
	dups, _ := detail["duplicates"].([]any)
	if len(dups) != 0 {
		t.Fatalf("duplicates = %v, want empty (UNIQUE hash makes same-hash duplicates impossible)", dups)
	}
	points, _ := detail["points_summary"].(map[string]any)
	assertObjectKeys(t, "points summary", points, "season_name", "total_points", "event_count")

	// 不存在 → 40401；非法 ID → 40001。
	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/admin/players/999999", "", adminHeaders), 40401)
	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/admin/players/abc", "", adminHeaders), 40001)

	// 内容管理：GET cs 可读；PUT admin 可写、cs → 40301；未注册 key → 40401。
	w = postJSON(t, engine, http.MethodGet, "/api/v1/admin/contents/announcement", "", csHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("get content status = %d, body = %s", w.Code, w.Body)
	}
	content := decodeData(t, w.Body.Bytes())
	assertObjectKeys(t, "content entry", content, "key", "value", "updated_at")
	if content["key"] != "announcement" {
		t.Fatalf("content key = %v", content["key"])
	}
	assertCode(t, postJSON(t, engine, http.MethodPut, "/api/v1/admin/contents/announcement",
		`{"value":{"text":"1 月 20 日抽签"}}`, csHeaders), 40301)
	w = postJSON(t, engine, http.MethodPut, "/api/v1/admin/contents/announcement",
		`{"value":{"text":"1 月 20 日抽签","online_at":"2027-01-15T00:00:00+08:00"}}`, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("put content status = %d, body = %s", w.Code, w.Body)
	}
	updated := decodeData(t, w.Body.Bytes())
	value, _ := updated["value"].(map[string]any)
	if value["text"] != "1 月 20 日抽签" {
		t.Fatalf("content value = %v", value)
	}

	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/admin/contents/not_a_key", "", adminHeaders), 40401)
	assertCode(t, postJSON(t, engine, http.MethodPut, "/api/v1/admin/contents/not_a_key",
		`{"value":{"text":"x"}}`, adminHeaders), 40401)

	// 内容修改留痕（content_update）。
	w = postJSON(t, engine, http.MethodGet, "/api/v1/admin/logs?action=content_update", "", adminHeaders)
	if total := decodeData(t, w.Body.Bytes())["total"]; total != float64(1) {
		t.Fatalf("content_update logs total = %v, want 1", total)
	}
}

// mustInsertRouterUserWithPhone 插入带昵称/手机号的微信用户，返回用户 ID。
func mustInsertRouterUserWithPhone(t *testing.T, db *store.DB, openid, phone, nickname string) int64 {
	t.Helper()
	res, err := db.SQL().ExecContext(t.Context(),
		`INSERT INTO users (openid, nickname, phone, last_login_at) VALUES (?, ?, ?, ?)`,
		openid, nickname, phone, "2027-01-02T10:00:00+08:00")
	if err != nil {
		t.Fatalf("insert user %s: %v", openid, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// TestPublicContentsHTTP 覆盖小程序公开内容读取（S5-4）：免登录、统一包装、未注册 key 40401。
func TestPublicContentsHTTP(t *testing.T) {
	engine, _, _ := newRefundTestEngine(t)

	w := postJSON(t, engine, http.MethodGet, "/api/v1/contents/announcement", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("public contents status = %d, body = %s", w.Code, w.Body)
	}
	data := decodeData(t, w.Body.Bytes())
	if data["key"] != "announcement" {
		t.Fatalf("key = %v, want announcement", data["key"])
	}
	if _, ok := data["value"].(map[string]any); !ok {
		t.Fatalf("value should be an object: %v", data["value"])
	}

	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/contents/not_a_key", "", nil), 40401)
}
