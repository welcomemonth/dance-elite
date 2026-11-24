package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/xuri/excelize/v2"
)

func mustScoreImportWorkbook(t *testing.T, rows [][]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	for i, row := range rows {
		cell := fmt.Sprintf("A%d", i+1)
		if err := f.SetSheetRow("Sheet1", cell, &row); err != nil {
			t.Fatalf("set row %d: %v", i+1, err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return buf.Bytes()
}

func TestParseScoreWorkbookMapsHeadersAndAwards(t *testing.T) {
	data := mustScoreImportWorkbook(t, [][]any{
		{"姓名", "证件号", "奖项", "晋级决赛", "备注"},
		{"甲选手", "110105201403070013", "冠军", "是", "备注1"},
		{"乙选手", "110105201403070021", "runner_up", "", ""},
	})
	rows, err := ParseScoreWorkbook(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].RowNo != 2 || rows[0].PlayerName != "甲选手" ||
		rows[0].Award != model.AwardChampion || !rows[0].AdvancedToFinal {
		t.Fatalf("row0 = %+v", rows[0])
	}
	if rows[1].Award != model.AwardRunnerUp || rows[1].AdvancedToFinal {
		t.Fatalf("row1 = %+v", rows[1])
	}
}

func TestParseScoreWorkbookRequiresHeaderColumns(t *testing.T) {
	data := mustScoreImportWorkbook(t, [][]any{
		{"姓名", "奖项"},
		{"甲选手", "冠军"},
	})
	if _, err := ParseScoreWorkbook(bytes.NewReader(data)); err == nil {
		t.Fatal("expected header error")
	}
}

func TestScoreTemplateWorkbookIsXLSX(t *testing.T) {
	data, err := BuildScoreTemplateWorkbook()
	if err != nil {
		t.Fatalf("template: %v", err)
	}
	if len(data) < 2 || !bytes.Equal(data[:2], []byte("PK")) {
		t.Fatalf("template is not an xlsx file")
	}
}

func TestImportPreviewCreatesIDCardPlayer(t *testing.T) {
	f, scores, _, _, adminID := newScoreFixture(t)
	eventID, _ := mustScoredEvent(t, f, "Imp", 10)
	ctx := context.Background()

	resp, err := scores.ImportPreview(ctx, adminID, eventID, "U13", "classical",
		[]model.ScorePreviewInputRow{{
			RowNo: 2, PlayerName: "导入新选手", IDCardNo: validIDForBirth("20140307"),
			Award: model.AwardChampion,
		}})
	if err != nil {
		t.Fatalf("import preview: %v", err)
	}
	if resp.Matched != 0 || resp.Created != 1 || resp.Errors != 0 || resp.ImportBatch == "" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(resp.Rows) != 1 || resp.Rows[0].MatchStatus != "created" || resp.Rows[0].PlayerID == nil {
		t.Fatalf("row = %+v", resp.Rows)
	}
	var name string
	if err := f.db.SQL().QueryRowContext(ctx,
		`SELECT name FROM players WHERE id = ?`, *resp.Rows[0].PlayerID).Scan(&name); err != nil {
		t.Fatalf("load player: %v", err)
	}
	if name != "导入新选手" {
		t.Fatalf("player name = %q", name)
	}

	if _, err := scores.Publish(ctx, adminID, eventID, "U13", "classical",
		resp.Rows, resp.ImportBatch); err != nil {
		t.Fatalf("publish created row: %v", err)
	}
}

func TestImportPreviewReportsErrorsWithoutCreatingPlayers(t *testing.T) {
	f, scores, _, _, adminID := newScoreFixture(t)
	eventID, _ := mustScoredEvent(t, f, "ImpErr", 10)
	ctx := context.Background()

	resp, err := scores.ImportPreview(ctx, adminID, eventID, "U13", "classical",
		[]model.ScorePreviewInputRow{
			{RowNo: 2, PlayerName: "", Award: model.AwardChampion},
			{RowNo: 3, PlayerName: "无奖项", IDCardNo: validIDForBirth("20140307")},
			{RowNo: 4, PlayerName: "特殊证件", IDCardNo: "P-12345", Award: model.AwardGold},
		})
	if err != nil {
		t.Fatalf("import preview: %v", err)
	}
	if resp.Errors != 3 || resp.Created != 0 || resp.Matched != 0 {
		t.Fatalf("resp = %+v", resp)
	}
	for _, row := range resp.Rows {
		if row.MatchStatus != "unmatched" || row.Error == nil || strings.Contains(*row.Error, "P-12345") {
			t.Fatalf("row = %+v", row)
		}
	}
	var n int
	if err := f.db.SQL().QueryRowContext(ctx,
		`SELECT COUNT(1) FROM players WHERE name IN ('无奖项', '特殊证件')`).Scan(&n); err != nil {
		t.Fatalf("count players: %v", err)
	}
	if n != 0 {
		t.Fatalf("players created = %d, want 0", n)
	}
}
