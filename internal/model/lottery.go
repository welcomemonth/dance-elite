package model

// 抽签 H5 的数据文件结构与接口 DTO。
//
// 抽签数据与报名库完全解耦：每个赛事一个 JSON 文件，文件名即访问 token。
// 数据由 cmd/lottery-import 从报名 Excel 生成，服务端只读写该文件。

// LotteryGenre 是抽签页面下拉框中的一个舞种。
type LotteryGenre struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// LotteryGroup 是一个「组别 × 舞种」下的字母组（A/B/C…）及其人数。
type LotteryGroup struct {
	Genre    string `json:"genre"`
	AgeGroup string `json:"age_group"`
	Letter   string `json:"letter"`
	Size     int    `json:"size"`
}

// LotterySlot 是抽签结果中的一个槽位：组字母 + 组内序号。
type LotterySlot struct {
	Letter string `json:"letter"`
	No     int    `json:"no"`
}

// LotteryPlayer 是参与抽签的一名选手；Slot 为 nil 表示尚未抽签。
type LotteryPlayer struct {
	Name        string       `json:"name"`
	IDCard      string       `json:"id_card"`
	Genre       string       `json:"genre"`
	AgeGroup    string       `json:"age_group"`
	Institution string       `json:"institution,omitempty"`
	Program     string       `json:"program,omitempty"`
	Slot        *LotterySlot `json:"slot"`
	DrawnAt     string       `json:"drawn_at,omitempty"`
}

// LotteryData 是一个赛事的完整抽签数据，整体读写同一个 JSON 文件。
type LotteryData struct {
	Version   int             `json:"version"`
	EventName string          `json:"event_name"`
	Genres    []LotteryGenre  `json:"genres"`
	Groups    []LotteryGroup  `json:"groups"`
	Players   []LotteryPlayer `json:"players"`
}

// LotteryDrawRequest 是 POST /h5/lottery/{token}/draw 的请求体。
type LotteryDrawRequest struct {
	Name   string `json:"name" binding:"required"`
	IDCard string `json:"id_card" binding:"required"`
	Genre  string `json:"genre" binding:"required"`
}

// LotteryQueryRequest 是 POST /h5/lottery/{token}/query 的请求体。
type LotteryQueryRequest struct {
	IDCard string `json:"id_card" binding:"required"`
	Genre  string `json:"genre" binding:"required"`
}

// LotteryMeta 是 GET /h5/lottery/{token} 的响应：页面初始化所需的赛事信息。
type LotteryMeta struct {
	EventName string         `json:"event_name"`
	Genres    []LotteryGenre `json:"genres"`
}

// LotteryResult 是抽签/查询接口的统一响应。
type LotteryResult struct {
	Name        string       `json:"name"`
	Genre       string       `json:"genre"`
	GenreLabel  string       `json:"genre_label"`
	AgeGroup    string       `json:"age_group"`
	Program     string       `json:"program,omitempty"`
	Institution string       `json:"institution,omitempty"`
	Slot        *LotterySlot `json:"slot"`
	DrawnAt     string       `json:"drawn_at,omitempty"`
	// JustDrawn 表示本次请求实际完成了抽签（false 为查询或重复点击）。
	JustDrawn bool `json:"just_drawn"`
}
