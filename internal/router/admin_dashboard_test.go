package router

import (
	"net/http"
	"testing"
)

// TestAdminDashboardHTTP 覆盖数据看板接口的鉴权、角色裁剪与聚合口径（API 文档 9.1）。
func TestAdminDashboardHTTP(t *testing.T) {
	engine, signer, db := newRefundTestEngine(t)
	adminHeaders := map[string]string{"Authorization": "Bearer " + adminToken(t, engine)}
	csHeaders := map[string]string{"Authorization": "Bearer " + csAdminToken(t, signer)}

	// 未登录 → 40101。
	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/admin/dashboard", "", nil), 40101)

	seasonID := setupSeason(t, engine, adminHeaders)
	eventID, groupID := mustCreateWaitlistEvent(t, engine, db, adminHeaders, seasonID, "看板赛事", 10)
	userID := mustInsertRouterUser(t, db, "o-dashboard")
	mustHTTPOrder(t, db, eventID, groupID, userID, "Dash0001", "ORD-DASH-1", "paid", "miniapp")

	// 管理员全量：指标卡、进度、趋势、截止赛事。
	w := postJSON(t, engine, http.MethodGet, "/api/v1/admin/dashboard", "", adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, body = %s", w.Code, w.Body)
	}
	data := decodeData(t, w.Body.Bytes())
	assertSnakeCaseKeys(t, "dashboard", data)
	assertObjectKeys(t, "dashboard", data, "metrics", "todos", "event_progress", "trend")

	metrics, _ := data["metrics"].(map[string]any)
	assertObjectKeys(t, "metrics", metrics,
		"today_registrations", "today_revenue", "total_registrations", "total_revenue",
		"pending_refund_tickets", "waitlist_total", "incomplete_materials", "active_events")
	if metrics["total_registrations"] != float64(1) {
		t.Fatalf("total_registrations = %v, want 1", metrics["total_registrations"])
	}
	if metrics["total_revenue"] != float64(98000) {
		t.Fatalf("total_revenue = %v, want 98000", metrics["total_revenue"])
	}
	if metrics["active_events"] != float64(1) {
		t.Fatalf("active_events = %v, want 1", metrics["active_events"])
	}

	todos, _ := data["todos"].(map[string]any)
	closing, _ := todos["closing_events"].([]any)
	if len(closing) != 1 {
		t.Fatalf("closing_events = %v, want 1 event within 7 days", closing)
	}
	closingEvent, _ := closing[0].(map[string]any)
	assertObjectKeys(t, "closing event", closingEvent, "event_id", "event_name", "days_left")
	if closingEvent["days_left"] != float64(1) {
		t.Fatalf("days_left = %v, want 1", closingEvent["days_left"])
	}

	progress, _ := data["event_progress"].([]any)
	if len(progress) != 1 {
		t.Fatalf("event_progress = %v, want 1 group", progress)
	}
	row, _ := progress[0].(map[string]any)
	assertObjectKeys(t, "event progress", row,
		"event_id", "event_name", "level", "dance_type", "registered", "quota", "waitlist_count")
	if row["registered"] != float64(10) || row["quota"] != float64(10) {
		t.Fatalf("progress registered/quota = %v/%v, want 10/10", row["registered"], row["quota"])
	}

	trend, _ := data["trend"].([]any)
	if len(trend) != 30 {
		t.Fatalf("trend len = %d, want 30", len(trend))
	}

	// 客服裁剪：仅待办工单相关字段有值。
	w = postJSON(t, engine, http.MethodGet, "/api/v1/admin/dashboard", "", csHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("cs dashboard status = %d, body = %s", w.Code, w.Body)
	}
	csData := decodeData(t, w.Body.Bytes())
	csMetrics, _ := csData["metrics"].(map[string]any)
	if csMetrics["total_registrations"] != float64(0) || csMetrics["total_revenue"] != float64(0) {
		t.Fatalf("cs metrics should be trimmed, got %v", csMetrics)
	}
	if progress, ok := csData["event_progress"].([]any); ok && len(progress) != 0 {
		t.Fatalf("cs event_progress should be empty, got %v", progress)
	}
	if _, ok := csData["todos"].(map[string]any); !ok {
		t.Fatalf("cs todos missing: %v", csData)
	}
}
