package model

// AdminWxUserItem 是后台用户列表单条记录（API 文档 13.1）。
// 手机号只输出脱敏形式（MaskPhone），证件号不落用户维度。
type AdminWxUserItem struct {
	ID                int64   `json:"id"`
	Nickname          string  `json:"nickname"`
	AvatarURL         string  `json:"avatar_url"`
	PhoneMasked       string  `json:"phone_masked"`
	RegistrationCount int64   `json:"registration_count"`
	Status            string  `json:"status"`
	CreatedAt         string  `json:"created_at"`
	LastLoginAt       *string `json:"last_login_at"`
}

// AdminWxUserListResponse 是后台用户列表响应（统一分页结构）。
type AdminWxUserListResponse struct {
	List     []AdminWxUserItem `json:"list"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// AdminPlayerBriefItem 是后台选手档案列表单条记录（API 文档 13.2）。
// 证件号只输出脱敏形态；不存在"明文查看证件号"功能（哈希不可逆）。
type AdminPlayerBriefItem struct {
	PlayerID          int64   `json:"player_id"`
	PlayerNo          string  `json:"player_no"`
	Name              string  `json:"name"`
	IDType            string  `json:"id_type"`
	IDCardMasked      *string `json:"id_card_masked"`
	BirthDate         *string `json:"birth_date"`
	Gender            *string `json:"gender"`
	Institution       string  `json:"institution"`
	MergedInto        *int64  `json:"merged_into"`
	RegistrationCount int64   `json:"registration_count"`
	ResultCount       int64   `json:"result_count"`
}

// AdminPlayerListResponse 是后台选手档案列表响应（统一分页结构）。
type AdminPlayerListResponse struct {
	List     []AdminPlayerBriefItem `json:"list"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// AdminPlayerRegistration 是选手详情中的历史报名记录（API 文档 13.2）。
type AdminPlayerRegistration struct {
	OrderNo   string `json:"order_no"`
	EventName string `json:"event_name"`
	Level     string `json:"level"`
	DanceType string `json:"dance_type"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// AdminPlayerResultSummary 是选手详情中的成绩记录（API 文档 13.2）。
type AdminPlayerResultSummary struct {
	Award     string `json:"award"`
	EventName string `json:"event_name"`
	Points    int64  `json:"points"`
	CreatedAt string `json:"created_at"`
}

// AdminPlayerPointsSummary 是选手详情中的积分汇总（按赛季聚合，取最近赛季）。
type AdminPlayerPointsSummary struct {
	SeasonName  string `json:"season_name"`
	TotalPoints int64  `json:"total_points"`
	EventCount  int64  `json:"event_count"`
}

// AdminPlayerDuplicate 是选手详情中的重复档案提示（同证件号哈希）。
type AdminPlayerDuplicate struct {
	PlayerID  int64  `json:"player_id"`
	PlayerNo  string `json:"player_no"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// AdminPlayerDetail 是选手档案详情（API 文档 13.2）。
type AdminPlayerDetail struct {
	AdminPlayerBriefItem
	Registrations  []AdminPlayerRegistration  `json:"registrations"`
	ResultsSummary []AdminPlayerResultSummary `json:"results_summary"`
	PointsSummary  AdminPlayerPointsSummary   `json:"points_summary"`
	Duplicates     []AdminPlayerDuplicate     `json:"duplicates"`
}

// ContentEntry 是内容配置读写响应（API 文档 13.4）。
// Value 为内容 JSON 对象（文案、图片 URL、上下线时间）。
type ContentEntry struct {
	Key       string         `json:"key"`
	Value     map[string]any `json:"value"`
	UpdatedAt string         `json:"updated_at"`
}
