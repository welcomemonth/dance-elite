package router

import (
	"context"
	"net/http"
	"testing"
)

// TestAdminRegistrationWithdrawHTTP 覆盖人工退赛的鉴权、角色、幂等与名额释放。
func TestAdminRegistrationWithdrawHTTP(t *testing.T) {
	engine, signer, db := newRefundTestEngine(t)
	adminHeaders := map[string]string{"Authorization": "Bearer " + adminToken(t, engine)}
	csHeaders := map[string]string{"Authorization": "Bearer " + csAdminToken(t, signer)}
	seasonID := setupSeason(t, engine, adminHeaders)
	eventID, groupID := mustCreateWaitlistEvent(t, engine, db, adminHeaders, seasonID, "人工退赛赛事", 5)
	userID := mustInsertRouterUser(t, db, "o-withdraw")
	mustHTTPOrder(t, db, eventID, groupID, userID, "HtpWd001", "ORD-WD-1", "paid", "miniapp")

	ctx := context.Background()
	if _, err := db.SQL().ExecContext(ctx,
		`UPDATE event_groups SET taken = 1 WHERE id = ?`, groupID); err != nil {
		t.Fatalf("seed taken: %v", err)
	}
	var registrationID int64
	if err := db.SQL().QueryRowContext(ctx,
		`SELECT id FROM registrations ORDER BY id DESC LIMIT 1`).Scan(&registrationID); err != nil {
		t.Fatalf("load registration: %v", err)
	}
	path := "/api/v1/admin/registrations/" + itoa(registrationID) + "/withdraw"

	// 未登录 → 40101；客服只读 → 40301。
	assertCode(t, postJSON(t, engine, http.MethodPost, path, `{}`, nil), 40101)
	assertCode(t, postJSON(t, engine, http.MethodPost, path, `{}`, csHeaders), 40301)
	// 报名不存在 → 40401；参数非法 → 40001。
	assertCode(t, postJSON(t, engine, http.MethodPost,
		"/api/v1/admin/registrations/999999/withdraw", `{}`, adminHeaders), 40401)
	assertCode(t, postJSON(t, engine, http.MethodPost,
		"/api/v1/admin/registrations/abc/withdraw", `{}`, adminHeaders), 40001)

	// 退赛成功：报名 withdrawn、名额释放、订单仍 paid。
	w := postJSON(t, engine, http.MethodPost, path, `{"reason":"个人原因"}`, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("withdraw status = %d, body = %s", w.Code, w.Body)
	}
	data := decodeData(t, w.Body.Bytes())
	assertSnakeCaseKeys(t, "withdraw", data)
	if data["status"] != "withdrawn" || data["released"] != true {
		t.Fatalf("withdraw data = %v", data)
	}
	var regStatus, orderStatus string
	var taken int
	_ = db.SQL().QueryRowContext(ctx, `SELECT status FROM registrations WHERE id = ?`, registrationID).Scan(&regStatus)
	_ = db.SQL().QueryRowContext(ctx,
		`SELECT status FROM orders WHERE registration_id = ?`, registrationID).Scan(&orderStatus)
	_ = db.SQL().QueryRowContext(ctx, `SELECT taken FROM event_groups WHERE id = ?`, groupID).Scan(&taken)
	if regStatus != "withdrawn" || orderStatus != "paid" || taken != 0 {
		t.Fatalf("after withdraw: reg=%s order=%s taken=%d", regStatus, orderStatus, taken)
	}

	// 幂等：重复退赛不再释放名额。
	w = postJSON(t, engine, http.MethodPost, path, `{}`, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("repeat withdraw status = %d, body = %s", w.Code, w.Body)
	}
	if data := decodeData(t, w.Body.Bytes()); data["released"] != false {
		t.Fatalf("repeat withdraw released = %v, want false", data["released"])
	}
	_ = db.SQL().QueryRowContext(ctx, `SELECT taken FROM event_groups WHERE id = ?`, groupID).Scan(&taken)
	if taken != 0 {
		t.Fatalf("taken after repeat = %d, want 0", taken)
	}
}
