package service

import (
	"context"
	"sync"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// dashboardCacheTTL 是看板聚合结果的缓存时长（API 文档 9.1 注意事项：60 秒）。
const dashboardCacheTTL = 60 * time.Second

// closingEventHorizonDays 是「临近截止赛事」的展望窗口（天）。
const closingEventHorizonDays = 7

// DashboardService 是后台数据看板服务（开发计划 B3，API 文档 9.1）。只读。
// 聚合查询按角色缓存 60 秒，不要求实时到秒。
type DashboardService struct {
	stores *store.Store
	now    func() time.Time

	mu    sync.Mutex
	cache map[string]dashboardCacheEntry
}

type dashboardCacheEntry struct {
	data      *model.Dashboard
	expiresAt time.Time
}

// NewDashboardService 构造数据看板服务。
func NewDashboardService(stores *store.Store) *DashboardService {
	return &DashboardService{
		stores: stores,
		now:    time.Now,
		cache:  make(map[string]dashboardCacheEntry),
	}
}

// Overview 返回看板总览。cs 角色按文档仅返回待办工单相关字段。
func (s *DashboardService) Overview(ctx context.Context, role string) (*model.Dashboard, error) {
	if role == model.RoleCS {
		return s.csOverview(ctx)
	}

	s.mu.Lock()
	if entry, ok := s.cache[role]; ok && time.Now().Before(entry.expiresAt) {
		s.mu.Unlock()
		return entry.data, nil
	}
	s.mu.Unlock()

	d, err := s.load(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[role] = dashboardCacheEntry{data: d, expiresAt: time.Now().Add(dashboardCacheTTL)}
	s.mu.Unlock()
	return d, nil
}

// csOverview 客服看板：仅新客服工单计数，其余字段裁剪为零值（API 文档 9.1 鉴权说明）。
func (s *DashboardService) csOverview(ctx context.Context) (*model.Dashboard, error) {
	pending, err := s.stores.Dashboard.PendingCSTickets(ctx)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载数据看板失败，请稍后重试")
	}
	return &model.Dashboard{
		Todos: model.DashboardTodos{NewCSTickets: pending},
	}, nil
}

// load 汇总四个聚合查询为一个看板响应。
func (s *DashboardService) load(ctx context.Context) (*model.Dashboard, error) {
	now := s.now().In(utils.ChinaLoc)
	today := now.Format("2006-01-02")
	nowISO := utils.FormatISO(s.now())

	metrics, err := s.stores.Dashboard.Metrics(ctx, today)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载数据看板失败，请稍后重试")
	}
	closing, err := s.stores.Dashboard.ClosingEvents(ctx, nowISO, closingEventHorizonDays)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载数据看板失败，请稍后重试")
	}
	progress, err := s.stores.Dashboard.EventProgress(ctx)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载数据看板失败，请稍后重试")
	}
	trend, err := s.stores.Dashboard.Trend(ctx, today)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载数据看板失败，请稍后重试")
	}
	pendingCS, err := s.stores.Dashboard.PendingCSTickets(ctx)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载数据看板失败，请稍后重试")
	}

	return &model.Dashboard{
		Metrics: metrics,
		Todos: model.DashboardTodos{
			PendingRefunds: metrics.PendingRefundTickets,
			NewCSTickets:   pendingCS,
			ClosingEvents:  closing,
		},
		EventProgress: progress,
		Trend:         trend,
	}, nil
}
