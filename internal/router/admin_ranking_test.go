package router

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

// TestRankingFlowHTTP 覆盖成绩发布 → 小程序榜 → 隐藏 → 纠错 → 晋级 → 奖金。
func TestRankingFlowHTTP(t *testing.T) {
	engine, _, db := newRefundTestEngine(t)
	adminHeaders := map[string]string{"Authorization": "Bearer " + adminToken(t, engine)}
	seasonID := setupSeason(t, engine, adminHeaders)
	eventID, groupID := mustCreateWaitlistEvent(t, engine, db, adminHeaders, seasonID, "榜单接口赛事", 20)

	// 插入 3 名已报名选手。
	ctx := context.Background()
	names := []string{"冠军甲", "亚军乙", "季军丙"}
	playerIDs := make([]int64, 0, 3)
	for i, name := range names {
		playerNo := fmt.Sprintf("Rk%06d", i)
		res, err := db.SQL().ExecContext(ctx,
			`INSERT INTO players (player_no, name, id_card_hash) VALUES (?, ?, ?)`,
			playerNo, name, "hash-"+playerNo)
		if err != nil {
			t.Fatalf("insert player: %v", err)
		}
		pid, _ := res.LastInsertId()
		playerIDs = append(playerIDs, pid)
		uid := mustInsertRouterUser(t, db, "o-rank-"+playerNo)
		if _, err := db.SQL().ExecContext(ctx,
			`INSERT INTO registrations (event_id, group_id, player_id, user_id, dance_type, institution, status)
			 VALUES (?, ?, ?, ?, 'classical', '机构R', 'active')`,
			eventID, groupID, pid, uid); err != nil {
			t.Fatalf("insert registration: %v", err)
		}
	}

	// 成绩预览 + 发布。
	rows := fmt.Sprintf(`{"rows":[
		{"row_no":1,"player_name":"冠军甲","award":"champion"},
		{"row_no":2,"player_name":"亚军乙","award":"runner_up"},
		{"row_no":3,"player_name":"季军丙","award":"third_place"}]}`)
	w := postJSON(t, engine, http.MethodPost,
		fmt.Sprintf("/api/v1/admin/events/%d/scores/preview?level=U11&dance_type=classical", eventID), rows, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", w.Code, w.Body)
	}
	preview := decodeData(t, w.Body.Bytes())
	assertSnakeCaseKeys(t, "score preview", preview)
	if preview["matched"] != float64(3) {
		t.Fatalf("preview matched=%v", preview["matched"])
	}
	publishBody := fmt.Sprintf(`{"rows":[
		{"row_no":1,"player_name":"冠军甲","award":"champion","match_status":"matched","player_id":%d},
		{"row_no":2,"player_name":"亚军乙","award":"runner_up","match_status":"matched","player_id":%d},
		{"row_no":3,"player_name":"季军丙","award":"third_place","match_status":"matched","player_id":%d}]}`,
		playerIDs[0], playerIDs[1], playerIDs[2])
	w = postJSON(t, engine, http.MethodPost,
		fmt.Sprintf("/api/v1/admin/events/%d/scores?level=U11&dance_type=classical", eventID), publishBody, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("publish status=%d body=%s", w.Code, w.Body)
	}
	if got := decodeData(t, w.Body.Bytes())["published"]; got != float64(3) {
		t.Fatalf("published=%v", got)
	}

	// 小程序榜单：3 人、冠军直通、按积分降序。
	w = postJSON(t, engine, http.MethodGet, "/api/v1/ranking?level=U11&dance_type=classical", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("public ranking status=%d body=%s", w.Code, w.Body)
	}
	pub := decodeData(t, w.Body.Bytes())
	assertSnakeCaseKeys(t, "public ranking", pub)
	ranking, _ := pub["ranking"].([]any)
	if len(ranking) != 3 {
		t.Fatalf("public ranking rows=%d", len(ranking))
	}
	first := ranking[0].(map[string]any)
	if first["rank"] != float64(1) || first["is_direct"] != true {
		t.Fatalf("public rank1 = %v", first)
	}
	direct, _ := pub["direct"].([]any)
	if len(direct) != 1 {
		t.Fatalf("direct rows=%d", len(direct))
	}
	entryID := int64(first["id"].(float64))

	// 选手详情。
	w = postJSON(t, engine, http.MethodGet, fmt.Sprintf("/api/v1/dancers/%d", entryID), "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("dancer status=%d body=%s", w.Code, w.Body)
	}
	assertSnakeCaseKeys(t, "dancer detail", decodeData(t, w.Body.Bytes()))

	// 后台榜单：隐藏冠军后不出现于小程序榜。
	hidedBody := `{"hidden":true}`
	w = postJSON(t, engine, http.MethodPut,
		fmt.Sprintf("/api/v1/admin/seasons/%d/leaderboards/U11/classical/players/%d", seasonID, playerIDs[0]),
		hidedBody, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("hide status=%d body=%s", w.Code, w.Body)
	}
	w = postJSON(t, engine, http.MethodGet, "/api/v1/ranking?level=U11&dance_type=classical", "", nil)
	if got := len(decodeData(t, w.Body.Bytes())["ranking"].([]any)); got != 2 {
		t.Fatalf("ranking after hide=%d", got)
	}

	// 纠错：冠军改亚军。
	w = postJSON(t, engine, http.MethodGet,
		fmt.Sprintf("/api/v1/admin/events/%d/scores?level=U11&dance_type=classical", eventID), "", adminHeaders)
	entries, _ := decodeData(t, w.Body.Bytes())["list"].([]any)
	if len(entries) != 3 {
		t.Fatalf("entries=%d", len(entries))
	}
	entry0 := entries[0].(map[string]any)
	w = postJSON(t, engine, http.MethodPut,
		fmt.Sprintf("/api/v1/admin/scores/%d", int64(entry0["entry_id"].(float64))),
		`{"award":"runner_up"}`, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("correct status=%d body=%s", w.Code, w.Body)
	}

	// 晋级名单与机构榜。
	w = postJSON(t, engine, http.MethodGet, fmt.Sprintf("/api/v1/admin/seasons/%d/finals-roster", seasonID), "", adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("finals roster status=%d body=%s", w.Code, w.Body)
	}
	w = postJSON(t, engine, http.MethodGet, fmt.Sprintf("/api/v1/admin/seasons/%d/institution-ranking", seasonID), "", adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("institution ranking status=%d body=%s", w.Code, w.Body)
	}

	// 纠错后奖金明细同步：冠军→亚军，甲级赛不再有奖金。
	w = postJSON(t, engine, http.MethodGet,
		fmt.Sprintf("/api/v1/admin/prize-statements?season_id=%d", seasonID), "", adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("prizes after correction status=%d body=%s", w.Code, w.Body)
	}
	if items, _ := decodeData(t, w.Body.Bytes())["list"].([]any); len(items) != 0 {
		t.Fatalf("prizes after correction=%d", len(items))
	}

	// 再纠正回冠军，奖金重新生成：甲级赛 3 人 < 40 → 1000 元 = 100000 分。
	w = postJSON(t, engine, http.MethodGet,
		fmt.Sprintf("/api/v1/admin/events/%d/scores?level=U11&dance_type=classical", eventID), "", adminHeaders)
	entries, _ = decodeData(t, w.Body.Bytes())["list"].([]any)
	if len(entries) != 3 {
		t.Fatalf("entries after first correction=%d", len(entries))
	}
	entry0 = entries[0].(map[string]any)
	w = postJSON(t, engine, http.MethodPut,
		fmt.Sprintf("/api/v1/admin/scores/%d", int64(entry0["entry_id"].(float64))),
		`{"award":"champion"}`, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("correct back status=%d body=%s", w.Code, w.Body)
	}

	w = postJSON(t, engine, http.MethodGet,
		fmt.Sprintf("/api/v1/admin/prize-statements?season_id=%d", seasonID), "", adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("prizes status=%d body=%s", w.Code, w.Body)
	}
	items, _ := decodeData(t, w.Body.Bytes())["list"].([]any)
	if len(items) != 1 {
		t.Fatalf("prize items=%d", len(items))
	}
	prize := items[0].(map[string]any)
	if prize["amount"] != float64(100000) {
		t.Fatalf("prize amount=%v", prize["amount"])
	}

	// 登记发放。
	prizeID := int64(prize["item_id"].(float64))
	w = postJSON(t, engine, http.MethodPost,
		fmt.Sprintf("/api/v1/admin/prize-statements/%d/pay", prizeID), `{"payee":"家长甲"}`, adminHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("pay prize status=%d body=%s", w.Code, w.Body)
	}
}

// TestRankingAuthHTTP 校验角色与负向路径。
func TestRankingAuthHTTP(t *testing.T) {
	engine, signer, _ := newRefundTestEngine(t)
	adminHeaders := map[string]string{"Authorization": "Bearer " + adminToken(t, engine)}
	csHeaders := map[string]string{"Authorization": "Bearer " + csAdminToken(t, signer)}
	seasonID := setupSeason(t, engine, adminHeaders)

	// 公开配置无需登录。
	if w := postJSON(t, engine, http.MethodGet, "/api/v1/config", "", nil); w.Code != http.StatusOK {
		t.Fatalf("config status=%d body=%s", w.Code, w.Body)
	}
	// 非法参数 → 40001。
	assertCode(t, postJSON(t, engine, http.MethodGet, "/api/v1/ranking?level=U99&dance_type=classical", "", nil), 40001)
	// cs 不可写榜单。
	assertCode(t, postJSON(t, engine, http.MethodPut,
		fmt.Sprintf("/api/v1/admin/seasons/%d/leaderboards/U11/classical/players/1", seasonID),
		`{"hidden":true}`, csHeaders), 40301)
	// 未登录不可读后台榜单。
	assertCode(t, postJSON(t, engine, http.MethodGet,
		fmt.Sprintf("/api/v1/admin/seasons/%d/leaderboards", seasonID), "", nil), 40101)
}
