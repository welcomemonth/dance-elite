package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
)

const (
	adminActionLeaderboardUpdate = "admin.leaderboard.update"
	adminActionQualification     = "admin.qualification.adjust"
	adminActionPrizeAdjust       = "admin.prize.adjust"
	adminActionPrizePaid         = "admin.prize.paid"
)

// RankingAdminService 是后台榜单中心（T29/T31）的读写服务。
type RankingAdminService struct {
	stores *store.Store
	now    func() time.Time
}

// NewRankingAdminService 构造后台榜单服务。
func NewRankingAdminService(stores *store.Store) *RankingAdminService {
	return &RankingAdminService{stores: stores, now: time.Now}
}

// ListLeaderboards 返回赛季 6 个榜单（含隐藏选手）。
func (s *RankingAdminService) ListLeaderboards(ctx context.Context, seasonID int64) (*model.AdminLeaderboardListResponse, error) {
	boards, err := s.stores.Boards.ListBySeason(ctx, seasonID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载榜单失败，请稍后重试")
	}
	results, err := s.stores.Results.ListPublishedBySeasonRead(ctx, seasonID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载成绩失败，请稍后重试")
	}
	institution := institutionOf(results)
	direct := map[[2]int64]bool{}
	for _, b := range boards {
		qs, err := s.stores.Qualifications.ListValidByBoard(ctx, b.ID)
		if err != nil {
			return nil, httpx.ErrInternal(err, "加载资格失败，请稍后重试")
		}
		for _, q := range qs {
			if q.Qualification.Source == model.QualificationAutoDirect ||
				q.Qualification.Source == model.QualificationManualDirect {
				direct[[2]int64{b.ID, q.Qualification.PlayerID}] = true
			}
		}
	}

	out := &model.AdminLeaderboardListResponse{List: make([]model.AdminLeaderboard, 0, len(boards))}
	for _, b := range boards {
		points, err := s.stores.Points.ListPlayerPointsByBoard(ctx, b.ID)
		if err != nil {
			return nil, httpx.ErrInternal(err, "加载积分失败，请稍后重试")
		}
		lb := model.AdminLeaderboard{
			BoardID: b.ID, Level: b.Level, DanceType: b.DanceType,
			PublishedAt: b.PublishedAt, Rows: make([]model.AdminLeaderboardRow, 0, len(points)),
		}
		for _, p := range points {
			rank := 0
			if p.PlayerPoint.Rank != nil {
				rank = *p.PlayerPoint.Rank
			}
			lb.Rows = append(lb.Rows, model.AdminLeaderboardRow{
				Rank: rank, PlayerID: p.PlayerPoint.PlayerID,
				PlayerNo: p.PlayerNo, PlayerName: p.PlayerName,
				DisplayName:         p.PlayerPoint.DisplayName,
				Institution:         institution[p.PlayerPoint.PlayerID],
				TotalPoints:         p.PlayerPoint.TotalPoints,
				EventCount:          len(parseEventIDs(p.PlayerPoint.UsedEventIDs)),
				Hidden:              p.PlayerPoint.Hidden,
				DirectQualification: direct[[2]int64{b.ID, p.PlayerPoint.PlayerID}],
			})
		}
		out.List = append(out.List, lb)
	}
	return out, nil
}

// UpdateLeaderboardRow 设置展示名/隐藏并重算排名。
func (s *RankingAdminService) UpdateLeaderboardRow(ctx context.Context, adminID, boardID, playerID int64,
	req model.UpdateLeaderboardRowRequest) error {
	board, err := s.stores.Boards.Get(ctx, boardID)
	if errors.Is(err, store.ErrNotFound) {
		return httpx.ErrNotFound("榜单不存在")
	}
	if err != nil {
		return httpx.ErrInternal(err, "加载榜单失败，请稍后重试")
	}
	now := s.now()
	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		if req.DisplayName != nil {
			name := *req.DisplayName
			if name == "" {
				if err := s.stores.Points.UpdateDisplayName(ctx, tx, boardID, playerID, nil); err != nil {
					return err
				}
			} else if err := s.stores.Points.UpdateDisplayName(ctx, tx, boardID, playerID, &name); err != nil {
				return err
			}
		}
		if req.Hidden != nil {
			if err := s.stores.Points.UpdateHidden(ctx, tx, boardID, playerID, *req.Hidden); err != nil {
				return err
			}
		}
		if err := recomputeBoardPoints(ctx, s.stores, tx, board.SeasonID, boardID, now); err != nil {
			return err
		}
		targetType := "board_player"
		return adminLog(ctx, s.stores, tx, &adminID, adminActionLeaderboardUpdate,
			&targetType, &playerID, map[string]any{"board_id": boardID,
				"display_name": req.DisplayName, "hidden": req.Hidden})
	})
	if errors.Is(err, store.ErrNotFound) {
		return httpx.ErrNotFound("该选手不在榜单中")
	}
	if err != nil {
		return err
	}
	return nil
}

// ListQualificationCards 返回赛季直通卡（自动 + 手动）。
func (s *RankingAdminService) ListQualificationCards(ctx context.Context, seasonID int64) ([]model.AdminQualificationCard, error) {
	rows, err := s.stores.Qualifications.ListValidBySeason(ctx, seasonID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载资格失败，请稍后重试")
	}
	out := make([]model.AdminQualificationCard, 0, len(rows))
	for _, r := range rows {
		source := "auto"
		if r.Qualification.Source == model.QualificationManualDirect ||
			r.Qualification.Source == model.QualificationManual {
			source = "manual"
		}
		out = append(out, model.AdminQualificationCard{
			CardID: r.Qualification.ID, PlayerID: r.Qualification.PlayerID,
			PlayerNo: r.PlayerNo, PlayerName: r.PlayerName,
			Board: r.BoardLevel + "·" + r.DanceType, Source: source,
			Reason: r.Qualification.Reason, GrantedAt: r.Qualification.CreatedAt,
		})
	}
	return out, nil
}

// GrantQualification 手动增补直通卡。
func (s *RankingAdminService) GrantQualification(ctx context.Context, adminID int64, req model.GrantQualificationRequest) (int64, error) {
	board, err := s.stores.Boards.Get(ctx, req.BoardID)
	if errors.Is(err, store.ErrNotFound) {
		return 0, httpx.ErrNotFound("榜单不存在")
	}
	if err != nil {
		return 0, httpx.ErrInternal(err, "加载榜单失败，请稍后重试")
	}
	var id int64
	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		id, err = s.stores.Qualifications.Insert(ctx, tx, model.FinalQualification{
			SeasonID: board.SeasonID, BoardID: board.ID, PlayerID: req.PlayerID,
			Source: model.QualificationManualDirect, Reason: strPtr(req.Reason),
			CreatedBy: &adminID,
		})
		if err != nil {
			return err
		}
		targetType := "qualification"
		return adminLog(ctx, s.stores, tx, &adminID, adminActionQualification,
			&targetType, &id, map[string]any{"action": "grant", "player_id": req.PlayerID, "reason": req.Reason})
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return 0, httpx.ErrConflict("该选手已持有该榜单有效资格")
		}
		return 0, err
	}
	return id, nil
}

// RevokeQualification 取消直通卡。
func (s *RankingAdminService) RevokeQualification(ctx context.Context, adminID, cardID int64) error {
	return s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		q, err := s.stores.Qualifications.Get(ctx, cardID)
		if errors.Is(err, store.ErrNotFound) {
			return httpx.ErrNotFound("资格不存在")
		}
		if err != nil {
			return err
		}
		if err := s.stores.Qualifications.Cancel(ctx, tx, cardID); err != nil {
			return httpx.ErrNotFound("资格不存在或已取消")
		}
		targetType := "qualification"
		return adminLog(ctx, s.stores, tx, &adminID, adminActionQualification,
			&targetType, &cardID, map[string]any{"action": "revoke", "player_id": q.PlayerID})
	})
}

// FinalsRoster 返回总决赛资格名单。
func (s *RankingAdminService) FinalsRoster(ctx context.Context, seasonID int64) ([]model.AdminFinalsRosterRow, error) {
	boards, err := s.stores.Boards.ListBySeason(ctx, seasonID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载榜单失败，请稍后重试")
	}
	results, err := s.stores.Results.ListPublishedBySeasonRead(ctx, seasonID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载成绩失败，请稍后重试")
	}
	institution := institutionOf(results)
	out := make([]model.AdminFinalsRosterRow, 0, 32)
	for _, b := range boards {
		qs, err := s.stores.Qualifications.ListValidByBoard(ctx, b.ID)
		if err != nil {
			return nil, httpx.ErrInternal(err, "加载资格失败，请稍后重试")
		}
		for _, q := range qs {
			source := "points_top20"
			manual := false
			switch q.Qualification.Source {
			case model.QualificationAutoDirect:
				source = "direct_card"
			case model.QualificationManualDirect:
				source, manual = "direct_card", true
			case model.QualificationManual:
				source, manual = "points_top20", true
			}
			rank := 0
			points := 0
			if p, err := s.stores.Points.GetPlayerPoint(ctx, b.ID, q.Qualification.PlayerID); err == nil {
				if p.PlayerPoint.Rank != nil {
					rank = *p.PlayerPoint.Rank
				}
				points = p.PlayerPoint.TotalPoints
			}
			out = append(out, model.AdminFinalsRosterRow{
				PlayerID: q.Qualification.PlayerID, PlayerNo: q.PlayerNo, PlayerName: q.PlayerName,
				Institution: institution[q.Qualification.PlayerID],
				Board:       b.Level + "·" + b.DanceType, Source: source,
				TotalPoints: points, Rank: rank, Manual: manual,
			})
		}
	}
	return out, nil
}

// AdjustFinalsRoster 手动增补/移除总决赛资格。
func (s *RankingAdminService) AdjustFinalsRoster(ctx context.Context, adminID int64, req model.AdjustFinalsRosterRequest) error {
	board, err := s.stores.Boards.Get(ctx, req.BoardID)
	if errors.Is(err, store.ErrNotFound) {
		return httpx.ErrNotFound("榜单不存在")
	}
	if err != nil {
		return httpx.ErrInternal(err, "加载榜单失败，请稍后重试")
	}
	if req.Action != "add" && req.Action != "remove" {
		return httpx.ErrBadRequest("action 取值不合法（add / remove）")
	}
	return s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		var targetID int64
		if req.Action == "add" {
			var err error
			targetID, err = s.stores.Qualifications.Insert(ctx, tx, model.FinalQualification{
				SeasonID: board.SeasonID, BoardID: board.ID, PlayerID: req.PlayerID,
				Source: model.QualificationManual, Reason: strPtr(req.Reason), CreatedBy: &adminID,
			})
			if err != nil {
				return httpx.ErrConflict("该选手已持有该榜单有效资格")
			}
		} else {
			qs, err := s.stores.Qualifications.ListValidByBoardTx(ctx, tx, board.ID)
			if err != nil {
				return err
			}
			for _, q := range qs {
				if q.Qualification.PlayerID == req.PlayerID {
					targetID = q.Qualification.ID
					break
				}
			}
			if targetID == 0 {
				return httpx.ErrNotFound("该选手不在总决赛名单中")
			}
			if err := s.stores.Qualifications.Cancel(ctx, tx, targetID); err != nil {
				return err
			}
		}
		targetType := "qualification"
		return adminLog(ctx, s.stores, tx, &adminID, adminActionQualification,
			&targetType, &targetID, map[string]any{"action": req.Action,
				"player_id": req.PlayerID, "reason": req.Reason})
	})
}

// InstitutionRanking 返回赛季机构榜（含荣誉与奖金）。
func (s *RankingAdminService) InstitutionRanking(ctx context.Context, seasonID int64) ([]model.AdminInstitutionRankRow, error) {
	rows, err := s.stores.Points.ListInstitutionPoints(ctx, seasonID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载机构榜失败，请稍后重试")
	}
	season, err := s.stores.Seasons.Get(ctx, seasonID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载赛季失败，请稍后重试")
	}
	var rulesPtr *string
	rules := model.DefaultPrizeRules
	if season != nil {
		rules = resolvePrizeRules(rulesPtr)
	}
	out := make([]model.AdminInstitutionRankRow, 0, len(rows))
	for _, r := range rows {
		rank := 0
		if r.Rank != nil {
			rank = *r.Rank
		}
		var honor *string
		prize := 0
		switch {
		case rank >= 1 && rank <= 3:
			h := "annual_elite"
			honor = &h
			prize = rules.InstitutionTop3 * 100
		case rank >= 4 && rank <= 10:
			h := "annual_top10"
			honor = &h
		}
		out = append(out, model.AdminInstitutionRankRow{
			Rank: rank, Institution: r.InstitutionName, TotalPoints: r.TotalPoints,
			PlayerCount: r.PlayerCount, Honor: honor, PrizeAmount: prize,
		})
	}
	return out, nil
}

// institutionOf 从成绩行提取选手最近机构（按行顺序后者覆盖）。
func institutionOf(rows []store.ResultRow) map[int64]string {
	out := map[int64]string{}
	for _, r := range rows {
		if r.Result.Institution != nil && *r.Result.Institution != "" {
			out[r.Result.PlayerID] = *r.Result.Institution
		}
	}
	return out
}

func parseEventIDs(raw string) []int64 {
	var ids []int64
	_ = json.Unmarshal([]byte(raw), &ids)
	return ids
}

// 赛季级积分/奖金规则的 settings key 前缀（赛事级 events.score_rules 优先）。
const (
	scoreRulesKeyPrefix = "ranking.score_rules."
	prizeRulesKeyPrefix = "ranking.prize_rules."
)

// GetScoreRules 返回赛季积分规则（未配置时返回默认值）。
func (s *RankingAdminService) GetScoreRules(ctx context.Context, seasonID int64) (model.ScoreRules, error) {
	raw, _, err := s.stores.Settings.GetWithMeta(ctx, scoreRulesKey(seasonID))
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return model.ScoreRules{}, httpx.ErrInternal(err, "加载积分规则失败，请稍后重试")
	}
	var ptr *string
	if err == nil {
		ptr = &raw
	}
	return resolveScoreRules(ptr), nil
}

// PutScoreRules 保存赛季积分规则。
func (s *RankingAdminService) PutScoreRules(ctx context.Context, adminID, seasonID int64, rules model.ScoreRules) error {
	return s.putRules(ctx, adminID, scoreRulesKey(seasonID), "score_rules", rules)
}

// GetPrizeRules 返回赛季奖金规则（未配置时返回默认值）。
func (s *RankingAdminService) GetPrizeRules(ctx context.Context, seasonID int64) (model.PrizeRules, error) {
	raw, _, err := s.stores.Settings.GetWithMeta(ctx, prizeRulesKey(seasonID))
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return model.PrizeRules{}, httpx.ErrInternal(err, "加载奖金规则失败，请稍后重试")
	}
	var ptr *string
	if err == nil {
		ptr = &raw
	}
	return resolvePrizeRules(ptr), nil
}

// PutPrizeRules 保存赛季奖金规则。
func (s *RankingAdminService) PutPrizeRules(ctx context.Context, adminID, seasonID int64, rules model.PrizeRules) error {
	return s.putRules(ctx, adminID, prizeRulesKey(seasonID), "prize_rules", rules)
}

func (s *RankingAdminService) putRules(ctx context.Context, adminID int64, key, kind string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return httpx.ErrBadRequest(kind + " 不是合法 JSON")
	}
	return s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		if err := s.stores.Settings.Upsert(ctx, tx, key, string(raw)); err != nil {
			return err
		}
		targetType := "season"
		return adminLog(ctx, s.stores, tx, &adminID, "admin.ranking_rules.update",
			&targetType, nil, map[string]any{"key": key})
	})
}

func scoreRulesKey(seasonID int64) string {
	return scoreRulesKeyPrefix + strconv.FormatInt(seasonID, 10)
}
func prizeRulesKey(seasonID int64) string {
	return prizeRulesKeyPrefix + strconv.FormatInt(seasonID, 10)
}

// UpdateLeaderboardRowByKey 按赛季 × 组别 × 舞种定位榜单后更新展示名/隐藏。
func (s *RankingAdminService) UpdateLeaderboardRowByKey(ctx context.Context, adminID, seasonID int64,
	level, danceType string, playerID int64, req model.UpdateLeaderboardRowRequest) (int64, error) {
	if err := validateGroupLevelFilter(level); err != nil {
		return 0, err
	}
	if err := validateDanceTypeFilter(danceType); err != nil {
		return 0, err
	}
	board, err := s.stores.Boards.GetByKey(ctx, seasonID, level, danceType)
	if errors.Is(err, store.ErrNotFound) {
		return 0, httpx.ErrNotFound("榜单不存在")
	}
	if err != nil {
		return 0, httpx.ErrInternal(err, "加载榜单失败，请稍后重试")
	}
	if err := s.UpdateLeaderboardRow(ctx, adminID, board.ID, playerID, req); err != nil {
		return 0, err
	}
	return board.ID, nil
}
