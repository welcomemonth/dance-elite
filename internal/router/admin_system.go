package router

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/middleware"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/service"
)

// adminSystemHandler 承载系统管理接口（联调 B4，API 文档 14.1/14.2）：
// 账号管理（super_admin 专属）与操作日志查询（super_admin / admin）。
type adminSystemHandler struct {
	accounts *service.AdminAccountService
	logs     *service.AdminLogQueryService
}

// registerAdminSystemRoutes 注册 /api/v1/admin/accounts 与 /api/v1/admin/logs。
func registerAdminSystemRoutes(group *gin.RouterGroup, accounts *service.AdminAccountService, logs *service.AdminLogQueryService) {
	h := &adminSystemHandler{accounts: accounts, logs: logs}

	super := group.Group("", middleware.RequireRole(model.RoleSuperAdmin))
	super.GET("/accounts", h.listAccounts)
	super.POST("/accounts", h.createAccount)
	super.PUT("/accounts/:admin_id", h.updateAccount)

	group.GET("/logs", middleware.RequireRole(model.RoleSuperAdmin, model.RoleAdmin), h.listLogs)
}

// listAccounts 后台账号列表（API 文档 14.1）。
func (h *adminSystemHandler) listAccounts(c *gin.Context) {
	resp, err := h.accounts.List(c.Request.Context(),
		strings.TrimSpace(c.Query("keyword")),
		queryInt(c, "page", 1), queryIntMax(c, "page_size", 20, 100))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// createAccount 创建后台账号（API 文档 14.1）。
func (h *adminSystemHandler) createAccount(c *gin.Context) {
	actorID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	var req model.CreateAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误：username/password/role/name 均为必填"))
		return
	}
	resp, err := h.accounts.Create(c.Request.Context(), actorID, req)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// updateAccount 更新账号状态/密码/角色（API 文档 14.1）。
func (h *adminSystemHandler) updateAccount(c *gin.Context) {
	actorID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	adminID, err := strconv.ParseInt(c.Param("admin_id"), 10, 64)
	if err != nil || adminID <= 0 {
		httpx.AbortError(c, httpx.ErrBadRequest("账号 ID 不合法"))
		return
	}
	var req model.UpdateAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误"))
		return
	}
	resp, err := h.accounts.Update(c.Request.Context(), actorID, adminID, req)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// listLogs 操作日志查询（API 文档 14.2）。
func (h *adminSystemHandler) listLogs(c *gin.Context) {
	adminID, ok := optionalInt64Query(c, "admin_id")
	if !ok {
		httpx.AbortError(c, httpx.ErrBadRequest("admin_id 不合法"))
		return
	}
	resp, err := h.logs.List(c.Request.Context(), model.AdminLogListQuery{
		AdminID:     adminID,
		Action:      c.Query("action"),
		TargetType:  c.Query("target_type"),
		CreatedFrom: c.Query("created_from"),
		CreatedTo:   c.Query("created_to"),
		Page:        queryInt(c, "page", 1),
		PageSize:    queryIntMax(c, "page_size", 20, 100),
	})
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}
