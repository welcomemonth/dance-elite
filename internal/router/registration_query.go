package router

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/middleware"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/token"
	"github.com/meetmonth/dance-elite/internal/service"
)

// registrationQueryHandler 承载小程序「我的报名」查询接口
// （开发计划 T35，API 文档 3.3/3.4）。
type registrationQueryHandler struct {
	svc *service.RegistrationQueryService
}

// registerRegistrationQueryRoutes 注册 GET /registrations 与
// GET /registrations/{registration_id}，均需用户登录且仅返回本人报名。
func registerRegistrationQueryRoutes(group *gin.RouterGroup, svc *service.RegistrationQueryService, signer *token.Signer) {
	h := &registrationQueryHandler{svc: svc}
	auth := middleware.UserAuth(signer)
	group.GET("/registrations", auth, h.list)
	group.GET("/registrations/:registration_id", auth, h.detail)
}

// list 返回当前用户报名列表（统一分页）。
func (h *registrationQueryHandler) list(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), userID,
		queryInt(c, "page", 1), queryIntMax(c, "page_size", 20, 100))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// detail 返回当前用户的一条报名详情；非本人/不存在统一 40401。
func (h *registrationQueryHandler) detail(c *gin.Context) {
	registrationID, err := strconv.ParseInt(c.Param("registration_id"), 10, 64)
	if err != nil || registrationID <= 0 {
		httpx.AbortError(c, httpx.ErrBadRequest("报名 ID 不合法"))
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	resp, err := h.svc.Detail(c.Request.Context(), userID, registrationID)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}
