package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// AdminRefundQueryService 是退款工单查询服务（联调 B2 散点，API 文档 11.4.2/11.4.2b）。只读。
type AdminRefundQueryService struct {
	stores *store.Store
	now    func() time.Time
}

// NewAdminRefundQueryService 构造退款工单查询服务。
func NewAdminRefundQueryService(stores *store.Store) *AdminRefundQueryService {
	return &AdminRefundQueryService{stores: stores, now: time.Now}
}

// List 返回退款工单分页列表（API 文档 11.4.2）。
// cs 角色仅返回自己登记的工单（CreatedBy=自身）。
func (s *AdminRefundQueryService) List(ctx context.Context, role string, adminID int64, q model.AdminRefundTicketListQuery) (*model.AdminRefundTicketListResponse, error) {
	if q.Status != "" && !isRefundStatus(q.Status) {
		return nil, httpx.ErrBadRequest("status 不合法")
	}
	createdBy := q.CreatedBy
	if role == model.RoleCS {
		createdBy = adminID
	}
	page, pageSize := normalizePage(q.Page, q.PageSize)
	list, total, err := s.stores.RefundTickets.ListAdmin(ctx, store.AdminRefundTicketFilter{
		Status:      q.Status,
		EventID:     q.EventID,
		CreatedFrom: q.CreatedFrom,
		CreatedTo:   q.CreatedTo,
		Keyword:     q.Keyword,
		CreatedBy:   createdBy,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载退款工单失败，请稍后重试")
	}
	return &model.AdminRefundTicketListResponse{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

// Detail 返回退款工单详情（API 文档 11.4.2b）：订单摘要 + 政策阶段 +
// 建议退款金额（与同意接口默认口径一致）+ 处理留痕。
func (s *AdminRefundQueryService) Detail(ctx context.Context, role string, adminID, ticketID int64) (*model.AdminRefundTicketDetail, error) {
	detail, err := s.stores.RefundTickets.GetAdminDetail(ctx, ticketID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("退款工单不存在")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载退款工单失败，请稍后重试")
	}

	// cs 仅可查看自己登记的工单。
	if role == model.RoleCS {
		ticket, err := s.stores.RefundTickets.Get(ctx, ticketID)
		if err != nil {
			return nil, httpx.ErrInternal(err, "加载退款工单失败，请稍后重试")
		}
		if ticket.CreatedBy == nil || *ticket.CreatedBy != adminID {
			return nil, httpx.ErrForbidden("仅可查看自己登记的工单")
		}
	}

	order, event, err := s.loadOrderEvent(ctx, detail.OrderNo)
	if err != nil {
		return nil, err
	}
	if event.LotteryAt != nil && *event.LotteryAt != "" {
		lotteryAt, parseErr := utils.ParseISO(*event.LotteryAt)
		if parseErr == nil && !s.now().Before(lotteryAt) {
			detail.PolicyStage = "lottery_after"
		} else {
			detail.PolicyStage = "lottery_before"
		}
	} else {
		detail.PolicyStage = "lottery_before"
	}
	detail.SuggestAmount = refundDefaultAmount(order, event, s.now())

	history, err := s.loadHistory(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	detail.History = history
	return detail, nil
}

// loadOrderEvent 读取工单对应订单与赛事（建议金额/政策阶段计算用）。
// 与同意退款的退费政策口径一致（refundDefaultAmount）。
func (s *AdminRefundQueryService) loadOrderEvent(ctx context.Context, orderNo string) (*model.Order, *model.Event, error) {
	order, err := s.stores.Orders.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, nil, httpx.ErrInternal(err, "加载退款工单失败，请稍后重试")
	}
	registration, err := s.stores.Registrations.Get(ctx, order.RegistrationID)
	if err != nil {
		return nil, nil, httpx.ErrInternal(err, "加载退款工单失败，请稍后重试")
	}
	event, err := s.stores.Events.Get(ctx, registration.EventID)
	if err != nil {
		return nil, nil, httpx.ErrInternal(err, "加载退款工单失败，请稍后重试")
	}
	return order, event, nil
}

// loadHistory 汇总该工单的处理留痕（admin_logs，target=refund_ticket，升序）。
func (s *AdminRefundQueryService) loadHistory(ctx context.Context, ticketID int64) ([]model.RefundHistoryEntry, error) {
	targetType := "refund_ticket"
	rows, _, err := s.stores.Admins.ListLogs(ctx, store.AdminLogFilter{
		TargetType: targetType, TargetID: ticketID, Page: 1, PageSize: 100,
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载处理留痕失败，请稍后重试")
	}
	history := make([]model.RefundHistoryEntry, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		history = append(history, model.RefundHistoryEntry{
			Action:    r.Action,
			Operator:  r.AdminName,
			CreatedAt: r.CreatedAt,
			Detail:    compactLogDetail(r.Detail),
		})
	}
	return history, nil
}

// compactLogDetail 把操作详情 JSON 压缩为单行文本；nil/非法 JSON 返回空串。
func compactLogDetail(raw *string) string {
	if raw == nil || *raw == "" {
		return ""
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(*raw), &detail); err != nil {
		return ""
	}
	parts := make([]string, 0, len(detail))
	for k, v := range detail {
		parts = append(parts, k+"="+ stringifyLogValue(v))
	}
	return strings.Join(parts, "; ")
}

func stringifyLogValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

// isRefundStatus 判断退款工单状态枚举值。
func isRefundStatus(status string) bool {
	switch status {
	case model.RefundTicketStatusPending, model.RefundTicketStatusProcessing,
		model.RefundTicketStatusApproved, model.RefundTicketStatusRejected,
		model.RefundTicketStatusRefunded, model.RefundTicketStatusRefundFailed:
		return true
	}
	return false
}
