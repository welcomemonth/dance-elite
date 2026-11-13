package router

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/token"
)

// opsAdminToken 按普通 admin 角色签发后台 token（权限核对用）。
// 使用不存在的 admin_id，避免与用例中创建/停用的账号 ID 冲突（吊销按 ID 生效）。
func opsAdminToken(t *testing.T, signer *token.Signer) string {
	t.Helper()
	raw, err := signer.IssueAdmin(900, model.RoleAdmin)
	if err != nil {
		t.Fatalf("issue ops admin token: %v", err)
	}
	return raw
}

// TestAdminAccountsHTTP 覆盖账号管理的权限矩阵、校验与停用踢下线（API 文档 14.1）。
func TestAdminAccountsHTTP(t *testing.T) {
	engine, signer, _ := newRefundTestEngine(t)
	adminHeaders := map[string]string{"Authorization": "Bearer " + adminToken(t, engine)}
	opsHeaders := map[string]string{"Authorization": "Bearer " + opsAdminToken(t, signer)}
	csHeaders := map[string]string{"Authorization": "Bearer " + csAdminToken(t, signer)}

	// 权限矩阵：accounts 全组 super_admin 专属，admin/cs → 40301。
	for _, headers := range []map[string]string{opsHeaders, csHeaders} {
		assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/admin/accounts", "", headers), 40301)
		assertCode(t, postJSON(t, engine, http.MethodPost, "/api/v1/admin/accounts",
			`{"username":"x1","password":"Passw0rd!","role":"cs","name":"x"}`, headers), 40301)
	}

	// 未登录 → 40101。
	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/admin/accounts", "", nil), 40101)

	// 创建成功：字段收敛为 admin_id/username/role。
	w := postJSON(t, engine, http.MethodPost, "/api/v1/admin/accounts",
		`{"username":"cs01","password":"Cs@12345","role":"cs","name":"客服小李"}`, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("create account status = %d, body = %s", w.Code, w.Body)
	}
	created := decodeData(t, w.Body.Bytes())
	assertObjectKeys(t, "created account", created, "admin_id", "username", "role")
	if created["username"] != "cs01" || created["role"] != "cs" {
		t.Fatalf("created = %v", created)
	}
	adminID := int64(created["admin_id"].(float64))

	// 用户名重复 / 弱密码 / 缺字段 → 40001。
	assertCode(t, postJSON(t, engine, http.MethodPost, "/api/v1/admin/accounts",
		`{"username":"cs01","password":"Cs@12345","role":"cs","name":"客服小李"}`, adminHeaders), 40001)
	assertCode(t, postJSON(t, engine, http.MethodPost, "/api/v1/admin/accounts",
		`{"username":"cs02","password":"short","role":"cs","name":"客服"}`, adminHeaders), 40001)
	assertCode(t, postJSON(t, engine, http.MethodPost, "/api/v1/admin/accounts",
		`{"username":"cs03","password":"Cs@12345","role":"root","name":"客服"}`, adminHeaders), 40001)

	// 列表包含新账号，且不带密码哈希字段。
	w = postJSON(t, engine, http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=20", "", adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("list accounts status = %d, body = %s", w.Code, w.Body)
	}
	listData := decodeData(t, w.Body.Bytes())
	if listData["total"] != float64(2) {
		t.Fatalf("accounts total = %v, want 2", listData["total"])
	}
	items, _ := listData["list"].([]any)
	item, _ := items[1].(map[string]any)
	assertObjectKeys(t, "account item", item,
		"admin_id", "username", "name", "role", "status", "last_login_at", "created_at")
	if item["status"] != "active" || item["name"] != "客服小李" {
		t.Fatalf("account item = %v", item)
	}

	// 停用 → 该账号 token 立即失效（踢下线）；不能停用自己 → 40901。
	csTokenRaw, err := signer.IssueAdmin(adminID, model.RoleCS)
	if err != nil {
		t.Fatalf("issue token for cs01: %v", err)
	}
	cs01Headers := map[string]string{"Authorization": "Bearer " + csTokenRaw}
	assertCode(t, postJSON(t, engine, http.MethodPut, "/api/v1/admin/accounts/1",
		`{"status":"disabled"}`, adminHeaders), 40901)

	w = postJSON(t, engine, http.MethodPut, fmt.Sprintf("/api/v1/admin/accounts/%d", adminID),
		`{"status":"disabled"}`, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("disable status = %d, body = %s", w.Code, w.Body)
	}
	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/admin/accounts", "", cs01Headers), 40101)

	// 重新激活 → 可再次登录访问（新 token 签发不受旧吊销影响）。
	w = postJSON(t, engine, http.MethodPut, fmt.Sprintf("/api/v1/admin/accounts/%d", adminID),
		`{"status":"active"}`, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("reactivate status = %d, body = %s", w.Code, w.Body)
	}
	csTokenRaw2, err := signer.IssueAdmin(adminID, model.RoleCS)
	if err != nil {
		t.Fatalf("reissue token for cs01: %v", err)
	}
	w = postJSON(t, engine, http.MethodGet, "/api/v1/admin/accounts", "",
		map[string]string{"Authorization": "Bearer " + csTokenRaw2})
	if w.Code != http.StatusForbidden {
		t.Fatalf("cs accounts after reactivate = %d, want 403", w.Code)
	}

	// 重置密码 + 调整角色。
	w = postJSON(t, engine, http.MethodPut, fmt.Sprintf("/api/v1/admin/accounts/%d", adminID),
		`{"password":"New@12345","role":"admin"}`, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("reset password status = %d, body = %s", w.Code, w.Body)
	}

	// 不存在账号 → 40401；空 body → 40001。
	assertCode(t, postJSON(t, engine, http.MethodPut, "/api/v1/admin/accounts/999999",
		`{"status":"disabled"}`, adminHeaders), 40401)
	assertCode(t, postJSON(t, engine, http.MethodPut, fmt.Sprintf("/api/v1/admin/accounts/%d", adminID),
		`{}`, adminHeaders), 40001)
}

// TestAdminLogsHTTP 覆盖操作日志的权限、筛选与字段（API 文档 14.2）。
func TestAdminLogsHTTP(t *testing.T) {
	engine, signer, _ := newRefundTestEngine(t)
	adminHeaders := map[string]string{"Authorization": "Bearer " + adminToken(t, engine)}
	opsHeaders := map[string]string{"Authorization": "Bearer " + opsAdminToken(t, signer)}
	csHeaders := map[string]string{"Authorization": "Bearer " + csAdminToken(t, signer)}

	// 先产生两条日志：创建账号 + 更新账号。
	w := postJSON(t, engine, http.MethodPost, "/api/v1/admin/accounts",
		`{"username":"cs01","password":"Cs@12345","role":"cs","name":"客服小李"}`, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("create account: %s", w.Body)
	}
	adminID := int64(decodeData(t, w.Body.Bytes())["admin_id"].(float64))
	w = postJSON(t, engine, http.MethodPut, fmt.Sprintf("/api/v1/admin/accounts/%d", adminID),
		`{"status":"disabled"}`, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("disable account: %s", w.Body)
	}

	// 权限：super_admin / admin 可查，cs → 40301，未登录 → 40101。
	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/admin/logs", "", nil), 40101)
	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/admin/logs", "", csHeaders), 40301)
	w = postJSON(t, engine, http.MethodGet, "/api/v1/admin/logs", "", opsHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("ops logs status = %d, body = %s", w.Code, w.Body)
	}

	// 全量列表：倒序、字段收敛（detail 为对象）。
	// 共 3 条：adminToken 登录留痕 + 创建账号 + 更新账号（API 文档 14.2：登录留痕）。
	w = postJSON(t, engine, http.MethodGet, "/api/v1/admin/logs?page=1&page_size=20", "", adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("logs status = %d, body = %s", w.Code, w.Body)
	}
	data := decodeData(t, w.Body.Bytes())
	if data["total"] != float64(3) {
		t.Fatalf("logs total = %v, want 3", data["total"])
	}
	items, _ := data["list"].([]any)
	entry, _ := items[0].(map[string]any)
	assertObjectKeys(t, "log entry", entry,
		"log_id", "admin_name", "action", "target_type", "target_id", "detail", "created_at")
	if entry["action"] != "account_update" {
		t.Fatalf("latest log action = %v, want account_update", entry["action"])
	}
	detail, _ := entry["detail"].(map[string]any)
	if detail["after_status"] != "disabled" {
		t.Fatalf("log detail = %v", detail)
	}
	if entry["admin_name"] == "" {
		t.Fatal("log admin_name is empty")
	}

	// action 筛选命中 1 条。
	w = postJSON(t, engine, http.MethodGet, "/api/v1/admin/logs?action=account_create", "", adminHeaders)
	if total := decodeData(t, w.Body.Bytes())["total"]; total != float64(1) {
		t.Fatalf("account_create logs total = %v, want 1", total)
	}

	// admin_id 筛选：不存在的人 → 0 条；非法 id → 40001。
	w = postJSON(t, engine, http.MethodGet, "/api/v1/admin/logs?admin_id=999999", "", adminHeaders)
	if total := decodeData(t, w.Body.Bytes())["total"]; total != float64(0) {
		t.Fatalf("unknown admin logs total = %v, want 0", total)
	}
	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/admin/logs?admin_id=abc", "", adminHeaders), 40001)
}
