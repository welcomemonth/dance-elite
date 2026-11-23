package service

import (
	"fmt"
	"strings"

	"github.com/meetmonth/dance-elite/internal/model"
)

// lotteryGenreOrder 是舞种的固定展示顺序。
var lotteryGenreOrder = []string{"classical", "folk"}

// lotteryGenreLabels 是舞种枚举到展示名的映射。
var lotteryGenreLabels = map[string]string{
	"classical": "古典舞",
	"folk":      "民族民间舞",
}

// lotteryGenreLabel 返回舞种展示名；未知枚举原样返回。
func lotteryGenreLabel(genre string) string {
	if label, ok := lotteryGenreLabels[genre]; ok {
		return label
	}
	return genre
}

// lotteryGenresOf 返回选手数据中实际出现过的舞种（按固定顺序）。
func lotteryGenresOf(players []model.LotteryPlayer) []model.LotteryGenre {
	present := make(map[string]bool, len(lotteryGenreOrder))
	for _, player := range players {
		present[player.Genre] = true
	}
	genres := make([]model.LotteryGenre, 0, len(lotteryGenreOrder))
	for _, key := range lotteryGenreOrder {
		if present[key] {
			genres = append(genres, model.LotteryGenre{Key: key, Label: lotteryGenreLabels[key]})
		}
	}
	return genres
}

// lotteryGenreChoices 返回待选舞种（去重，按固定顺序）。
func lotteryGenreChoices(players []model.LotteryPlayer) []model.LotteryGenre {
	pending := make(map[string]bool, len(players))
	for _, player := range players {
		if player.SlotNo == nil {
			pending[player.Genre] = true
		}
	}
	choices := make([]model.LotteryGenre, 0, len(pending))
	for _, key := range lotteryGenreOrder {
		if pending[key] {
			choices = append(choices, model.LotteryGenre{Key: key, Label: lotteryGenreLabels[key]})
		}
	}
	// 兜底：手工数据出现未知舞种时仍给出选项。
	for key := range pending {
		if _, ok := lotteryGenreLabels[key]; !ok {
			choices = append(choices, model.LotteryGenre{Key: key, Label: key})
		}
	}
	return choices
}

// lotterySlotText 把槽位格式化为日志可读的 A3 形式。
func lotterySlotText(slot model.LotterySlot) string {
	return fmt.Sprintf("%s%d", slot.Letter, slot.No)
}

// normalizeLotteryName 去掉姓名中的全部空白，兼容输入时多打的空格。
func normalizeLotteryName(name string) string {
	return strings.Join(strings.Fields(name), "")
}
