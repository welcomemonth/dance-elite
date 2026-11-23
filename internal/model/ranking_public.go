package model

// PublicRankingConfig 是榜单首页配置（API 文档 6.1）。
type PublicRankingConfig struct {
	Season       string         `json:"season"`
	Title        string         `json:"title"`
	Subtitle     string         `json:"subtitle"`
	Levels       []PublicOption `json:"levels"`
	DanceStyles  []PublicOption `json:"dance_styles"`
	AdvanceQuota int            `json:"advance_quota"`
	UpdatedAt    *string        `json:"updated_at"`
}

// PublicOption 是下拉/切换选项。
type PublicOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// PublicAwardItem 是直通榜选手的奖项罗列。
type PublicAwardItem struct {
	EventName   string `json:"event_name"`
	Award       string `json:"award"`
	PrizeAmount int    `json:"prize_amount"`
}

// PublicDirectEntry 是直通榜条目（API 文档 6.2）。
type PublicDirectEntry struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	AvatarURL string            `json:"avatar_url"`
	Awards    []PublicAwardItem `json:"awards"`
}

// PublicRankingEntry 是积分榜条目（API 文档 6.2）。
type PublicRankingEntry struct {
	ID          int64  `json:"id"`
	Rank        int    `json:"rank"`
	PrevRank    int    `json:"prev_rank"`
	RankChange  int    `json:"rank_change"`
	Name        string `json:"name"`
	AvatarURL   string `json:"avatar_url"`
	TotalPoints int    `json:"total_points"`
	IsDirect    bool   `json:"is_direct"`
}

// PublicRankingBoard 是单个榜单数据（API 文档 6.2）。
type PublicRankingBoard struct {
	BoardID   int64                `json:"board_id"`
	UpdatedAt *string              `json:"updated_at"`
	Direct    []PublicDirectEntry  `json:"direct"`
	Ranking   []PublicRankingEntry `json:"ranking"`
}

// PublicInstitutionEntry 是机构榜条目（API 文档 6.3）。
type PublicInstitutionEntry struct {
	Rank        int     `json:"rank"`
	Name        string  `json:"name"`
	TotalPoints int     `json:"total_points"`
	PlayerCount int     `json:"player_count"`
	Honor       *string `json:"honor"`
}

// PublicInstitutionRanking 是机构年度榜（API 文档 6.3）。
type PublicInstitutionRanking struct {
	UpdatedAt    *string                  `json:"updated_at"`
	Institutions []PublicInstitutionEntry `json:"institutions"`
}

// PublicDancerEvent 是选手积分详情中的一场成绩。
type PublicDancerEvent struct {
	EventName string  `json:"event_name"`
	EventDate *string `json:"event_date"`
	Level     string  `json:"level"`
	Award     string  `json:"award"`
	Points    int     `json:"points"`
}

// PublicDancerBoard 是选手所属榜单。
type PublicDancerBoard struct {
	Level     string `json:"level"`
	DanceType string `json:"dance_type"`
}

// PublicDancerQualification 是晋级情况。
type PublicDancerQualification struct {
	Type   string `json:"type"` // direct / points
	Source string `json:"source"`
}

// PublicDancerDetail 是选手积分详情（API 文档 6.4）。
type PublicDancerDetail struct {
	ID            int64                      `json:"id"`
	Name          string                     `json:"name"`
	AvatarURL     string                     `json:"avatar_url"`
	Board         PublicDancerBoard          `json:"board"`
	Rank          int                        `json:"rank"`
	TotalPoints   int                        `json:"total_points"`
	CountedEvents []PublicDancerEvent        `json:"counted_events"`
	History       []PublicDancerEvent        `json:"history"`
	Honors        []string                   `json:"honors"`
	Qualification *PublicDancerQualification `json:"qualification"`
}
