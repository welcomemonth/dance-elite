package lotteryimport

import (
	"fmt"
	"sort"
	"strings"

	"github.com/meetmonth/dance-elite/internal/model"
)

// ValidationError 汇总一次导入中的全部数据问题，便于一次性修完 Excel。
type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	return "导入校验未通过：\n  - " + strings.Join(e.Problems, "\n  - ")
}

// Build 读取报名 Excel 并生成抽签数据。
//
// 只依赖表头名称定位列，列顺序变化不影响导入。任何一条数据不合法都会
// 返回 *ValidationError（列出全部问题与行号），不会生成半份数据。
// 返回值 warnings 描述分组人数超出 8-12 人的情况，不阻断导入。
func Build(excelPath, eventName string) (*model.LotteryData, []string, error) {
	rows, err := readRows(excelPath)
	if err != nil {
		return nil, nil, err
	}
	cols, err := mapColumns(rows[0])
	if err != nil {
		return nil, nil, err
	}

	var parsed []row
	var problems []string
	for i := 1; i < len(rows); i++ {
		if isBlankRow(rows[i], cols) {
			continue
		}
		r, err := parseRow(rows[i], cols, i+1)
		if err != nil {
			problems = append(problems, fmt.Sprintf("第 %d 行：%v", i+1, err))
			continue
		}
		parsed = append(parsed, r)
	}
	if len(parsed) == 0 && len(problems) == 0 {
		return nil, nil, fmt.Errorf("Excel 没有有效的报名数据")
	}
	problems = append(problems, duplicateProblems(parsed)...)
	if len(problems) > 0 {
		return nil, nil, &ValidationError{Problems: problems}
	}

	players := make([]model.LotteryPlayer, 0, len(parsed))
	for _, r := range parsed {
		players = append(players, model.LotteryPlayer{
			Name:        r.name,
			IDCard:      r.idCard,
			Genre:       r.genre,
			AgeGroup:    r.ageGroup,
			Institution: r.institution,
			Program:     r.program,
			Phone:       r.phone,
		})
	}
	groups, groupWarnings := computeGroups(players)
	warnings := phoneWarnings(parsed)
	warnings = append(warnings, groupWarnings...)
	if eventName == "" {
		eventName = "优舞盟赛事抽签"
	}
	return &model.LotteryData{
		Version:   2,
		EventName: eventName,
		Genres:    presentGenres(players),
		Groups:    groups,
		Players:   players,
	}, warnings, nil
}

// phoneWarnings 汇总手机号不合法等提示（带 Excel 行号）。
func phoneWarnings(rows []row) []string {
	var warnings []string
	for _, r := range rows {
		if r.phoneWarning != "" {
			warnings = append(warnings, fmt.Sprintf("第 %d 行：%s", r.excelRow, r.phoneWarning))
		}
	}
	return warnings
}

// duplicateProblems 检查「身份证号 + 舞种」是否重复（同一人同一舞种只能报名一次）。
// 同一身份证号报不同舞种是允许的，不在此列。
func duplicateProblems(rows []row) []string {
	type key struct{ idCard, genre string }
	lines := make(map[key][]int, len(rows))
	for _, r := range rows {
		k := key{r.idCard, r.genre}
		lines[k] = append(lines[k], r.excelRow)
	}

	var problems []string
	for k, rowsOfKey := range lines {
		if len(rowsOfKey) < 2 {
			continue
		}
		sort.Ints(rowsOfKey)
		parts := make([]string, len(rowsOfKey))
		for i, n := range rowsOfKey {
			parts[i] = fmt.Sprintf("第 %d 行", n)
		}
		problems = append(problems, fmt.Sprintf("身份证号 %s 在%s中重复出现（%s）",
			k.idCard, lotteryGenreLabels[k.genre], strings.Join(parts, "、")))
	}
	sort.Strings(problems)
	return problems
}
