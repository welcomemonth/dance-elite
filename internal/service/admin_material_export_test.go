package service

import (
	"context"
	"strings"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
)

// TestMaterialCollectionExport 覆盖催收名单导出：缺失项中文、联系状态与审计留痕。
func TestMaterialCollectionExport(t *testing.T) {
	f := newAdminQueryFixture(t)
	ctx := context.Background()
	u11 := mustGroupID(t, f.db, f.eventID, model.GroupLevelU11, model.DanceTypeClassical)
	adminID := mustAdminID(t, f.regFixture, "admin")

	missing := f.seed(t, "M1", "ORD-EXP-MAT", model.OrderStatusPaid,
		model.OrderSourceMiniapp, model.DanceTypeClassical, u11, "缺音频选手")
	f.fillMaterials(t, missing.RegistrationID,
		withoutField(allMaterialFields, model.MaterialFieldAudio)...)
	if _, err := f.collection.MarkContacted(ctx, adminID, missing.RegistrationID, "已电话提醒"); err != nil {
		t.Fatalf("mark contacted: %v", err)
	}

	file, err := f.collection.Export(ctx, adminID, model.AdminMissingMaterialQuery{EventID: f.eventID})
	if err != nil {
		t.Fatalf("export missing materials: %v", err)
	}
	if file.Rows != 1 {
		t.Fatalf("rows = %d, want 1", file.Rows)
	}
	rows := readExportSheetNamed(t, file.Content, materialExportSheetName)
	if len(rows) != 2 {
		t.Fatalf("sheet rows = %d, want 2: %v", len(rows), rows)
	}
	body := strings.Join(flattenRows(rows), "|")
	for _, want := range []string{"缺音频选手", "舞蹈音频", "已联系", "7/8"} {
		if !strings.Contains(body, want) {
			t.Fatalf("material export missing %q: %s", want, body)
		}
	}
	if n := countAdminLogAction(t, f.db, adminActionMaterialExport); n != 1 {
		t.Fatalf("material export logs = %d, want 1", n)
	}
}
