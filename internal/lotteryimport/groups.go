package lotteryimport

import (
	"fmt"
	"strings"

	"github.com/meetmonth/dance-elite/internal/model"
)

// genreOrder / ageGroupOrder 保证生成的分组顺序稳定（A/B/C 与组别展示一致）。
var (
	genreOrder    = []string{"classical", "folk"}
	ageGroupOrder = []string{"U11", "U13", "U15"}
)

// lotteryGenreLabels 是舞种枚举到展示名的映射。
var lotteryGenreLabels = map[string]string{
	"classical": "古典舞",
	"folk":      "民族民间舞",
}

// presentGenres 返回当前数据中实际出现过的舞种（按固定顺序），
// 避免页面下拉框出现没有选手的舞种。
func presentGenres(players []model.LotteryPlayer) []model.LotteryGenre {
	present := make(map[string]bool, len(genreOrder))
	for _, p := range players {
		present[p.Genre] = true
	}
	genres := make([]model.LotteryGenre, 0, len(genreOrder))
	for _, key := range genreOrder {
		if present[key] {
			genres = append(genres, model.LotteryGenre{Key: key, Label: lotteryGenreLabels[key]})
		}
	}
	return genres
}

// computeGroups 为每个「组别 × 舞种」按「组数最少、各组均分」切分字母组。
//
// 组数 = ceil(人数/12)，各组人数相差不超过 1，余数从 A 组开始依次 +1。
// 人数不足或无法落在 8-12 人时返回提示，但不阻断导入。
func computeGroups(players []model.LotteryPlayer) ([]model.LotteryGroup, []string) {
	type scope struct{ genre, ageGroup string }
	counts := make(map[scope]int, len(players))
	for _, p := range players {
		counts[scope{p.Genre, p.AgeGroup}]++
	}

	var groups []model.LotteryGroup
	var warnings []string
	for _, genre := range genreOrder {
		for _, ageGroup := range ageGroupOrder {
			n := counts[scope{genre, ageGroup}]
			if n == 0 {
				continue
			}
			sizes := distributeGroupSizes(n)
			for i, size := range sizes {
				letter := groupLetter(i)
				groups = append(groups, model.LotteryGroup{
					Genre:    genre,
					AgeGroup: ageGroup,
					Letter:   letter,
					Size:     size,
				})
			}
			if warning := groupSizeWarning(genre, ageGroup, n, sizes); warning != "" {
				warnings = append(warnings, warning)
			}
		}
	}
	return groups, warnings
}

// distributeGroupSizes 把 n 人切成 ceil(n/12) 组，各组人数差不超过 1。
func distributeGroupSizes(n int) []int {
	if n <= 0 {
		return nil
	}
	groupCount := (n + 11) / 12
	base, remainder := n/groupCount, n%groupCount
	sizes := make([]int, groupCount)
	for i := range sizes {
		sizes[i] = base
		if i < remainder {
			sizes[i]++
		}
	}
	return sizes
}

// groupLetter 把组下标转为 A、B、…、Z、AA 形式的字母。
func groupLetter(index int) string {
	letter := ""
	for index >= 0 {
		letter = string(rune('A'+index%26)) + letter
		index = index/26 - 1
	}
	return letter
}

// groupSizeWarning 在某个「组别 × 舞种」无法落在每组 8-12 人时给出提示。
func groupSizeWarning(genre, ageGroup string, n int, sizes []int) string {
	outOfRange := false
	parts := make([]string, len(sizes))
	for i, size := range sizes {
		parts[i] = fmt.Sprint(size)
		if size < 8 || size > 12 {
			outOfRange = true
		}
	}
	if !outOfRange {
		return ""
	}
	return fmt.Sprintf("%s %s 共 %d 人，按 %d 组分为 %s 人，超出自定每组 8-12 人，请确认数据",
		lotteryGenreLabels[genre], ageGroup, n, len(sizes), strings.Join(parts, "/"))
}
