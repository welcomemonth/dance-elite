package router

import (
	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/middleware"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/service"
)

// adminDashboardHandler 承载后台数据看板接口（联调 B3，API 文档 9.1）。
type adminDashboardHandler struct {
	svc *service.DashboardService
}

// registerAdminDashboardRoutes 注册 GET /api/v1/admin/dashboard。
// 管理员全量；cs 仅待办工单相关字段（service 层按角色裁剪）。
func registerAdminDashboardRoutes(group *gin.RouterGroup, svc *service.DashboardService) {
	h := &adminDashboardHandler{svc: svc}
	group.GET("/dashboard", h.overview)
}

// overview 数据看板总览。
func (h *adminDashboardHandler) overview(c *gin.Context) {
	role, ok := middleware.GetAdminRole(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	resp, err := h.svc.Overview(c.Request.Context(), role)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}
