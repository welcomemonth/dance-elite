package model

// AdminScoreEntry 是成绩录入页的单条成绩（已发布）。
type AdminScoreEntry struct {
	EntryID         int64   `json:"entry_id"`
	PlayerID        int64   `json:"player_id"`
	PlayerNo        string  `json:"player_no"`
	PlayerName      string  `json:"player_name"`
	IDCardMasked    *string `json:"id_card_masked"`
	Award           *string `json:"award"`
	AdvancedToFinal bool    `json:"advanced_to_final"`
	Points          int     `json:"points"`
	Remark          *string `json:"remark"`
	Published       bool    `json:"published"`
}

// ScorePreviewInputRow 是解析预览/发布时提交的一行（在线录入或前端解析 Excel 后）。
type ScorePreviewInputRow struct {
	RowNo           int    `json:"row_no"`
	PlayerName      string `json:"player_name" binding:"required,max=30"`
	IDCardNo        string `json:"id_card_no"`
	Award           string `json:"award"`
	AdvancedToFinal bool   `json:"advanced_to_final"`
	Remark          string `json:"remark" binding:"max=200"`
}

// ScorePreviewRequest 是 POST .../scores/preview 的请求体。
type ScorePreviewRequest struct {
	Rows []ScorePreviewInputRow `json:"rows" binding:"required,min=1,dive"`
}

// ScorePreviewRowResult 是预览结果单行。
type ScorePreviewRowResult struct {
	RowNo           int     `json:"row_no"`
	PlayerName      string  `json:"player_name"`
	IDCardMasked    *string `json:"id_card_masked"`
	Award           *string `json:"award"`
	AdvancedToFinal bool    `json:"advanced_to_final"`
	// match_status：matched（命中已有档案）/ created（导入时新建）/ new（待新建，仅预览有错误行时出现）/
	// unmatched（无法匹配或建档）。
	MatchStatus string  `json:"match_status"`
	PlayerID    *int64  `json:"player_id"`
	Remark      *string `json:"remark"`
	Error       *string `json:"error,omitempty"`
}

// ScorePreviewResponse 是预览结果。
type ScorePreviewResponse struct {
	TotalRows   int                     `json:"total_rows"`
	Matched     int                     `json:"matched"`
	Created     int                     `json:"created"`
	Errors      int                     `json:"errors"`
	Unmatched   int                     `json:"unmatched"` // 兼容字段：等于 errors
	ImportBatch string                  `json:"import_batch,omitempty"`
	Rows        []ScorePreviewRowResult `json:"rows"`
}

// PublishScoresRequest 是确认发布成绩的请求体（rows 用预览结果）。
type PublishScoresRequest struct {
	Rows []ScorePreviewRowResult `json:"rows" binding:"required,min=1,dive"`
}

// PublishScoresResponse 是发布结果。
type PublishScoresResponse struct {
	Published int `json:"published"`
}

// CorrectScoreRequest 是成绩纠错请求体（修改奖项/晋级/备注）。
type CorrectScoreRequest struct {
	Award           string `json:"award"`
	AdvancedToFinal *bool  `json:"advanced_to_final"`
	Remark          string `json:"remark" binding:"max=200"`
}

// AdminLeaderboardRow 是榜单管理页的一行。
type AdminLeaderboardRow struct {
	Rank                int     `json:"rank"`
	PlayerID            int64   `json:"player_id"`
	PlayerNo            string  `json:"player_no"`
	PlayerName          string  `json:"player_name"`
	DisplayName         *string `json:"display_name"`
	Institution         string  `json:"institution"`
	TotalPoints         int     `json:"total_points"`
	EventCount          int     `json:"event_count"`
	Hidden              bool    `json:"hidden"`
	DirectQualification bool    `json:"direct_qualification"`
}

// AdminLeaderboard 是单个榜单。
type AdminLeaderboard struct {
	BoardID     int64                 `json:"board_id"`
	Level       string                `json:"level"`
	DanceType   string                `json:"dance_type"`
	Rows        []AdminLeaderboardRow `json:"rows"`
	PublishedAt *string               `json:"published_at"`
}

// AdminLeaderboardListResponse 是一个赛季的 6 个榜单。
type AdminLeaderboardListResponse struct {
	List []AdminLeaderboard `json:"list"`
}

// UpdateLeaderboardRowRequest 是设置展示名/隐藏的请求体。
type UpdateLeaderboardRowRequest struct {
	DisplayName *string `json:"display_name"`
	Hidden      *bool   `json:"hidden"`
}

// AdminQualificationCard 是直通卡（含自动与手动）。
type AdminQualificationCard struct {
	CardID     int64   `json:"card_id"`
	PlayerID   int64   `json:"player_id"`
	PlayerNo   string  `json:"player_no"`
	PlayerName string  `json:"player_name"`
	Board      string  `json:"board"`
	Source     string  `json:"source"` // auto / manual
	Reason     *string `json:"reason"`
	GrantedAt  string  `json:"granted_at"`
}

// GrantQualificationRequest 是手动增补直通卡。
type GrantQualificationRequest struct {
	BoardID  int64  `json:"board_id" binding:"required"`
	PlayerID int64  `json:"player_id" binding:"required"`
	Reason   string `json:"reason" binding:"required,max=200"`
}

// AdminFinalsRosterRow 是总决赛资格名单行。
type AdminFinalsRosterRow struct {
	PlayerID    int64  `json:"player_id"`
	PlayerNo    string `json:"player_no"`
	PlayerName  string `json:"player_name"`
	Institution string `json:"institution"`
	Board       string `json:"board"`
	Source      string `json:"source"` // direct_card / points_top20 / manual
	TotalPoints int    `json:"total_points"`
	Rank        int    `json:"rank"`
	Manual      bool   `json:"manual"`
}

// AdjustFinalsRosterRequest 是手动增补/移除总决赛资格。
type AdjustFinalsRosterRequest struct {
	BoardID  int64  `json:"board_id" binding:"required"`
	PlayerID int64  `json:"player_id" binding:"required"`
	Action   string `json:"action" binding:"required"` // add / remove
	Reason   string `json:"reason" binding:"required,max=200"`
}

// AdminInstitutionRankRow 是后台机构年度榜行。
type AdminInstitutionRankRow struct {
	Rank        int     `json:"rank"`
	Institution string  `json:"institution"`
	TotalPoints int     `json:"total_points"`
	PlayerCount int     `json:"player_count"`
	Honor       *string `json:"honor"` // annual_elite / annual_top10
	PrizeAmount int     `json:"prize_amount"`
}

// AdminPrizeStatementItem 是奖金明细单行。
type AdminPrizeStatementItem struct {
	ItemID           int64   `json:"item_id"`
	EventID          *int64  `json:"event_id"`
	EventName        string  `json:"event_name"`
	PlayerID         *int64  `json:"player_id"`
	PlayerName       string  `json:"player_name"`
	Institution      *string `json:"institution"`
	Award            string  `json:"award"`
	Amount           int     `json:"amount"`
	AdjustedAmount   int     `json:"adjusted_amount"`
	AdjustReason     *string `json:"adjust_reason"`
	Paid             bool    `json:"paid"`
	PaidAt           *string `json:"paid_at"`
	Payee            *string `json:"payee"`
	Remark           *string `json:"remark"`
	ExcludeFromPrize bool    `json:"exclude_from_prize"`
}

// AdjustPrizeRequest 是奖金金额调整。
type AdjustPrizeRequest struct {
	Amount int    `json:"amount" binding:"required"`
	Reason string `json:"reason" binding:"required,max=200"`
}

// PayPrizeRequest 是登记发放。
type PayPrizeRequest struct {
	Payee string `json:"payee" binding:"required,max=50"`
	Note  string `json:"note" binding:"max=200"`
}
