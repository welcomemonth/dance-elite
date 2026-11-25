package router

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/middleware"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/service"
)

// lotteryHandler 承载免登录抽签 H5 的元信息、查询与抽签接口（API 文档 7.2）。
type lotteryHandler struct {
	svc *service.LotteryService
}

// registerLotteryH5Routes 注册 /h5/lottery/{token} 下的免登录接口。
// 抽签与查询按 IP 限流，避免有人刷接口；赛事现场可能大量家长共用 WiFi，
// 阈值与音频审核持平，不设得更严。
func registerLotteryH5Routes(group *gin.RouterGroup, svc *service.LotteryService) {
	h := &lotteryHandler{svc: svc}
	// 抽签链接只允许在微信内打开：非微信浏览器直接 404，且不暴露接口存在性。
	group.Use(middleware.WeChatOnly())
	group.GET("/lottery/:token", middleware.RateLimit(120, time.Minute), h.meta)
	group.POST("/lottery/:token/lookup", middleware.RateLimit(60, time.Minute), h.lookup)
	group.POST("/lottery/:token/query", middleware.RateLimit(60, time.Minute), h.query)
	group.POST("/lottery/:token/draw", middleware.RateLimit(60, time.Minute), h.draw)
}

// meta 返回页面初始化所需的赛事名称与舞种列表。
func (h *lotteryHandler) meta(c *gin.Context) {
	resp, err := h.svc.Meta(c.Request.Context(), c.Param("token"))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// lookup 按证件号（可选姓名）返回全部抽签记录；已抽签记录带抽签时间。
func (h *lotteryHandler) lookup(c *gin.Context) {
	var req model.LotteryLookupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误：身份证号不能为空").WithCause(err))
		return
	}
	resp, err := h.svc.Lookup(c.Request.Context(), c.Param("token"), req)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// query 只按身份证号查询全部已抽签信息（不需要舞种）。
func (h *lotteryHandler) query(c *gin.Context) {
	var req model.LotteryLookupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误：身份证号不能为空").WithCause(err))
		return
	}
	resp, err := h.svc.Query(c.Request.Context(), c.Param("token"), req)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// draw 按姓名 + 身份证号抽签；舞种为空时由服务端决定（唯一/弹出选择）。
func (h *lotteryHandler) draw(c *gin.Context) {
	var req model.LotteryDrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误：姓名和身份证号不能为空").WithCause(err))
		return
	}
	resp, err := h.svc.Draw(c.Request.Context(), c.Param("token"), req)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}
