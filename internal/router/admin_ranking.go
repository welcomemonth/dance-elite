package router

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/middleware"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/service"
)

type adminRankingHandler struct {
	scores   *service.ScoreService
	rankings *service.RankingAdminService
	prizes   *service.PrizeService
}

// registerAdminRankingRoutes 注册榜单中心接口（赛季规则/成绩/榜单/晋级/机构/奖金）。
// 全部写操作仅 admin / super_admin；读操作 cs 也可用。
func registerAdminRankingRoutes(group *gin.RouterGroup, scores *service.ScoreService,
	rankings *service.RankingAdminService, prizes *service.PrizeService) {
	h := &adminRankingHandler{scores: scores, rankings: rankings, prizes: prizes}
	write := group.Group("", middleware.RequireRole(model.RoleSuperAdmin, model.RoleAdmin))

	// 赛季级规则。
	group.GET("/seasons/:id/score-rules", h.getScoreRules)
	write.PUT("/seasons/:id/score-rules", h.putScoreRules)
	group.GET("/seasons/:id/prize-rules", h.getPrizeRules)
	write.PUT("/seasons/:id/prize-rules", h.putPrizeRules)

	// 成绩录入与纠错。
	group.GET("/events/:id/scores", h.listScores)
	group.GET("/events/:id/scores/template", h.scoreTemplate)
	write.POST("/events/:id/scores/import", h.importScores)
	write.POST("/events/:id/scores/preview", h.previewScores)
	write.POST("/events/:id/scores", h.publishScores)
	write.PUT("/scores/:entry_id", h.correctScore)

	// 榜单与晋级。
	group.GET("/seasons/:id/leaderboards", h.listLeaderboards)
	write.PUT("/seasons/:id/leaderboards/:level/:dance_type/players/:player_id", h.updateLeaderboardRow)
	group.GET("/seasons/:id/qualification-cards", h.listQualificationCards)
	write.POST("/seasons/:id/qualification-cards", h.grantQualification)
	write.DELETE("/seasons/:id/qualification-cards/:card_id", h.revokeQualification)
	group.GET("/seasons/:id/finals-roster", h.finalsRoster)
	write.POST("/seasons/:id/finals-roster/adjust", h.adjustFinalsRoster)
	group.GET("/seasons/:id/institution-ranking", h.institutionRanking)

	// 奖金明细。
	group.GET("/prize-statements", h.listPrizes)
	write.GET("/prize-statements/export", h.exportPrizes)
	write.PUT("/prize-statements/:item_id", h.adjustPrize)
	write.POST("/prize-statements/:item_id/pay", h.payPrize)
}

func pathInt64(c *gin.Context, name string) (int64, bool) {
	v, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || v <= 0 {
		httpx.AbortError(c, httpx.ErrBadRequest(name+" 非法"))
		return 0, false
	}
	return v, true
}

func (h *adminRankingHandler) getScoreRules(c *gin.Context) {
	seasonID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	rules, err := h.rankings.GetScoreRules(c.Request.Context(), seasonID)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, rules)
}

func (h *adminRankingHandler) putScoreRules(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	seasonID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var rules model.ScoreRules
	if err := c.ShouldBindJSON(&rules); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误：积分规则不是合法 JSON").WithCause(err))
		return
	}
	if err := h.rankings.PutScoreRules(c.Request.Context(), adminID, seasonID, rules); err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, rules)
}

func (h *adminRankingHandler) getPrizeRules(c *gin.Context) {
	seasonID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	rules, err := h.rankings.GetPrizeRules(c.Request.Context(), seasonID)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, rules)
}

func (h *adminRankingHandler) putPrizeRules(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	seasonID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var rules model.PrizeRules
	if err := c.ShouldBindJSON(&rules); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误：奖金规则不是合法 JSON").WithCause(err))
		return
	}
	if err := h.rankings.PutPrizeRules(c.Request.Context(), adminID, seasonID, rules); err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, rules)
}

func (h *adminRankingHandler) listScores(c *gin.Context) {
	eventID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	list, err := h.scores.List(c.Request.Context(), eventID,
		c.Query("level"), c.Query("dance_type"))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, gin.H{"list": list})
}

// scoreTemplate 下载成绩导入模板（固定列：姓名/证件号/奖项/晋级决赛/备注）。
func (h *adminRankingHandler) scoreTemplate(c *gin.Context) {
	content, err := service.BuildScoreTemplateWorkbook()
	if err != nil {
		httpx.AbortError(c, httpx.ErrInternal(err, "生成导入模板失败，请稍后重试"))
		return
	}
	name := "成绩导入模板.xlsx"
	disposition := fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		name, url.PathEscape(name))
	c.Header("Content-Disposition", disposition)
	c.Data(http.StatusOK, exportMIME, content)
}

// importScores 上传成绩 Excel：解析、匹配/建档选手并返回预览（不写成绩）。
func (h *adminRankingHandler) importScores(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	eventID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	rows, err := parseScoreImportFile(c)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	resp, err := h.scores.ImportPreview(c.Request.Context(), adminID, eventID,
		c.Query("level"), c.Query("dance_type"), rows)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

func (h *adminRankingHandler) previewScores(c *gin.Context) {
	eventID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req model.ScorePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误：rows 不能为空").WithCause(err))
		return
	}
	resp, err := h.scores.Preview(c.Request.Context(), eventID,
		c.Query("level"), c.Query("dance_type"), req.Rows)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

func (h *adminRankingHandler) publishScores(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	eventID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req model.PublishScoresRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误：rows 不能为空").WithCause(err))
		return
	}
	resp, err := h.scores.Publish(c.Request.Context(), adminID, eventID,
		c.Query("level"), c.Query("dance_type"), req.Rows, c.Query("import_batch"))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

func (h *adminRankingHandler) correctScore(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	entryID, ok := pathInt64(c, "entry_id")
	if !ok {
		return
	}
	var req model.CorrectScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误").WithCause(err))
		return
	}
	newID, err := h.scores.Correct(c.Request.Context(), adminID, entryID, req)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, gin.H{"entry_id": newID})
}

func (h *adminRankingHandler) listLeaderboards(c *gin.Context) {
	seasonID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	resp, err := h.rankings.ListLeaderboards(c.Request.Context(), seasonID)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, resp)
}

func (h *adminRankingHandler) updateLeaderboardRow(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	seasonID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	playerID, ok := pathInt64(c, "player_id")
	if !ok {
		return
	}
	board, err := h.rankings.UpdateLeaderboardRowByKey(c.Request.Context(), adminID,
		seasonID, c.Param("level"), c.Param("dance_type"), playerID, decodeLeaderboardPatch(c))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, gin.H{"board_id": board, "player_id": playerID})
}

func decodeLeaderboardPatch(c *gin.Context) model.UpdateLeaderboardRowRequest {
	var req model.UpdateLeaderboardRowRequest
	_ = c.ShouldBindJSON(&req)
	return req
}

func (h *adminRankingHandler) listQualificationCards(c *gin.Context) {
	seasonID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	list, err := h.rankings.ListQualificationCards(c.Request.Context(), seasonID)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, gin.H{"list": list})
}

func (h *adminRankingHandler) grantQualification(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	var req model.GrantQualificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误：board_id/player_id/reason 必填").WithCause(err))
		return
	}
	id, err := h.rankings.GrantQualification(c.Request.Context(), adminID, req)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, gin.H{"card_id": id})
}

func (h *adminRankingHandler) revokeQualification(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	cardID, ok := pathInt64(c, "card_id")
	if !ok {
		return
	}
	if err := h.rankings.RevokeQualification(c.Request.Context(), adminID, cardID); err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, gin.H{"card_id": cardID, "status": "canceled"})
}

func (h *adminRankingHandler) finalsRoster(c *gin.Context) {
	seasonID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	list, err := h.rankings.FinalsRoster(c.Request.Context(), seasonID)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, gin.H{"list": list})
}

func (h *adminRankingHandler) adjustFinalsRoster(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	var req model.AdjustFinalsRosterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误：board_id/player_id/action/reason 必填").WithCause(err))
		return
	}
	if err := h.rankings.AdjustFinalsRoster(c.Request.Context(), adminID, req); err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, gin.H{"adjusted": true})
}

func (h *adminRankingHandler) institutionRanking(c *gin.Context) {
	seasonID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	list, err := h.rankings.InstitutionRanking(c.Request.Context(), seasonID)
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, gin.H{"list": list})
}

func (h *adminRankingHandler) listPrizes(c *gin.Context) {
	seasonID, ok := optionalInt64Query(c, "season_id")
	if !ok {
		httpx.AbortError(c, httpx.ErrBadRequest("season_id 不合法"))
		return
	}
	if seasonID <= 0 {
		httpx.AbortError(c, httpx.ErrBadRequest("season_id 必填"))
		return
	}
	eventID, ok := optionalInt64Query(c, "event_id")
	if !ok {
		httpx.AbortError(c, httpx.ErrBadRequest("event_id 不合法"))
		return
	}
	list, err := h.prizes.List(c.Request.Context(), seasonID, c.Query("status"))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	if eventID > 0 {
		filtered := list[:0]
		for _, item := range list {
			if item.EventID != nil && *item.EventID == eventID {
				filtered = append(filtered, item)
			}
		}
		list = filtered
	}
	total := 0
	for _, item := range list {
		total += item.AdjustedAmount
	}
	httpx.OK(c, gin.H{"list": list, "total": len(list), "total_amount": total})
}

func (h *adminRankingHandler) exportPrizes(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	seasonID, ok := optionalInt64Query(c, "season_id")
	if !ok {
		httpx.AbortError(c, httpx.ErrBadRequest("season_id 不合法"))
		return
	}
	eventID, ok := optionalInt64Query(c, "event_id")
	if !ok {
		httpx.AbortError(c, httpx.ErrBadRequest("event_id 不合法"))
		return
	}
	file, err := h.prizes.Export(c.Request.Context(), adminID, seasonID, eventID, c.Query("status"))
	if err != nil {
		httpx.AbortError(c, err)
		return
	}
	disposition := fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		file.Name, url.PathEscape(file.Name))
	c.Header("Content-Disposition", disposition)
	c.Data(http.StatusOK, exportMIME, file.Content)
}

func (h *adminRankingHandler) adjustPrize(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	itemID, ok := pathInt64(c, "item_id")
	if !ok {
		return
	}
	var req model.AdjustPrizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误：amount/reason 必填").WithCause(err))
		return
	}
	if err := h.prizes.Adjust(c.Request.Context(), adminID, itemID, req); err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, gin.H{"item_id": itemID, "amount": req.Amount})
}

func (h *adminRankingHandler) payPrize(c *gin.Context) {
	adminID, ok := middleware.GetAdminID(c)
	if !ok {
		httpx.AbortError(c, httpx.ErrUnauthorized("未登录或登录已过期"))
		return
	}
	itemID, ok := pathInt64(c, "item_id")
	if !ok {
		return
	}
	var req model.PayPrizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortError(c, httpx.ErrBadRequest("请求参数错误：payee 必填").WithCause(err))
		return
	}
	if err := h.prizes.Pay(c.Request.Context(), adminID, itemID, req); err != nil {
		httpx.AbortError(c, err)
		return
	}
	httpx.OK(c, gin.H{"item_id": itemID, "paid": true})
}
