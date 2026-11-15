package service

import (
	"context"
	"strings"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
)

// TestRegistrationQueryListMine 覆盖「我的报名」列表的可见范围、状态映射、
// 资料完成度、分页与本人隔离。
func TestRegistrationQueryListMine(t *testing.T) {
	f := newRegQueryFixture(t)
	ctx := context.Background()
	userA := mustInsertRegUser(t, f.db, "o-reg-list-a")
	userB := mustInsertRegUser(t, f.db, "o-reg-list-b")
	classical := mustGroupID(t, f.db, f.eventID, model.GroupLevelU11, model.DanceTypeClassical)
	folk := mustGroupID(t, f.db, f.eventID, model.GroupLevelU11, model.DanceTypeFolk)

	paid := f.seed(t, userA, "ORD-PAID", model.OrderStatusPaid,
		model.RegistrationStatusActive, model.DanceTypeClassical, classical, "张小雨")
	f.fillMaterials(t, paid.RegistrationID,
		model.MaterialFieldPhoto, model.MaterialFieldProgramName, model.MaterialFieldContactPhone)
	refunding := f.seed(t, userA, "ORD-REFUNDING", model.OrderStatusRefunding,
		model.RegistrationStatusActive, model.DanceTypeFolk, folk, "张小雨")
	refunded := f.seed(t, userA, "ORD-REFUNDED", model.OrderStatusRefunded,
		model.RegistrationStatusWithdrawn, model.DanceTypeClassical, classical, "张小雨")
	// 已取消 / 待支付 / 他人报名都不应出现。
	f.seed(t, userA, "ORD-CANCELED", model.OrderStatusCanceled,
		model.RegistrationStatusCanceled, model.DanceTypeClassical, classical, "张小雨")
	f.seed(t, userA, "ORD-PENDING", model.OrderStatusPendingPayment,
		model.RegistrationStatusPendingPayment, model.DanceTypeClassical, classical, "张小雨")
	f.seed(t, userB, "ORD-OTHER", model.OrderStatusPaid,
		model.RegistrationStatusActive, model.DanceTypeClassical, classical, "别人")

	resp, err := f.svc.List(ctx, userA, 1, 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.Total != 3 || len(resp.List) != 3 {
		t.Fatalf("list = total %d, len %d, want 3", resp.Total, len(resp.List))
	}
	if resp.List == nil {
		t.Fatal("list must not be nil")
	}

	statusByID := make(map[int64]string, len(resp.List))
	for _, item := range resp.List {
		statusByID[item.RegistrationID] = item.Status
	}
	for id, want := range map[int64]string{
		paid.RegistrationID:      model.MiniappRegistrationStatusPaid,
		refunding.RegistrationID: model.MiniappRegistrationStatusRefunding,
		refunded.RegistrationID:  model.MiniappRegistrationStatusRefunded,
	} {
		if statusByID[id] != want {
			t.Fatalf("registration %d status = %q, want %q", id, statusByID[id], want)
		}
	}

	for _, item := range resp.List {
		if item.RegistrationID != paid.RegistrationID {
			continue
		}
		if item.EventID != f.eventID || item.EventName != "我的报名赛事" ||
			item.Level != model.EventLevelGradeA || item.LevelGroup != model.GroupLevelU11 ||
			item.DanceType != model.DanceTypeClassical || item.PlayerName != "张小雨" ||
			item.PlayerNo != paid.PlayerNo || item.OrderNo != "ORD-PAID" || item.Amount != 98000 {
			t.Fatalf("paid item = %+v", item)
		}
		if item.MaterialStatus.Total != 8 || item.MaterialStatus.Completed != 3 {
			t.Fatalf("material_status = %+v, want 3/8", item.MaterialStatus)
		}
		if item.MaterialDeadline == nil || *item.MaterialDeadline == "" {
			t.Fatal("material_deadline should not be empty")
		}
	}

	page, err := f.svc.List(ctx, userA, 2, 2)
	if err != nil {
		t.Fatalf("page list: %v", err)
	}
	if page.Total != 3 || len(page.List) != 1 {
		t.Fatalf("page result = total %d, len %d, want total 3, len 1", page.Total, len(page.List))
	}

	emptyUser := mustInsertRegUser(t, f.db, "o-reg-list-empty")
	empty, err := f.svc.List(ctx, emptyUser, 1, 20)
	if err != nil {
		t.Fatalf("empty list: %v", err)
	}
	if empty.Total != 0 || empty.List == nil || len(empty.List) != 0 {
		t.Fatalf("empty list = %+v, want empty non-nil list", empty)
	}
}

// TestRegistrationQueryDetail 覆盖报名详情的状态/文案/资料只读/退款信息与
// 非本人隔离。
func TestRegistrationQueryDetail(t *testing.T) {
	f := newRegQueryFixture(t)
	ctx := context.Background()
	user := mustInsertRegUser(t, f.db, "o-reg-detail")
	otherUser := mustInsertRegUser(t, f.db, "o-reg-detail-other")
	group := mustGroupID(t, f.db, f.eventID, model.GroupLevelU11, model.DanceTypeClassical)
	folkGroup := mustGroupID(t, f.db, f.eventID, model.GroupLevelU11, model.DanceTypeFolk)

	paid := f.seed(t, user, "ORD-DETAIL-PAID", model.OrderStatusPaid,
		model.RegistrationStatusActive, model.DanceTypeClassical, group, "张小雨")
	f.fillMaterials(t, paid.RegistrationID,
		withoutRegQueryField(regQueryAllMaterialFields, model.MaterialFieldAudio)...)

	detail, err := f.svc.Detail(ctx, user, paid.RegistrationID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.Status != model.MiniappRegistrationStatusPaid {
		t.Fatalf("status = %q, want paid", detail.Status)
	}
	if detail.MaterialReadonly {
		t.Fatal("material should be editable before material_check_at")
	}
	if !strings.Contains(detail.StatusDesc, "上传资料") {
		t.Fatalf("status_desc = %q, want upload hint", detail.StatusDesc)
	}
	if detail.Event.ID != f.eventID || detail.Event.Name != "我的报名赛事" ||
		detail.Event.Level != model.EventLevelGradeA || detail.Event.EventDate == nil {
		t.Fatalf("event = %+v", detail.Event)
	}
	if detail.LevelGroup != model.GroupLevelU11 || detail.DanceType != model.DanceTypeClassical ||
		detail.PlayerName != "张小雨" || detail.PlayerNo != paid.PlayerNo ||
		detail.Institution == nil || *detail.Institution != "上海灵舞艺术中心" ||
		detail.OrderNo != "ORD-DETAIL-PAID" || detail.Amount != 98000 || detail.PaidAt == nil {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.Refund != nil {
		t.Fatalf("refund = %+v, want nil for paid registration", detail.Refund)
	}
	if detail.Materials.PhotoURL == nil || detail.Materials.AudioURL != nil ||
		detail.Materials.AudioFilename != nil || detail.Materials.ProgramName == nil {
		t.Fatalf("materials = %+v", detail.Materials)
	}

	// 资料截止后只读，状态文案切换为已截止。
	f.setTimelinePast(t)
	detail, err = f.svc.Detail(ctx, user, paid.RegistrationID)
	if err != nil {
		t.Fatalf("detail after deadline: %v", err)
	}
	if !detail.MaterialReadonly {
		t.Fatal("material should be readonly after material_check_at")
	}
	if !strings.Contains(detail.StatusDesc, "资料上传已截止") {
		t.Fatalf("status_desc = %q, want deadline hint", detail.StatusDesc)
	}

	// 资料齐全后文案切换为已提交。
	f.fillMaterials(t, paid.RegistrationID, regQueryAllMaterialFields...)
	detail, err = f.svc.Detail(ctx, user, paid.RegistrationID)
	if err != nil {
		t.Fatalf("detail after complete materials: %v", err)
	}
	if !strings.Contains(detail.StatusDesc, "资料已提交") {
		t.Fatalf("status_desc = %q, want complete hint", detail.StatusDesc)
	}

	// 退款中：展示 processing 与工单号/原因。
	refunding := f.seed(t, user, "ORD-DETAIL-REFUNDING", model.OrderStatusRefunding,
		model.RegistrationStatusActive, model.DanceTypeFolk, folkGroup, "张小雨")
	f.addRefundTicket(t, refunding.OrderID, "RF-DETAIL-1", model.RefundTicketStatusApproved, "个人原因")
	detail, err = f.svc.Detail(ctx, user, refunding.RegistrationID)
	if err != nil {
		t.Fatalf("refunding detail: %v", err)
	}
	if detail.Status != model.MiniappRegistrationStatusRefunding ||
		detail.Refund == nil || detail.Refund.Status != "processing" ||
		detail.Refund.TicketNo != "RF-DETAIL-1" || detail.Refund.Reason != "个人原因" {
		t.Fatalf("refunding detail = %+v, refund = %+v", detail, detail.Refund)
	}
	if !strings.Contains(detail.StatusDesc, "退款处理中") {
		t.Fatalf("status_desc = %q, want refunding hint", detail.StatusDesc)
	}

	// 已退款：展示 refunded。
	refunded := f.seed(t, user, "ORD-DETAIL-REFUNDED", model.OrderStatusRefunded,
		model.RegistrationStatusWithdrawn, model.DanceTypeClassical, group, "张小雨")
	f.addRefundTicket(t, refunded.OrderID, "RF-DETAIL-2", model.RefundTicketStatusRefunded, "个人原因")
	detail, err = f.svc.Detail(ctx, user, refunded.RegistrationID)
	if err != nil {
		t.Fatalf("refunded detail: %v", err)
	}
	if detail.Status != model.MiniappRegistrationStatusRefunded ||
		detail.Refund == nil || detail.Refund.Status != model.MiniappRegistrationStatusRefunded {
		t.Fatalf("refunded detail = %+v, refund = %+v", detail, detail.Refund)
	}

	// 非本人 / 不存在 / 已取消 / 待支付统一 40401。
	canceled := f.seed(t, user, "ORD-DETAIL-CANCELED", model.OrderStatusCanceled,
		model.RegistrationStatusCanceled, model.DanceTypeClassical, group, "张小雨")
	pending := f.seed(t, user, "ORD-DETAIL-PENDING", model.OrderStatusPendingPayment,
		model.RegistrationStatusPendingPayment, model.DanceTypeClassical, group, "张小雨")
	cases := []struct {
		name   string
		userID int64
		regID  int64
	}{
		{"非本人", otherUser, paid.RegistrationID},
		{"不存在", user, 999999},
		{"已取消", user, canceled.RegistrationID},
		{"待支付", user, pending.RegistrationID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.svc.Detail(ctx, tc.userID, tc.regID); mustBizCode(t, err) != 40401 {
				t.Fatalf("detail error = %v, want 40401", err)
			}
		})
	}
}
