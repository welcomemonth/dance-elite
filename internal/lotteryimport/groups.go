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

// computeGroups 为每个「组别 × 舞种」切分字母组。
//
// 组数 = max( n/10, n/15 + 1 )，各组人数相差不超过 1，余数从 A 组开始依次 +1。
// 该规则优先按 10 人一组，同时避免小人数被过度拆分。例如：
//
//	14 人 → 1 组（14）
//	15 人 → 2 组（8/7）
//	19 人 → 2 组（10/9）
//	29 人 → 2 组（15/14）
//	31 人 → 3 组（11/10/10）
//	39 人 → 3 组（13/13/13）
//	40 人 → 4 组（10/10/10/10）
//	49 人 → 4 组（13/12/12/12）
//
// 仅当出现少于 7 人的组时给出提示（当前规则下最小为 7 人，基本不会触发）。
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

// distributeGroupSizes 按组数 = max(n/10, n/15+1) 切分，各组人数差不超过 1。
func distributeGroupSizes(n int) []int {
	if n <= 0 {
		return nil
	}
	// 优先按 10 人一组
	groupCount := n / 10
	// 小人数时避免过度拆分：n/15 + 1
	if alt := n/15 + 1; alt > groupCount {
		groupCount = alt
	}
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

// groupSizeWarning 在某个「组别 × 舞种」出现少于 7 人的组时给出提示。
func groupSizeWarning(genre, ageGroup string, n int, sizes []int) string {
	tooSmall := false
	parts := make([]string, len(sizes))
	for i, size := range sizes {
		parts[i] = fmt.Sprint(size)
		if size < 7 {
			tooSmall = true
		}
	}
	if !tooSmall {
		return ""
	}
	return fmt.Sprintf("%s %s 共 %d 人，按 %d 组分为 %s 人，存在少于 7 人的组，请确认数据",
		lotteryGenreLabels[genre], ageGroup, n, len(sizes), strings.Join(parts, "/"))
}
