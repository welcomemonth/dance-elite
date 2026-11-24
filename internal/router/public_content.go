package router

import (
	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/service"
)

// registerPublicContentRoutes 注册小程序端公开内容读取接口（S5-4）：
// GET /api/v1/contents/{key}，免登录，与后台内容管理读写同一份 settings 表。
func registerPublicContentRoutes(group *gin.RouterGroup, contents *service.ContentService) {
	h := &publicContentHandler{contents: contents}
	group.GET("/contents/:key", h.get)
}

type publicContentHandler struct {
	contents *service.ContentService
}

// get 读取已注册 key 的公开内容；未注册 key → 40401。
func (h *publicContentHandler) get(c *gin.Context) {
	resp, err := h.contents.Get(c.Request.Context(), c.Param("key"))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}
