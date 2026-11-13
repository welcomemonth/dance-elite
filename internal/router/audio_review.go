package router

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/middleware"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/service"
)

// audioReviewHandler 承载音频审核链接生成（后台）与 H5 免登录数据接口。
type audioReviewHandler struct {
	svc *service.AudioReviewService
}

// registerAdminAudioReviewRoutes 注册 POST /admin/events/{id}/audio-review-token
// （API 文档 10.6：admin / cs 均可）。
func registerAdminAudioReviewRoutes(group *gin.RouterGroup, svc *service.AudioReviewService) {
	h := &audioReviewHandler{svc: svc}
	write := group.Group("", middleware.RequireRole(model.RoleSuperAdmin, model.RoleAdmin, model.RoleCS))
	write.POST("/events/:id/audio-review-token", h.generateToken)
}

// registerAudioReviewH5Routes 注册 /h5 下的免登录音频审核接口（API 文档 7.1）。
func registerAudioReviewH5Routes(group *gin.RouterGroup, svc *service.AudioReviewService) {
	h := &audioReviewHandler{svc: svc}
	group.GET("/audio-review/:token", middleware.RateLimit(60, time.Minute), h.get)
}

// generateToken 生成/重新生成音频审核链接（旧链接立即失效）。
func (h *audioReviewHandler) generateToken(c *gin.Context) {
	eventID, ok := eventIDParam(c)
	if !ok {
		return
	}
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	resp, err := h.svc.GenerateToken(c.Request.Context(), adminID, eventID)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// get 返回 H5 审核列表（token 无效/已失效 → 40401）。
func (h *audioReviewHandler) get(c *gin.Context) {
	resp, err := h.svc.Get(c.Request.Context(), c.Param("token"), c.Query("keyword"))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}
