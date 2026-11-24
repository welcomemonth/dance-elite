package router

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/middleware"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/service"
)

// archiveMIME 是媒体归档 zip 的 Content-Type。
const archiveMIME = "application/zip"

// adminMaterialArchiveHandler 承载赛事媒体归档导出（S5-1/S5-2，API 文档 12.6）。
type adminMaterialArchiveHandler struct {
	svc *service.MaterialArchiveService
}

// registerAdminMaterialArchiveRoutes 注册 /api/v1/admin/events/{id}/materials/archive。
// 仅 admin / super_admin 可导出（cs → 40301）。
func registerAdminMaterialArchiveRoutes(group *gin.RouterGroup, svc *service.MaterialArchiveService) {
	h := &adminMaterialArchiveHandler{svc: svc}
	write := group.Group("", middleware.RequireRole(model.RoleSuperAdmin, model.RoleAdmin))
	write.GET("/events/:id/materials/archive", h.archive)
}

// archive 流式输出赛事媒体 zip（按选手分目录）。
func (h *adminMaterialArchiveHandler) archive(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	eventID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || eventID <= 0 {
		httpx.AbortError(c, httpx.ErrBadRequest("赛事 ID 非法"))
		return
	}

	name := h.svc.FileName(eventID)
	disposition := fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		name, url.PathEscape(name))
	c.Header("Content-Disposition", disposition)
	c.Header("Content-Type", archiveMIME)

	if _, err := h.svc.Archive(c.Request.Context(), adminID, eventID,
		service.MaterialArchiveOptions{
			PlayerNos:       c.QueryArray("player_nos"),
			IncludeCanceled: queryBool(c, "include_canceled"),
		}, c.Writer); err != nil {
		// 已开始输出时无法再改响应，只记录错误；未输出时返回统一 JSON 错误。
		if c.Writer.Written() {
			_ = c.Error(err)
			return
		}
		httpx.AbortError(c, err)
	}
}
