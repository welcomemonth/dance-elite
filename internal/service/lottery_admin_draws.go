package service

import (
	"context"
	"sort"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// buildDraws 把已抽签选手转换为后台明细，并用 unionid 关联小程序账号。
func (s *AdminLotteryService) buildDraws(ctx context.Context, players []model.LotteryPlayer) ([]model.AdminLotteryDraw, error) {
	draws := make([]model.AdminLotteryDraw, 0, len(players))
	unionIDs := make([]string, 0, len(players))
	for _, player := range players {
		if player.SlotNo == nil || player.GroupLetter == nil {
			continue
		}
		draw := model.AdminLotteryDraw{
			PlayerName: player.Name,
			Genre:      player.Genre,
			GenreLabel: lotteryGenreLabel(player.Genre),
			AgeGroup:   player.AgeGroup,
			Slot:       player.Slot(),
			AutoDrawn:  player.AutoDrawn,
			IDCardHash: player.IDCardHash,
		}
		if player.DrawnAt != nil {
			draw.DrawnAt = *player.DrawnAt
		}
		if player.DrawnOpenID != nil {
			draw.OpenID = *player.DrawnOpenID
		}
		if player.DrawnUnionID != nil {
			draw.UnionID = *player.DrawnUnionID
			unionIDs = append(unionIDs, *player.DrawnUnionID)
		}
		draws = append(draws, draw)
	}
	if len(unionIDs) > 0 {
		users, err := s.stores.Users.ByUnionIDs(ctx, unionIDs)
		if err != nil {
			return nil, httpx.ErrInternal(err, "读取抽签微信账号失败，请稍后重试")
		}
		for i := range draws {
			if draws[i].UnionID == "" {
				continue
			}
			user, ok := users[draws[i].UnionID]
			if !ok {
				continue
			}
			matched := &model.AdminLotteryDrawUser{ID: user.ID}
			if user.Nickname != nil {
				matched.Nickname = *user.Nickname
			}
			if user.Phone != nil {
				matched.Phone = utils.MaskPhone(*user.Phone)
			}
			draws[i].User = matched
		}
	}
	markSameWechatCounts(draws)
	sort.SliceStable(draws, func(i, j int) bool {
		if draws[i].DrawnAt == draws[j].DrawnAt {
			return draws[i].PlayerName < draws[j].PlayerName
		}
		return draws[i].DrawnAt < draws[j].DrawnAt
	})
	return draws, nil
}

// markSameWechatCounts 统计同一 openid 在本场操作的选手人数（按证件号哈希去重），
// 便于发现一个微信替多人抽签；同一选手多个舞种只计 1 人。
func markSameWechatCounts(draws []model.AdminLotteryDraw) {
	players := make(map[string]map[string]struct{}, len(draws))
	for _, draw := range draws {
		if draw.OpenID == "" {
			continue
		}
		playerKey := draw.IDCardHash
		if playerKey == "" {
			playerKey = draw.PlayerName
		}
		if players[draw.OpenID] == nil {
			players[draw.OpenID] = make(map[string]struct{})
		}
		players[draw.OpenID][playerKey] = struct{}{}
	}
	for i := range draws {
		if draws[i].OpenID != "" {
			draws[i].SameWechatCount = len(players[draws[i].OpenID])
		}
	}
}
