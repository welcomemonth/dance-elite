package service

import (
	"context"
	"errors"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
)

// AdminUserQueryService 是后台用户/选手档案查询服务（联调 B5，API 文档 13.1/13.2）。只读。
// 证件号不落任何响应（哈希不可逆），手机号只出脱敏形态。
type AdminUserQueryService struct {
	stores *store.Store
}

// NewAdminUserQueryService 构造后台用户/选手查询服务。
func NewAdminUserQueryService(stores *store.Store) *AdminUserQueryService {
	return &AdminUserQueryService{stores: stores}
}

// ListUsers 返回微信用户分页列表（API 文档 13.1）。
func (s *AdminUserQueryService) ListUsers(ctx context.Context, keyword, status string, page, pageSize int) (*model.AdminWxUserListResponse, error) {
	if status != "" && status != "active" && status != "blocked" {
		return nil, httpx.ErrBadRequest("status 仅支持 active / blocked")
	}
	page, pageSize = normalizePage(page, pageSize)
	list, total, err := s.stores.Users.ListWxUsers(ctx, store.AdminWxUserFilter{
		Keyword: keyword, Status: status, Page: page, PageSize: pageSize,
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载用户列表失败，请稍后重试")
	}
	return &model.AdminWxUserListResponse{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

// ListPlayers 返回选手档案分页列表（API 文档 13.2）。
func (s *AdminUserQueryService) ListPlayers(ctx context.Context, keyword string, page, pageSize int) (*model.AdminPlayerListResponse, error) {
	page, pageSize = normalizePage(page, pageSize)
	list, total, err := s.stores.Players.ListPlayerBriefs(ctx, store.AdminPlayerFilter{
		Keyword: keyword, Page: page, PageSize: pageSize,
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载选手档案失败，请稍后重试")
	}
	return &model.AdminPlayerListResponse{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetPlayer 返回选手档案详情（API 文档 13.2）：基本信息 + 历史报名 +
// 成绩汇总 + 积分汇总 + 重复档案提示。
func (s *AdminUserQueryService) GetPlayer(ctx context.Context, playerID int64) (*model.AdminPlayerDetail, error) {
	brief, err := s.stores.Players.GetPlayerBrief(ctx, playerID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("选手档案不存在")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载选手档案失败，请稍后重试")
	}

	registrations, err := s.stores.Players.ListPlayerRegistrations(ctx, playerID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载选手档案失败，请稍后重试")
	}
	results, err := s.stores.Players.ListPlayerResults(ctx, playerID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载选手档案失败，请稍后重试")
	}
	points, err := s.stores.Players.GetPlayerPointsSummary(ctx, playerID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载选手档案失败，请稍后重试")
	}
	duplicates, err := s.stores.Players.ListPlayerDuplicates(ctx, playerID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载选手档案失败，请稍后重试")
	}

	return &model.AdminPlayerDetail{
		AdminPlayerBriefItem: *brief,
		Registrations:        registrations,
		ResultsSummary:       results,
		PointsSummary:        points,
		Duplicates:           duplicates,
	}, nil
}
