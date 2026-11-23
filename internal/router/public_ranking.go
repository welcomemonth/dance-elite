package router

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/service"
)

type publicRankingHandler struct {
	svc *service.PublicRankingService
}

// registerPublicRankingRoutes 注册小程序榜单只读接口（API 文档 6.1–6.4，免鉴权）。
func registerPublicRankingRoutes(group *gin.RouterGroup, svc *service.PublicRankingService) {
	h := &publicRankingHandler{svc: svc}
	group.GET("/config", h.config)
	group.GET("/ranking", h.ranking)
	group.GET("/institution/ranking", h.institutionRanking)
	group.GET("/dancers/:entry_id", h.dancer)
}

func (h *publicRankingHandler) config(c *gin.Context) {
	cfg, err := h.svc.Config(c.Request.Context())
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, cfg)
}

func (h *publicRankingHandler) ranking(c *gin.Context) {
	board, err := h.svc.Ranking(c.Request.Context(), c.Query("level"), c.Query("dance_type"))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, board)
}

func (h *publicRankingHandler) institutionRanking(c *gin.Context) {
	resp, err := h.svc.InstitutionRanking(c.Request.Context())
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

func (h *publicRankingHandler) dancer(c *gin.Context) {
	entryID, err := strconv.ParseInt(c.Param("entry_id"), 10, 64)
	if err != nil || entryID <= 0 {
		httpx.AbortError(c, httpx.ErrBadRequest("entry_id 非法"))
		return
	}
	detail, err := h.svc.DancerDetail(c.Request.Context(), entryID)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, detail)
}
