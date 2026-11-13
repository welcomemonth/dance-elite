package service

import (
	"context"
	"encoding/json"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
)

// AdminLogQueryService 是操作日志查询服务（联调 B4，API 文档 14.2）。只读。
// 日志只增不删、不可修改；导出仅限 super_admin（本期无导出接口）。
type AdminLogQueryService struct {
	stores *store.Store
}

// NewAdminLogQueryService 构造操作日志查询服务。
func NewAdminLogQueryService(stores *store.Store) *AdminLogQueryService {
	return &AdminLogQueryService{stores: stores}
}

// List 返回操作日志分页列表（API 文档 14.2）：按操作人/动作/对象/时间范围筛选。
func (s *AdminLogQueryService) List(ctx context.Context, q model.AdminLogListQuery) (*model.AdminLogListResponse, error) {
	page, pageSize := normalizePage(q.Page, q.PageSize)
	rows, total, err := s.stores.Admins.ListLogs(ctx, store.AdminLogFilter{
		AdminID:     q.AdminID,
		Action:      q.Action,
		TargetType:  q.TargetType,
		CreatedFrom: q.CreatedFrom,
		CreatedTo:   q.CreatedTo,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载操作日志失败，请稍后重试")
	}

	list := make([]model.AdminLogItem, 0, len(rows))
	for _, r := range rows {
		detail, err := parseLogDetail(r.Detail)
		if err != nil {
			return nil, httpx.ErrInternal(err, "加载操作日志失败，请稍后重试")
		}
		list = append(list, model.AdminLogItem{
			LogID:      r.LogID,
			AdminName:  r.AdminName,
			Action:     r.Action,
			TargetType: r.TargetType,
			TargetID:   r.TargetID,
			Detail:     detail,
			CreatedAt:  r.CreatedAt,
		})
	}
	return &model.AdminLogListResponse{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

// parseLogDetail 把 admin_logs.detail 的 JSON 文本解析为对象；nil 返回空对象。
func parseLogDetail(raw *string) (map[string]any, error) {
	if raw == nil || *raw == "" {
		return map[string]any{}, nil
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(*raw), &detail); err != nil {
		return nil, err
	}
	if detail == nil {
		detail = map[string]any{}
	}
	return detail, nil
}
