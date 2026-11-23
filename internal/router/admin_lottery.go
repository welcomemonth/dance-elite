package router

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/middleware"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/service"
	"github.com/meetmonth/dance-elite/internal/store"
)

// adminLotteryHandler 承载后台「赛事抽签」页接口（API 文档 7.3）。
type adminLotteryHandler struct {
	svc *service.AdminLotteryService
}

// registerAdminLotteryRoutes 注册 /api/v1/admin/lottery/sessions 下的接口。
// 列表/详情 cs 也可查看；导入、修改设置、导出仅 admin / super_admin。
func registerAdminLotteryRoutes(group *gin.RouterGroup, svc *service.AdminLotteryService) {
	h := &adminLotteryHandler{svc: svc}
	sessions := group.Group("/lottery/sessions")
	sessions.GET("", h.list)
	sessions.GET("/:token", h.get)

	write := sessions.Group("", middleware.RequireRole(model.RoleSuperAdmin, model.RoleAdmin))
	write.POST("", h.create)
	write.PUT("/:token", h.updateSettings)
	write.GET("/:token/export", h.export)
}

// list 返回全部赛事抽签（按更新时间倒序）。
func (h *adminLotteryHandler) list(c *gin.Context) {
	resp, err := h.svc.List(c.Request.Context())
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// get 返回单场抽签详情（含分组进度）。
func (h *adminLotteryHandler) get(c *gin.Context) {
	token, ok := lotteryTokenParam(c)
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), token)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// create 上传报名 Excel 并生成（或覆盖）赛事抽签。
func (h *adminLotteryHandler) create(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	input, err := parseLotteryImportForm(c)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	resp, err := h.svc.Import(c.Request.Context(), adminID, input)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// updateSettings 修改抽签时间窗口、短信模板或关联赛事。
func (h *adminLotteryHandler) updateSettings(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	token, ok := lotteryTokenParam(c)
	if !ok {
		return
	}
	var req model.AdminLotterySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误").WithCause(err))
		return
	}
	resp, err := h.svc.UpdateSettings(c.Request.Context(), adminID, token, req)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// export 导出抽签结果 Excel（姓名、身份证号、抽签信息）。
func (h *adminLotteryHandler) export(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	token, ok := lotteryTokenParam(c)
	if !ok {
		return
	}
	file, err := h.svc.Export(c.Request.Context(), adminID, token)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	disposition := fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		file.Name, url.PathEscape(file.Name))
	c.Header("Content-Disposition", disposition)
	c.Data(http.StatusOK, exportMIME, file.Content)
}

// lotteryTokenParam 读取并校验路径中的抽签 token。
func lotteryTokenParam(c *gin.Context) (string, bool) {
	token := c.Param("token")
	if !store.ValidLotteryToken(token) {
		httpx.AbortError(c, httpx.ErrBadRequest("抽签链接无效"))
		return "", false
	}
	return token, true
}
