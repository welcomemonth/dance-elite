package service

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/meetmonth/dance-elite/internal/model"
)

// resolveScoreRules 解析赛事的积分规则：event.score_rules 合法时覆盖默认值。
func resolveScoreRules(raw *string) model.ScoreRules {
	rules := cloneScoreRules(model.DefaultScoreRules)
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return rules
	}
	var parsed model.ScoreRules
	if err := json.Unmarshal([]byte(*raw), &parsed); err != nil {
		return rules
	}
	if len(parsed.BasePoints) > 0 {
		rules.BasePoints = parsed.BasePoints
	}
	if len(parsed.LevelWeights) > 0 {
		rules.LevelWeights = parsed.LevelWeights
	}
	if len(parsed.SizeBands) > 0 {
		rules.SizeBands = parsed.SizeBands
	}
	return rules
}

// resolvePrizeRules 解析赛事的奖金规则：event.prize_rules 合法时覆盖默认值。
func resolvePrizeRules(raw *string) model.PrizeRules {
	rules := model.DefaultPrizeRules
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return rules
	}
	var parsed model.PrizeRules
	if err := json.Unmarshal([]byte(*raw), &parsed); err != nil {
		return rules
	}
	if len(parsed.GradeA.Ranks) > 0 {
		rules.GradeA = parsed.GradeA
	}
	if len(parsed.Super.Ranks) > 0 {
		rules.Super = parsed.Super
	}
	if len(parsed.Final.Amounts) > 0 {
		rules.Final = parsed.Final
	}
	if parsed.InstitutionTop3 > 0 {
		rules.InstitutionTop3 = parsed.InstitutionTop3
	}
	return rules
}

func cloneScoreRules(in model.ScoreRules) model.ScoreRules {
	out := model.ScoreRules{
		BasePoints:   make(map[string]int, len(in.BasePoints)),
		LevelWeights: make(map[string]float64, len(in.LevelWeights)),
		SizeBands:    append([]model.SizeBand(nil), in.SizeBands...),
	}
	for k, v := range in.BasePoints {
		out.BasePoints[k] = v
	}
	for k, v := range in.LevelWeights {
		out.LevelWeights[k] = v
	}
	return out
}

// computePoints 按「名次基础分 × 级别权重 × 人数系数」计算单场积分。
// participants 为该赛事该榜单的参赛人数（用于人数系数分档）。
func computePoints(award, eventLevel string, participants int, rules model.ScoreRules) int {
	base, ok := rules.BasePoints[award]
	if !ok {
		return 0
	}
	weight, ok := rules.LevelWeights[eventLevel]
	if !ok {
		weight = 1
	}
	factor := 1.0
	bands := append([]model.SizeBand(nil), rules.SizeBands...)
	sort.Slice(bands, func(i, j int) bool { return bands[i].MinPlayers > bands[j].MinPlayers })
	for _, band := range bands {
		if participants >= band.MinPlayers {
			factor = band.Factor
			break
		}
	}
	return int(math.Round(float64(base) * weight * factor))
}

// computePrize 按奖金规则计算单个奖项的奖金（分）；不适用返回 0。
func computePrize(award, eventLevel string, participants int, rules model.PrizeRules) int {
	rank, ok := model.AwardRanks[award]
	if !ok {
		return 0
	}
	switch eventLevel {
	case "grade_a":
		return tierPrize(rules.GradeA, rank, participants)
	case "super":
		return tierPrize(rules.Super, rank, participants)
	case "final":
		if rank < 1 || rank > len(rules.Final.Amounts) {
			return 0
		}
		return rules.Final.Amounts[rank-1] * 100
	default:
		return 0
	}
}

func tierPrize(tier model.PrizeTier, rank, participants int) int {
	idx := -1
	for i, r := range tier.Ranks {
		if r == rank {
			idx = i
			break
		}
	}
	if idx < 0 {
		return 0
	}
	amounts := tier.Below
	if tier.Threshold > 0 && participants >= tier.Threshold {
		amounts = tier.Above
	}
	if idx >= len(amounts) {
		return 0
	}
	return amounts[idx] * 100
}

// validateRankingRuleJSON 校验规则 JSON 是否合法（赛事表单保存时用）。
func validateRankingRuleJSON(raw, kind string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return fmt.Errorf("%s 不是合法 JSON: %w", kind, err)
	}
	return nil
}
