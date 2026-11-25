package router

import (
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/middleware"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/pkg/token"
	"github.com/meetmonth/dance-elite/internal/service"
)

// lotteryHandler 承载免登录抽签 H5 的元信息、授权、查询与抽签接口（API 文档 7.2）。
type lotteryHandler struct {
	svc         *service.LotteryService
	auth        *service.LotteryAuthService
	signer      *token.Signer
	authEnabled bool
}

// registerLotteryH5Routes 注册 /h5/lottery/{token} 下的免登录接口。
// 抽签与查询按 IP 限流，避免有人刷接口；赛事现场可能大量家长共用 WiFi，
// 阈值与音频审核持平，不设得更严。
//
// 公众号网页授权开启时（生产默认），查询与抽签都要求有效授权 cookie，
// 抽签把 openid/unionid 写入选手记录；本地 dev 可用 open 模式跳过。
func registerLotteryH5Routes(group *gin.RouterGroup, svc *service.LotteryService,
	auth *service.LotteryAuthService, signer *token.Signer) {
	h := &lotteryHandler{
		svc:         svc,
		auth:        auth,
		signer:      signer,
		authEnabled: auth != nil && auth.Enabled() && signer != nil,
	}
	// 抽签链接只允许在微信内打开：非微信浏览器直接 404，且不暴露接口存在性。
	group.Use(middleware.WeChatOnly())

	if h.authEnabled {
		// 授权跳转与回调：不要求已有 cookie，但同样只允许微信内访问。
		// 赛事现场大量家长共用 WiFi，阈值与 meta 持平，避免误伤。
		group.GET("/lottery/:token/auth/start", middleware.RateLimit(120, time.Minute), h.authStart)
		group.GET("/lottery/:token/auth/callback", middleware.RateLimit(120, time.Minute), h.authCallback)
	}

	metaHandlers := []gin.HandlerFunc{}
	if h.authEnabled {
		metaHandlers = append(metaHandlers, middleware.OptionalLotteryAuth(signer))
	}
	metaHandlers = append(metaHandlers, middleware.RateLimit(120, time.Minute), h.meta)
	group.GET("/lottery/:token", metaHandlers...)

	protected := func(handler gin.HandlerFunc) []gin.HandlerFunc {
		handlers := []gin.HandlerFunc{middleware.RateLimit(60, time.Minute)}
		if h.authEnabled {
			handlers = append(handlers, middleware.RequireLotteryAuth(signer))
		}
		return append(handlers, handler)
	}
	group.POST("/lottery/:token/lookup", protected(h.lookup)...)
	group.POST("/lottery/:token/query", protected(h.query)...)
	group.POST("/lottery/:token/draw", protected(h.draw)...)
}

// meta 返回页面初始化所需的赛事名称与舞种列表，以及当前浏览器是否已授权。
func (h *lotteryHandler) meta(c *gin.Context) {
	resp, err := h.svc.Meta(c.Request.Context(), c.Param("token"))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	resp.Authorized = !h.authEnabled
	if h.authEnabled {
		_, _, ok := middleware.GetLotteryIdentity(c)
		resp.Authorized = ok
	}
	// 授权状态随 cookie 变化，禁止中间层与浏览器缓存。
	c.Header("Cache-Control", "no-store")
	httpx.OK(c, resp)
}

// authStart 302 跳转公众号网页授权（snsapi_base 静默授权）。
func (h *lotteryHandler) authStart(c *gin.Context) {
	redirectURL, state, err := h.auth.Start(c.Request.Context(), c.Param("token"))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	middleware.SetLotteryStateCookie(c, state)
	c.Header("Cache-Control", "no-store")
	c.Redirect(http.StatusFound, redirectURL)
}

// authCallback 校验 state、用 code 换 openid/unionid，种授权 cookie 后回到抽签页。
// 失败统一回页面并带 auth_error=1，由前端展示「重新授权」，不暴露内部原因。
func (h *lotteryHandler) authCallback(c *gin.Context) {
	rawToken := c.Param("token")
	stateCookie, _ := middleware.LotteryStateCookie(c)
	result, err := h.auth.Callback(c.Request.Context(), rawToken,
		c.Query("code"), c.Query("state"), stateCookie)
	middleware.ClearLotteryStateCookie(c)
	c.Header("Cache-Control", "no-store")
	if err != nil {
		logger.WarnEvent(c.Request.Context(), "lottery.auth_failed", "抽签微信授权失败", "error", err)
		c.Redirect(http.StatusFound, "/lottery/"+url.PathEscape(rawToken)+"?auth_error=1")
		return
	}
	middleware.SetLotteryAuthCookie(c, result.Cookie, h.auth.CookieMaxAge())
	c.Redirect(http.StatusFound, "/lottery/"+url.PathEscape(rawToken))
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
	actor := service.LotteryActor{}
	if h.authEnabled {
		openID, unionID, ok := middleware.GetLotteryIdentity(c)
		if !ok {
			httpx.AbortError(c, httpx.ErrUnauthorized("请重新在微信中打开链接完成授权后继续"))
			return
		}
		actor = service.LotteryActor{OpenID: openID, UnionID: unionID}
	}
	resp, err := h.svc.Draw(c.Request.Context(), c.Param("token"), req, actor)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}
