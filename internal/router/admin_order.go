package router

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/middleware"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/service"
)

// adminOrderHandler 承载后台订单查询接口（开发计划 T17，API 文档 11.1）。
type adminOrderHandler struct {
	svc *service.AdminOrderQueryService
}

// registerAdminOrderRoutes 注册 /api/v1/admin/orders 下的接口。
// 列表对 admin/super_admin/cs 全部只读。
func registerAdminOrderRoutes(group *gin.RouterGroup, svc *service.AdminOrderQueryService) {
	h := &adminOrderHandler{svc: svc}
	orders := group.Group("/orders")
	orders.GET("", h.list)
	orders.GET("/:order_no", h.detail)
	// 订单列表导出（S5-6）：与列表同一筛选，仅 admin / super_admin（cs → 40301）。
	write := orders.Group("", middleware.RequireRole(model.RoleSuperAdmin, model.RoleAdmin))
	write.GET("/export", h.export)
	// 明文查看手机号：需独立权限（默认拒绝 → 40301），按账号限流 10 次/分钟，
	// 每次调用落 reveal_phone 日志（API 文档 11.3）。
	orders.POST("/:order_no/reveal-phone",
		middleware.RequireRole(model.RoleSuperAdmin, model.RoleAdmin),
		middleware.RateLimitKeyed(10, time.Minute, adminRateKey),
		h.revealPhone)
}

// adminRateKey 返回按管理员账号限流的键。
func adminRateKey(c *gin.Context) string {
	if id, ok := middleware.GetAdminID(c); ok {
		return "admin:" + strconv.FormatInt(id, 10)
	}
	return ""
}

// list 订单分页列表（赛事/组别/舞种/状态/来源/支付时间/关键字筛选）。
func (h *adminOrderHandler) list(c *gin.Context) {
	eventID, ok := optionalInt64Query(c, "event_id")
	if !ok {
		httpx.AbortError(c, httpx.ErrBadRequest("event_id 不合法"))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), model.AdminOrderListQuery{
		EventID:         eventID,
		Level:           c.Query("level"),
		DanceType:       c.Query("dance_type"),
		Status:          c.Query("status"),
		IncludeCanceled: queryBool(c, "include_canceled"),
		Source:          c.Query("source"),
		PaidFrom:        c.Query("paid_from"),
		PaidTo:          c.Query("paid_to"),
		Keyword:         c.Query("keyword"),
		Page:            queryInt(c, "page", 1),
		PageSize:        queryIntMax(c, "page_size", 20, 100),
	})
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// detail 订单详情（订单 + 支付流水 + 退款工单 + 客服工单 + 资料 + 操作记录）。
func (h *adminOrderHandler) detail(c *gin.Context) {
	resp, err := h.svc.Detail(c.Request.Context(), c.Param("order_no"))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// export 按当前筛选导出订单列表（S5-6，仅 admin / super_admin）。
func (h *adminOrderHandler) export(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	eventID, ok := optionalInt64Query(c, "event_id")
	if !ok {
		httpx.AbortError(c, httpx.ErrBadRequest("event_id 不合法"))
		return
	}
	file, err := h.svc.Export(c.Request.Context(), adminID, model.AdminOrderListQuery{
		EventID:         eventID,
		Level:           c.Query("level"),
		DanceType:       c.Query("dance_type"),
		Status:          c.Query("status"),
		IncludeCanceled: queryBool(c, "include_canceled"),
		Source:          c.Query("source"),
		PaidFrom:        c.Query("paid_from"),
		PaidTo:          c.Query("paid_to"),
		Keyword:         c.Query("keyword"),
	})
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	disposition := fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		file.Name, url.PathEscape(file.Name))
	c.Header("Content-Disposition", disposition)
	c.Data(http.StatusOK, exportMIME, file.Content)
}

// revealPhone 明文查看订单联系电话（API 文档 11.3），调用留痕。
func (h *adminOrderHandler) revealPhone(c *gin.Context) {
	actorID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	resp, err := h.svc.RevealPhone(c.Request.Context(), actorID, c.Param("order_no"))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// queryBool 解析布尔 query 参数（true/1 视为真，缺省为假）。
func queryBool(c *gin.Context, name string) bool {
	switch c.Query(name) {
	case "true", "1":
		return true
	default:
		return false
	}
}
