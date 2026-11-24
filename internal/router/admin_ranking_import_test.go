package router

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/xuri/excelize/v2"
)

func buildScoreImportXLSX(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	rows := [][]any{
		{"姓名", "证件号", "奖项", "晋级决赛", "备注"},
		{"导入选手", routerValidIDCard, "冠军", "是", "接口测试"},
	}
	for i, row := range rows {
		if err := f.SetSheetRow("Sheet1", fmt.Sprintf("A%d", i+1), &row); err != nil {
			t.Fatalf("set row %d: %v", i+1, err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return buf.Bytes()
}

func TestScoreImportAndTemplateHTTP(t *testing.T) {
	engine, _, db := newRefundTestEngine(t)
	auth := map[string]string{"Authorization": "Bearer " + adminToken(t, engine)}
	seasonID := setupSeason(t, engine, auth)
	eventID, _ := mustCreateWaitlistEvent(t, engine, db, auth, seasonID, "成绩导入赛事", 20)

	w := postMultipart(t, engine,
		fmt.Sprintf("/api/v1/admin/events/%d/scores/import?level=U11&dance_type=classical", eventID),
		nil, map[string]uploadFile{"file": {name: "scores.xlsx", data: buildScoreImportXLSX(t)}}, auth)
	if w.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", w.Code, w.Body)
	}
	data := decodeData(t, w.Body.Bytes())
	assertSnakeCaseKeys(t, "score import", data)
	if data["created"] != float64(1) || data["errors"] != float64(0) {
		t.Fatalf("import data = %v", data)
	}
	rows, _ := data["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("rows = %v", rows)
	}
	row := rows[0].(map[string]any)
	if row["match_status"] != "created" || row["player_id"] == nil {
		t.Fatalf("row = %v", row)
	}

	w = postJSON(t, engine, http.MethodGet,
		fmt.Sprintf("/api/v1/admin/events/%d/scores/template", eventID), "", auth)
	if w.Code != http.StatusOK {
		t.Fatalf("template status=%d body=%s", w.Code, w.Body)
	}
	if ct := w.Header().Get("Content-Type"); ct != exportMIME {
		t.Fatalf("template content-type = %q", ct)
	}
}
