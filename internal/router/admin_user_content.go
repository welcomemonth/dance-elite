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

// adminUserContentHandler 承载后台用户/选手/内容管理接口（联调 B5，
// API 文档 13.1/13.2/13.4）。
type adminUserContentHandler struct {
	users    *service.AdminUserQueryService
	contents *service.ContentService
}

// registerAdminUserContentRoutes 注册 /api/v1/admin/users、/players（列表/详情）
// 与 /contents/{key}。users/players 只读（cs+）；contents 读 cs+、写 admin+。
func registerAdminUserContentRoutes(group *gin.RouterGroup, users *service.AdminUserQueryService, contents *service.ContentService) {
	h := &adminUserContentHandler{users: users, contents: contents}
	group.GET("/users", h.listUsers)
	group.GET("/players", h.listPlayers)
	group.GET("/players/:player_id", h.getPlayer)

	write := group.Group("", middleware.RequireRole(model.RoleSuperAdmin, model.RoleAdmin))
	write.PUT("/contents/:key", h.putContent)
	write.POST("/contents/:key/image", h.uploadContentImage)
	group.GET("/contents/:key", h.getContent)
}

// listUsers 微信用户列表（API 文档 13.1）。
func (h *adminUserContentHandler) listUsers(c *gin.Context) {
	resp, err := h.users.ListUsers(c.Request.Context(),
		strings.TrimSpace(c.Query("keyword")), c.Query("status"),
		queryInt(c, "page", 1), queryIntMax(c, "page_size", 20, 100))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// listPlayers 选手档案列表（API 文档 13.2）。
func (h *adminUserContentHandler) listPlayers(c *gin.Context) {
	resp, err := h.users.ListPlayers(c.Request.Context(),
		strings.TrimSpace(c.Query("keyword")),
		queryInt(c, "page", 1), queryIntMax(c, "page_size", 20, 100))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// getPlayer 选手档案详情（API 文档 13.2）。
func (h *adminUserContentHandler) getPlayer(c *gin.Context) {
	playerID, err := strconv.ParseInt(c.Param("player_id"), 10, 64)
	if err != nil || playerID <= 0 {
		httpx.AbortError(c, httpx.ErrBadRequest("选手 ID 不合法"))
		return
	}
	resp, err := h.users.GetPlayer(c.Request.Context(), playerID)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// getContent 读取内容配置（API 文档 13.4）。
func (h *adminUserContentHandler) getContent(c *gin.Context) {
	resp, err := h.contents.Get(c.Request.Context(), c.Param("key"))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// putContent 写入内容配置（API 文档 13.4）：admin+，cs → 40301。
func (h *adminUserContentHandler) putContent(c *gin.Context) {
	actorID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	var req struct {
		Value map[string]any `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误：value 为必填 JSON 对象").WithCause(err))
		return
	}
	resp, err := h.contents.Put(c.Request.Context(), actorID, c.Param("key"), req.Value)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// uploadContentImage 上传内容图片（S5-5，公众号二维码）：admin+，cs → 40301。
func (h *adminUserContentHandler) uploadContentImage(c *gin.Context) {
	actorID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	file, closer, ok := readUploadedFile(c, "file", "图片", "图片不能超过 2MB",
		service.MaxContentImageRequestBytes)
	if !ok {
		return
	}
	defer closer()
	resp, err := h.contents.UploadImage(c.Request.Context(), actorID, c.Param("key"),
		&service.ContentImageFile{Filename: file.Filename, Size: file.Size, Content: file.Content})
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}
