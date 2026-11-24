package service

import (
	"context"
	"strings"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
)

// TestWaitlistExport 覆盖候补队列导出：按筛选、脱敏证件号、中文状态与审计留痕。
func TestWaitlistExport(t *testing.T) {
	a := newAdminWaitlistFixture(t)
	ctx := context.Background()
	adminID := mustAdminID(t, a.regFixture, "admin")

	file, err := a.svc.Export(ctx, adminID, model.AdminWaitlistListQuery{
		EventID: a.eventID, Level: model.GroupLevelU11, DanceType: model.DanceTypeClassical,
	})
	if err != nil {
		t.Fatalf("export waitlists: %v", err)
	}
	rows := readExportSheetNamed(t, file.Content, waitlistExportSheetName)
	if len(rows) != 2 {
		t.Fatalf("sheet rows = %d, want 2: %v", len(rows), rows)
	}
	body := strings.Join(flattenRows(rows), "|")
	for _, want := range []string{"排队中", "张小雨", "13812345678", "U11", "古典舞"} {
		if !strings.Contains(body, want) {
			t.Fatalf("waitlist export missing %q: %s", want, body)
		}
	}
	raw := validIDForBirth("20160307")
	if strings.Contains(body, raw) {
		t.Fatalf("waitlist export must not contain raw id card: %s", body)
	}
	if n := countAdminLogAction(t, a.db, adminActionWaitlistExport); n != 1 {
		t.Fatalf("waitlist export logs = %d, want 1", n)
	}
}
