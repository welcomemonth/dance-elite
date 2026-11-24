package service

import (
	"bytes"
	"context"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/xuri/excelize/v2"
)

func TestPrizeExportWorkbook(t *testing.T) {
	f, scores, _, _, adminID := newScoreFixture(t)
	eventID, _ := mustScoredEvent(t, f, "PExp", 10, "奖金选手")
	ctx := context.Background()

	preview, err := scores.Preview(ctx, eventID, "U13", "classical",
		[]model.ScorePreviewInputRow{{RowNo: 1, PlayerName: "奖金选手", Award: model.AwardChampion}})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if _, err := scores.Publish(ctx, adminID, eventID, "U13", "classical", preview.Rows, ""); err != nil {
		t.Fatalf("publish: %v", err)
	}

	seasonID := mustSeasonOf(t, f, eventID)
	file, err := NewPrizeService(f.stores).Export(ctx, adminID, seasonID, eventID, "")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if file.Rows != 1 || !bytes.HasPrefix(file.Content, []byte("PK")) {
		t.Fatalf("file = %+v", file)
	}

	xf, err := excelize.OpenReader(bytes.NewReader(file.Content))
	if err != nil {
		t.Fatalf("open exported workbook: %v", err)
	}
	defer func() { _ = xf.Close() }()
	rows, err := xf.GetRows(prizeExportSheetName)
	if err != nil {
		t.Fatalf("read sheet: %v", err)
	}
	if len(rows) != 2 || rows[1][4] != "奖金选手" || rows[1][6] != "冠军" || rows[1][8] != "待发放" {
		t.Fatalf("rows = %#v", rows)
	}
}
