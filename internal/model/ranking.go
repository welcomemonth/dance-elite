package model

// 奖项英文枚举键（results.award，展示层转中文）。
const (
	AwardChampion    = "champion"     // 冠军
	AwardRunnerUp    = "runner_up"    // 亚军
	AwardThird       = "third"        // 季军
	AwardEliteDancer = "elite_dancer" // 菁英舞者
	AwardSpecialGold = "special_gold" // 特金奖
	AwardGold        = "gold"         // 金奖
	AwardSilver      = "silver"       // 银奖
)

// AwardOptions 是奖项枚举顺序（用于校验与默认分值表）。
var AwardOptions = []string{
	AwardChampion, AwardRunnerUp, AwardThird, AwardEliteDancer,
	AwardSpecialGold, AwardGold, AwardSilver,
}

// AwardRanks 是奖项对应的名次（用于直通与奖金规则）。
var AwardRanks = map[string]int{
	AwardChampion: 1, AwardRunnerUp: 2, AwardThird: 3,
}

// IsValidAward 判断奖项是否合法。
func IsValidAward(award string) bool {
	return AwardRanks[award] > 0 || award == AwardEliteDancer ||
		award == AwardSpecialGold || award == AwardGold || award == AwardSilver
}

// 成绩记录状态（results.status）。
const (
	ResultStatusPublished = "published"
	ResultStatusCorrected = "corrected"
	ResultStatusVoided    = "voided"
)

// 总决赛资格来源（final_qualifications.source）。
const (
	QualificationAutoDirect   = "auto_direct"
	QualificationManualDirect = "manual_direct"
	QualificationPoints       = "points"
	QualificationManual       = "manual"
)

// Board 是榜单（赛季 × 组别 × 舞种）。
type Board struct {
	ID          int64
	SeasonID    int64
	Level       string
	DanceType   string
	Status      string
	PublishedAt *string
	CreatedAt   string
	UpdatedAt   string
}

// Result 是成绩记录（赛事 × 榜单 × 选手）。
type Result struct {
	ID              int64
	EventID         int64
	BoardID         int64
	PlayerID        int64
	Institution     *string
	Award           string
	Points          int
	PrizeAmount     int
	AdvancedToFinal bool
	Remark          *string
	Status          string
	ImportBatch     *string
	CreatedBy       *int64
	CreatedAt       string
	UpdatedAt       string
}

// PlayerPoint 是选手积分快照。
type PlayerPoint struct {
	ID           int64
	SeasonID     int64
	BoardID      int64
	PlayerID     int64
	TotalPoints  int
	UsedEventIDs string
	Rank         *int
	DisplayName  *string
	Hidden       bool
	ComputedAt   *string
}

// InstitutionPoint 是机构年度榜快照。
type InstitutionPoint struct {
	ID              int64
	SeasonID        int64
	InstitutionName string
	TotalPoints     int
	PlayerCount     int
	Rank            *int
	ComputedAt      *string
}

// FinalQualification 是总决赛资格记录。
type FinalQualification struct {
	ID        int64
	SeasonID  int64
	BoardID   int64
	PlayerID  int64
	Source    string
	Reason    *string
	Status    string
	CreatedBy *int64
	CreatedAt string
	UpdatedAt string
}

// Prize 是奖金明细。
type Prize struct {
	ID              int64
	SeasonID        int64
	EventID         *int64
	BoardID         *int64
	PlayerID        *int64
	InstitutionName *string
	Award           string
	Amount          int
	Status          string
	GrantedAt       *string
	GranteeNote     *string
	CreatedAt       string
	UpdatedAt       string
}

// DefaultScoreRules 是默认积分规则（FAQ：名次基础分 × 级别权重 × 人数系数）。
var DefaultScoreRules = ScoreRules{
	BasePoints: map[string]int{
		AwardChampion: 100, AwardRunnerUp: 85, AwardThird: 75,
		AwardEliteDancer: 65, AwardSpecialGold: 55, AwardGold: 45, AwardSilver: 30,
	},
	LevelWeights: map[string]float64{"grade_a": 1.0, "super": 1.2, "final": 1.5},
	SizeBands: []SizeBand{
		{MinPlayers: 40, Factor: 1.2},
		{MinPlayers: 20, Factor: 1.0},
		{MinPlayers: 0, Factor: 0.8},
	},
}

// ScoreRules 是积分规则（可被 events.score_rules 覆盖）。
type ScoreRules struct {
	BasePoints   map[string]int     `json:"base_points"`
	LevelWeights map[string]float64 `json:"level_weights"`
	SizeBands    []SizeBand         `json:"size_bands"`
}

// SizeBand 是按参赛人数分档的系数。
type SizeBand struct {
	MinPlayers int     `json:"min_players"`
	Factor     float64 `json:"factor"`
}

// DefaultPrizeRules 是默认奖金规则（FAQ：按参赛人数分档）。
var DefaultPrizeRules = PrizeRules{
	GradeA:          PrizeTier{Ranks: []int{1}, Threshold: 40, Above: []int{200000}, Below: []int{100000}},
	Super:           PrizeTier{Ranks: []int{1, 2}, Threshold: 40, Above: []int{300000, 150000}, Below: []int{200000, 100000}},
	Final:           PrizeFinal{Amounts: []int{1000000, 300000, 200000, 100000, 100000, 100000, 100000, 100000}},
	InstitutionTop3: 1000000,
}

// PrizeRules 是奖金规则（可被 events.prize_rules 覆盖）。
type PrizeRules struct {
	GradeA          PrizeTier  `json:"grade_a"`
	Super           PrizeTier  `json:"super"`
	Final           PrizeFinal `json:"final"`
	InstitutionTop3 int        `json:"institution_top3"`
}

// PrizeTier 是甲级/超级赛奖金分档（金额单位：元）。
type PrizeTier struct {
	Ranks     []int `json:"ranks"`
	Threshold int   `json:"threshold"`
	Above     []int `json:"above"`
	Below     []int `json:"below"`
}

// PrizeFinal 是总决赛奖金（金额单位：元）。
type PrizeFinal struct {
	Amounts []int `json:"amounts"`
}
