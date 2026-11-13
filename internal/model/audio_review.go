package model

// AudioReviewTokenResponse 是生成/重新生成音频审核链接的响应（API 文档 10.6）。
type AudioReviewTokenResponse struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// AudioReviewEntry 是音频审核 H5 列表中的一条报名（API 文档 7.1）。
// 仅含姓名/节目名/音频三类信息，不含任何联系方式。
type AudioReviewEntry struct {
	PlayerName  string `json:"player_name"`
	ProgramName string `json:"program_name"`
	AudioURL    string `json:"audio_url"`
	UpdatedAt   string `json:"updated_at"`
}

// AudioReviewData 是音频审核 H5 数据响应（API 文档 7.1）。
type AudioReviewData struct {
	EventName string             `json:"event_name"`
	Entries   []AudioReviewEntry `json:"entries"`
}
