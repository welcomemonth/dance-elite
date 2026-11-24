package service

import (
	"context"
	"strings"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
)

// TestAdminOrderExport 覆盖订单导出：筛选口径、列内容、已取消留档与审计留痕。
func TestAdminOrderExport(t *testing.T) {
	f := newAdminQueryFixture(t)
	ctx := context.Background()
	u11 := mustGroupID(t, f.db, f.eventID, model.GroupLevelU11, model.DanceTypeClassical)
	adminID := mustAdminID(t, f.regFixture, "admin")

	paid := f.seed(t, "X1", "ORD-EXP-PAID", model.OrderStatusPaid,
		model.OrderSourceMiniapp, model.DanceTypeClassical, u11, "张小雨")
	f.addPayPayment(t, paid.OrderNo, "4200000000000000001")
	canceled := f.seed(t, "X2", "ORD-EXP-CANCEL", model.OrderStatusCanceled,
		model.OrderSourceManual, model.DanceTypeClassical, u11, "李小舞")

	file, err := f.orderSvc.Export(ctx, adminID, model.AdminOrderListQuery{EventID: f.eventID})
	if err != nil {
		t.Fatalf("export orders: %v", err)
	}
	if file.Rows != 1 {
		t.Fatalf("rows = %d, want 1 (canceled excluded)", file.Rows)
	}
	rows := readExportSheetNamed(t, file.Content, orderExportSheetName)
	if len(rows) != 2 {
		t.Fatalf("sheet rows = %d, want 2: %v", len(rows), rows)
	}
	body := strings.Join(flattenRows(rows), "|")
	for _, want := range []string{paid.OrderNo, "张小雨", "已支付", "小程序", "4200000000000000001"} {
		if !strings.Contains(body, want) {
			t.Fatalf("export body missing %q: %s", want, body)
		}
	}
	assertNoIDNumber(t, body)

	if n := countAdminLogAction(t, f.db, adminActionOrderExport); n != 1 {
		t.Fatalf("order export logs = %d, want 1", n)
	}

	// include_canceled=true 导出已取消留档并标注状态。
	file, err = f.orderSvc.Export(ctx, adminID, model.AdminOrderListQuery{
		EventID: f.eventID, IncludeCanceled: true,
	})
	if err != nil {
		t.Fatalf("export orders with canceled: %v", err)
	}
	body = strings.Join(flattenRows(readExportSheetNamed(t, file.Content, orderExportSheetName)), "|")
	if !strings.Contains(body, canceled.OrderNo) || !strings.Contains(body, "已取消") {
		t.Fatalf("export should include canceled order: %s", body)
	}
}
