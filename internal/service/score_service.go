package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

const (
	adminActionScorePublish = "admin.score.publish"
	adminActionScoreCorrect = "admin.score.correct"
)

// ScoreService 负责成绩录入、发布与纠错（T28）。
type ScoreService struct {
	stores  *store.Store
	players *PlayerService
	now     func() time.Time
}

// NewScoreService 构造成绩服务。
func NewScoreService(stores *store.Store, players *PlayerService) *ScoreService {
	return &ScoreService{stores: stores, players: players, now: time.Now}
}

// boardContext 解析赛事与榜单，并校验组别/舞种属于该赛季。
func (s *ScoreService) boardContext(ctx context.Context, eventID int64, level, dance string) (*model.Event, *model.Board, error) {
	if err := validateGroupLevelFilter(level); err != nil {
		return nil, nil, err
	}
	if err := validateDanceTypeFilter(dance); err != nil {
		return nil, nil, err
	}
	event, err := s.stores.Events.Get(ctx, eventID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil, httpx.ErrNotFound("赛事不存在")
	}
	if err != nil {
		return nil, nil, httpx.ErrInternal(err, "加载赛事失败，请稍后重试")
	}
	board, err := s.stores.Boards.GetByKey(ctx, event.SeasonID, level, dance)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil, httpx.ErrNotFound("榜单不存在")
	}
	if err != nil {
		return nil, nil, httpx.ErrInternal(err, "加载榜单失败，请稍后重试")
	}
	return event, board, nil
}

// List 返回赛事某榜单已发布成绩。
func (s *ScoreService) List(ctx context.Context, eventID int64, level, dance string) ([]model.AdminScoreEntry, error) {
	_, board, err := s.boardContext(ctx, eventID, level, dance)
	if err != nil {
		return nil, err
	}
	rows, err := s.stores.Results.ListByEventBoard(ctx, eventID, board.ID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载成绩失败，请稍后重试")
	}
	out := make([]model.AdminScoreEntry, 0, len(rows))
	for _, r := range rows {
		if r.Result.Status != model.ResultStatusPublished {
			continue
		}
		award := r.Result.Award
		out = append(out, model.AdminScoreEntry{
			EntryID: r.Result.ID, PlayerID: r.Result.PlayerID,
			PlayerNo: r.PlayerNo, PlayerName: r.PlayerName,
			Award: &award, AdvancedToFinal: r.Result.AdvancedToFinal,
			Points: r.Result.Points, Remark: r.Result.Remark, Published: true,
		})
	}
	return out, nil
}

// Preview 解析成绩行并按证件号/姓名匹配选手（不写库）。
func (s *ScoreService) Preview(ctx context.Context, eventID int64, level, dance string, rows []model.ScorePreviewInputRow) (*model.ScorePreviewResponse, error) {
	event, board, err := s.boardContext(ctx, eventID, level, dance)
	if err != nil {
		return nil, err
	}
	_ = board
	players, err := s.stores.Registrations.ListEventPlayers(ctx, eventID, level, dance)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载报名选手失败，请稍后重试")
	}
	resp := &model.ScorePreviewResponse{TotalRows: len(rows), Rows: make([]model.ScorePreviewRowResult, 0, len(rows))}
	for i, row := range rows {
		item := model.ScorePreviewRowResult{
			RowNo: row.RowNo, PlayerName: strings.TrimSpace(row.PlayerName),
			AdvancedToFinal: row.AdvancedToFinal, MatchStatus: "unmatched",
		}
		if item.RowNo <= 0 {
			item.RowNo = i + 1
		}
		if row.Award != "" {
			a := normalizeAward(row.Award)
			item.Award = &a
		}
		if row.Remark != "" {
			item.Remark = strPtr(row.Remark)
		}
		if row.IDCardNo != "" {
			item.IDCardMasked = strPtr(utils.MaskIDNumber(row.IDCardNo))
			if pid := s.matchByIDNumber(ctx, event, row.IDCardNo); pid > 0 {
				item.PlayerID = &pid
				item.MatchStatus = "matched"
			}
		} else if pid := matchByName(players, item.PlayerName); pid > 0 {
			item.PlayerID = &pid
			item.MatchStatus = "matched"
		}
		if item.MatchStatus == "matched" {
			resp.Matched++
		} else {
			resp.Unmatched++
		}
		resp.Rows = append(resp.Rows, item)
	}
	resp.Errors = resp.Unmatched
	return resp, nil
}

// matchByIDNumber 按证件号哈希匹配已有选手；未命中返回 0（预览不建档）。
// 依次尝试身份证与护照哈希，兼容成绩导入模板中的任意证件号。
func (s *ScoreService) matchByIDNumber(ctx context.Context, event *model.Event, raw string) int64 {
	_ = event
	for _, idType := range []string{utils.IDTypeIDCard, utils.IDTypePassport} {
		hash := s.players.HashIDNumber(idType, raw)
		if hash == "" {
			continue
		}
		player, err := s.stores.Players.GetByHash(ctx, hash)
		if err == nil && player != nil {
			return player.ID
		}
	}
	return 0
}

func matchByName(players []store.EventPlayerRow, name string) int64 {
	var hit int64
	count := 0
	for _, p := range players {
		if p.PlayerName == name {
			hit = p.PlayerID
			count++
		}
	}
	if count == 1 {
		return hit
	}
	return 0
}

// Publish 发布成绩：写成绩 → 固化积分/奖金 → 重算快照与资格。
func (s *ScoreService) Publish(ctx context.Context, adminID, eventID int64, level, dance string,
	rows []model.ScorePreviewRowResult, importBatch string) (*model.PublishScoresResponse, error) {
	event, board, err := s.boardContext(ctx, eventID, level, dance)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if (r.MatchStatus != "matched" && r.MatchStatus != "created") || r.PlayerID == nil {
			return nil, httpx.ErrBadRequest("存在未匹配/未建档的成绩行，请修正后重新预览")
		}
	}
	scoreRules := resolveScoreRules(event.ScoreRules)
	prizeRules := resolvePrizeRules(event.PrizeRules)
	institutions, err := s.institutionMap(ctx, eventID, level, dance)
	if err != nil {
		return nil, err
	}

	published := 0
	now := s.now()
	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		participants := 0
		for _, r := range rows {
			if isPublishableScoreRow(r) {
				participants++
			}
		}
		prizes := make([]model.Prize, 0, len(rows))
		for _, r := range rows {
			if !isPublishableScoreRow(r) {
				continue
			}
			award := ""
			if r.Award != nil {
				award = normalizeAward(*r.Award)
			}
			if !model.IsValidAward(award) {
				return httpx.ErrBadRequest("奖项不合法")
			}
			points := computePoints(award, event.Level, participants, scoreRules)
			prize := 0
			if !event.ExcludeFromPrize {
				prize = computePrize(award, event.Level, participants, prizeRules)
			}
			params := store.ResultParams{
				EventID: eventID, BoardID: board.ID, PlayerID: *r.PlayerID,
				Institution: institutions[*r.PlayerID], Award: award, Points: points,
				PrizeAmount: prize, AdvancedToFinal: r.AdvancedToFinal, Remark: r.Remark,
				ImportBatch: strPtrOrNil(importBatch), CreatedBy: &adminID,
			}
			existing, err := s.stores.Results.GetPublishedByPlayer(ctx, tx, eventID, board.ID, *r.PlayerID)
			if err != nil {
				return err
			}
			if existing != nil {
				if err := s.stores.Results.UpdatePublished(ctx, tx, existing.ID, params); err != nil {
					return err
				}
			} else if _, err := s.stores.Results.Insert(ctx, tx, params); err != nil {
				return err
			}
			published++
			if prize > 0 {
				institution := ""
				if params.Institution != nil {
					institution = *params.Institution
				}
				prizes = append(prizes, model.Prize{
					SeasonID: event.SeasonID, EventID: &eventID, BoardID: &board.ID,
					PlayerID: r.PlayerID, Award: award, Amount: prize,
					InstitutionName: strPtrOrNil(institution),
				})
			}
		}
		if err := s.stores.Prizes.ReplacePendingForEventBoard(ctx, tx, eventID, board.ID, prizes); err != nil {
			return err
		}
		if err := s.stores.Boards.MarkPublished(ctx, tx, board.ID, utils.FormatISO(now)); err != nil {
			return err
		}
		if err := s.recompute(ctx, tx, event.SeasonID, board.ID, now); err != nil {
			return err
		}
		targetType := "board"
		return adminLog(ctx, s.stores, tx, &adminID, adminActionScorePublish,
			&targetType, &board.ID, map[string]any{
				"event_id": eventID, "level": level, "dance_type": dance,
				"published": published, "import_batch": importBatch,
			})
	})
	if err != nil {
		return nil, err
	}
	return &model.PublishScoresResponse{Published: published}, nil
}

// Correct 纠错：旧成绩置 corrected，写入新成绩并重算。
func (s *ScoreService) Correct(ctx context.Context, adminID, entryID int64, req model.CorrectScoreRequest) (int64, error) {
	old, err := s.stores.Results.GetPublished(ctx, entryID)
	if errors.Is(err, store.ErrNotFound) {
		return 0, httpx.ErrNotFound("成绩不存在或已纠错")
	}
	if err != nil {
		return 0, httpx.ErrInternal(err, "加载成绩失败，请稍后重试")
	}
	event, err := s.stores.Events.Get(ctx, old.EventID)
	if err != nil {
		return 0, httpx.ErrInternal(err, "加载赛事失败，请稍后重试")
	}
	board, err := s.stores.Boards.Get(ctx, old.BoardID)
	if err != nil {
		return 0, httpx.ErrInternal(err, "加载榜单失败，请稍后重试")
	}

	award := old.Award
	if req.Award != "" {
		award = normalizeAward(req.Award)
	}
	if !model.IsValidAward(award) {
		return 0, httpx.ErrBadRequest("奖项不合法")
	}
	advanced := old.AdvancedToFinal
	if req.AdvancedToFinal != nil {
		advanced = *req.AdvancedToFinal
	}
	remark := old.Remark
	if req.Remark != "" {
		remark = strPtr(req.Remark)
	}

	scoreRules := resolveScoreRules(event.ScoreRules)
	prizeRules := resolvePrizeRules(event.PrizeRules)
	var newID int64
	now := s.now()
	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		participants, err := s.stores.Results.CountPublishedByEventBoard(ctx, tx, old.EventID, old.BoardID)
		if err != nil {
			return err
		}
		if participants < 1 {
			participants = 1
		}
		if err := s.stores.Results.MarkCorrected(ctx, tx, entryID); err != nil {
			return err
		}
		points := computePoints(award, event.Level, participants, scoreRules)
		prize := 0
		if !event.ExcludeFromPrize {
			prize = computePrize(award, event.Level, participants, prizeRules)
		}
		newID, err = s.stores.Results.Insert(ctx, tx, store.ResultParams{
			EventID: old.EventID, BoardID: old.BoardID, PlayerID: old.PlayerID,
			Institution: old.Institution, Award: award, Points: points,
			PrizeAmount: prize, AdvancedToFinal: advanced, Remark: remark,
			CreatedBy: &adminID,
		})
		if err != nil {
			return err
		}
		if err := s.recompute(ctx, tx, event.SeasonID, board.ID, now); err != nil {
			return err
		}
		if err := s.rebuildBoardPrizes(ctx, tx, event, board.ID, prizeRules); err != nil {
			return err
		}
		targetType := "result"
		return adminLog(ctx, s.stores, tx, &adminID, adminActionScoreCorrect,
			&targetType, &newID, map[string]any{
				"old_result_id": entryID, "award": award, "points": points,
			})
	})
	if err != nil {
		return 0, err
	}
	return newID, nil
}

// rebuildBoardPrizes 按赛事某榜单当前有效成绩重建待发放奖金（纠错后同步奖金明细）。
func (s *ScoreService) rebuildBoardPrizes(ctx context.Context, tx *sql.Tx, event *model.Event,
	boardID int64, rules model.PrizeRules) error {
	rows, err := s.stores.Results.ListPublishedByEventBoard(ctx, tx, event.ID, boardID)
	if err != nil {
		return err
	}
	prizes := make([]model.Prize, 0, len(rows))
	if !event.ExcludeFromPrize {
		for _, r := range rows {
			amount := computePrize(r.Result.Award, event.Level, len(rows), rules)
			if amount <= 0 {
				continue
			}
			pid := r.Result.PlayerID
			prizes = append(prizes, model.Prize{
				SeasonID: event.SeasonID, EventID: &event.ID, BoardID: &boardID,
				PlayerID: &pid, Award: r.Result.Award, Amount: amount,
				InstitutionName: r.Result.Institution,
			})
		}
	}
	return s.stores.Prizes.ReplacePendingForEventBoard(ctx, tx, event.ID, boardID, prizes)
}

// recompute 重算榜单积分、资格与赛季机构榜（须在事务内调用）。
func (s *ScoreService) recompute(ctx context.Context, tx *sql.Tx, seasonID, boardID int64, now time.Time) error {
	if err := recomputeBoardPoints(ctx, s.stores, tx, seasonID, boardID, now); err != nil {
		return err
	}
	boards, err := s.stores.Boards.ListBySeasonTx(ctx, tx, seasonID)
	if err != nil {
		return err
	}
	return recomputeInstitutionPoints(ctx, s.stores, tx, seasonID, boards, now)
}

// institutionMap 返回报名单元内选手的机构（成绩录入时写入）。
func (s *ScoreService) institutionMap(ctx context.Context, eventID int64, level, dance string) (map[int64]*string, error) {
	players, err := s.stores.Registrations.ListEventPlayers(ctx, eventID, level, dance)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载报名选手失败，请稍后重试")
	}
	out := make(map[int64]*string, len(players))
	for _, p := range players {
		out[p.PlayerID] = p.Institution
	}
	return out, nil
}

// normalizeAward 兼容前端历史写法 third_place → third。
func normalizeAward(award string) string {
	if award == "third_place" {
		return model.AwardThird
	}
	return award
}

// isPublishableScoreRow 判断预览行是否已有确定的选手档案（含导入时新建）。
func isPublishableScoreRow(r model.ScorePreviewRowResult) bool {
	return (r.MatchStatus == "matched" || r.MatchStatus == "created") && r.PlayerID != nil
}

func strPtrOrNil(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
