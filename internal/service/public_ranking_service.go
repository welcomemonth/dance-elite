package service

import (
	"context"
	"errors"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/storage"
	"github.com/meetmonth/dance-elite/internal/store"
)

// PublicRankingService 是小程序榜单只读接口（API 文档 6.1–6.4）。
type PublicRankingService struct {
	stores *store.Store
}

// NewPublicRankingService 构造小程序榜单服务。
func NewPublicRankingService(stores *store.Store) *PublicRankingService {
	return &PublicRankingService{stores: stores}
}

// Config 返回榜单首页配置（当前 active 赛季）。
func (s *PublicRankingService) Config(ctx context.Context) (*model.PublicRankingConfig, error) {
	season, err := s.stores.Seasons.GetActive(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("暂无进行中的赛季")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载赛季失败，请稍后重试")
	}
	updated, err := s.stores.Points.MaxComputedAt(ctx, season.ID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载榜单失败，请稍后重试")
	}
	cfg := &model.PublicRankingConfig{
		Season:       season.Name,
		Title:        "国舞菁英独舞冠军赛",
		Subtitle:     "直通榜 · 积分榜",
		AdvanceQuota: advanceQuota,
		UpdatedAt:    updated,
		Levels: []model.PublicOption{
			{Value: "U11", Label: "U11"},
			{Value: "U13", Label: "U13"},
			{Value: "U15", Label: "U15"},
		},
		DanceStyles: []model.PublicOption{
			{Value: "classical", Label: "古典舞"},
			{Value: "folk", Label: "民族民间舞"},
		},
	}
	return cfg, nil
}

// Ranking 返回指定榜单的直通榜与积分榜（API 文档 6.2）。
func (s *PublicRankingService) Ranking(ctx context.Context, level, dance string) (*model.PublicRankingBoard, error) {
	if err := validateGroupLevelFilter(level); err != nil {
		return nil, err
	}
	if err := validateDanceTypeFilter(dance); err != nil {
		return nil, err
	}
	season, err := s.stores.Seasons.GetActive(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("暂无进行中的赛季")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载赛季失败，请稍后重试")
	}
	board, err := s.stores.Boards.GetByKey(ctx, season.ID, level, dance)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("榜单不存在")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载榜单失败，请稍后重试")
	}
	points, err := s.stores.Points.ListPlayerPointsByBoard(ctx, board.ID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载积分失败，请稍后重试")
	}
	seasonResults, err := s.stores.Results.ListPublishedBySeasonRead(ctx, season.ID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载成绩失败，请稍后重试")
	}
	awardsOf := map[int64][]model.PublicAwardItem{}
	for _, r := range seasonResults {
		if r.Result.BoardID != board.ID {
			continue
		}
		awardsOf[r.Result.PlayerID] = append(awardsOf[r.Result.PlayerID], model.PublicAwardItem{
			EventName: r.EventName, Award: r.Result.Award, PrizeAmount: r.Result.PrizeAmount,
		})
	}
	quals, err := s.stores.Qualifications.ListValidByBoard(ctx, board.ID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载资格失败，请稍后重试")
	}
	isDirect := map[int64]string{}
	for _, q := range quals {
		switch q.Qualification.Source {
		case model.QualificationAutoDirect, model.QualificationManualDirect:
			isDirect[q.Qualification.PlayerID] = stringOr(q.Qualification.Reason, "直通")
		}
	}

	out := &model.PublicRankingBoard{
		BoardID: board.ID, UpdatedAt: board.PublishedAt,
		Direct: make([]model.PublicDirectEntry, 0), Ranking: make([]model.PublicRankingEntry, 0),
	}
	for _, p := range points {
		if p.PlayerPoint.Hidden {
			continue
		}
		entryID := p.PlayerPoint.ID
		name := displayName(p.PlayerPoint.DisplayName, p.PlayerName)
		avatar := avatarURL(p.AvatarPath)
		if _, ok := isDirect[p.PlayerPoint.PlayerID]; ok {
			out.Direct = append(out.Direct, model.PublicDirectEntry{
				ID: entryID, Name: name, AvatarURL: avatar,
				Awards: nonNilAwards(awardsOf[p.PlayerPoint.PlayerID]),
			})
		}
		if p.PlayerPoint.Rank == nil {
			continue
		}
		out.Ranking = append(out.Ranking, model.PublicRankingEntry{
			ID: entryID, Rank: *p.PlayerPoint.Rank,
			PrevRank: *p.PlayerPoint.Rank, RankChange: 0,
			Name: name, AvatarURL: avatar,
			TotalPoints: p.PlayerPoint.TotalPoints,
			IsDirect:    isDirect[p.PlayerPoint.PlayerID] != "",
		})
	}
	return out, nil
}

// InstitutionRanking 返回机构年度榜（API 文档 6.3）。
func (s *PublicRankingService) InstitutionRanking(ctx context.Context) (*model.PublicInstitutionRanking, error) {
	season, err := s.stores.Seasons.GetActive(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("暂无进行中的赛季")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载赛季失败，请稍后重试")
	}
	rows, err := s.stores.Points.ListInstitutionPoints(ctx, season.ID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载机构榜失败，请稍后重试")
	}
	updated, err := s.stores.Points.MaxComputedAt(ctx, season.ID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载榜单失败，请稍后重试")
	}
	out := &model.PublicInstitutionRanking{UpdatedAt: updated, Institutions: make([]model.PublicInstitutionEntry, 0, len(rows))}
	for _, r := range rows {
		if r.Rank == nil {
			continue
		}
		var honor *string
		switch {
		case *r.Rank >= 1 && *r.Rank <= 3:
			h := "elite_3"
			honor = &h
		case *r.Rank >= 4 && *r.Rank <= 10:
			h := "top_10"
			honor = &h
		}
		out.Institutions = append(out.Institutions, model.PublicInstitutionEntry{
			Rank: *r.Rank, Name: r.InstitutionName, TotalPoints: r.TotalPoints,
			PlayerCount: r.PlayerCount, Honor: honor,
		})
	}
	return out, nil
}

// DancerDetail 返回选手积分详情（API 文档 6.4）。
func (s *PublicRankingService) DancerDetail(ctx context.Context, entryID int64) (*model.PublicDancerDetail, error) {
	pp, err := s.stores.Points.GetPlayerPointByID(ctx, entryID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("条目不存在")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载积分失败，请稍后重试")
	}
	if pp.PlayerPoint.Hidden {
		return nil, httpx.ErrNotFound("条目不存在")
	}
	board, err := s.stores.Boards.Get(ctx, pp.PlayerPoint.BoardID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载榜单失败，请稍后重试")
	}
	results, err := s.stores.Results.ListPublishedByPlayer(ctx, pp.PlayerPoint.SeasonID, pp.PlayerPoint.PlayerID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载成绩失败，请稍后重试")
	}
	used := map[int64]bool{}
	for _, id := range parseEventIDs(pp.PlayerPoint.UsedEventIDs) {
		used[id] = true
	}
	history := make([]model.PublicDancerEvent, 0, len(results))
	counted := make([]model.PublicDancerEvent, 0, 3)
	honors := make([]string, 0, len(results))
	for _, r := range results {
		if r.Result.BoardID != board.ID {
			continue
		}
		ev := model.PublicDancerEvent{
			EventName: r.EventName, EventDate: r.EventDate, Level: r.EventLevel,
			Award: r.Result.Award, Points: r.Result.Points,
		}
		history = append(history, ev)
		if used[r.Result.EventID] {
			counted = append(counted, ev)
		}
		honors = append(honors, r.Result.Award)
	}

	detail := &model.PublicDancerDetail{
		ID: entryID, Name: displayName(pp.PlayerPoint.DisplayName, pp.PlayerName),
		AvatarURL:     avatarURL(pp.AvatarPath),
		Board:         model.PublicDancerBoard{Level: board.Level, DanceType: board.DanceType},
		TotalPoints:   pp.PlayerPoint.TotalPoints,
		CountedEvents: counted, History: history, Honors: honors,
	}
	if pp.PlayerPoint.Rank != nil {
		detail.Rank = *pp.PlayerPoint.Rank
	}
	quals, err := s.stores.Qualifications.ListValidByBoard(ctx, board.ID)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载资格失败，请稍后重试")
	}
	for _, q := range quals {
		if q.Qualification.PlayerID != pp.PlayerPoint.PlayerID {
			continue
		}
		typ := "points"
		if q.Qualification.Source == model.QualificationAutoDirect ||
			q.Qualification.Source == model.QualificationManualDirect {
			typ = "direct"
		}
		detail.Qualification = &model.PublicDancerQualification{
			Type: typ, Source: stringOr(q.Qualification.Reason, ""),
		}
		break
	}
	return detail, nil
}

func displayName(display *string, real string) string {
	if display != nil && *display != "" {
		return *display
	}
	return real
}

func avatarURL(path *string) string {
	if path == nil {
		return ""
	}
	return storage.PublicURL(*path)
}

func nonNilAwards(in []model.PublicAwardItem) []model.PublicAwardItem {
	if in == nil {
		return []model.PublicAwardItem{}
	}
	return in
}

func stringOr(s *string, fallback string) string {
	if s != nil && *s != "" {
		return *s
	}
	return fallback
}
