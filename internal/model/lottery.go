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
	Name        string `json:"name"`
	IDCard      string `json:"id_card"`
	Genre       string `json:"genre"`
	AgeGroup    string `json:"age_group"`
	Institution string `json:"institution,omitempty"`
	Program     string `json:"program,omitempty"`
	// Phone 是短信提醒用的手机号（明文）；为空表示不发短信。
	Phone   string       `json:"phone,omitempty"`
	Slot    *LotterySlot `json:"slot"`
	DrawnAt string       `json:"drawn_at,omitempty"`
	// AutoDrawn 表示该槽位由系统在截止后自动分配（非本人操作）。
	AutoDrawn bool `json:"auto_drawn,omitempty"`
	// SMS 记录该选手的短信发送状态；nil 表示从未尝试发送。
	SMS *LotterySMSState `json:"sms,omitempty"`
}

// LotterySMSState 记录一名选手的提醒短信与结果短信发送情况。
//
// 两种短信各自独立记录：SentAt 非空表示已成功发送；Attempts 达到上限后
// 不再重试，避免模板错误时无休止地调用短信接口。
type LotterySMSState struct {
	RemindSentAt   string `json:"remind_sent_at,omitempty"`
	RemindAttempts int    `json:"remind_attempts,omitempty"`
	ResultSentAt   string `json:"result_sent_at,omitempty"`
	ResultAttempts int    `json:"result_attempts,omitempty"`
}

// LotteryData 是一个赛事的完整抽签数据，整体读写同一个 JSON 文件。
//
// StartAt / EndAt 为空表示该赛事未配置抽签时间窗口（沿用旧行为：随时可抽、
// 无自动分配、无短信提醒）。时间格式为全库统一的 ISO 8601 本地时间。
type LotteryData struct {
	Version   int    `json:"version"`
	EventName string `json:"event_name"`
	// StartAt 是抽签开始时间；到点前页面只展示开始时间，抽签接口拒绝请求。
	StartAt string `json:"start_at,omitempty"`
	// EndAt 是抽签截止时间；到点后剩余未抽选手由系统自动分配。
	EndAt string `json:"end_at,omitempty"`
	// RemindBeforeMinutes 是截止前多少分钟发送提醒短信；0 表示默认 120 分钟。
	RemindBeforeMinutes int `json:"remind_before_minutes,omitempty"`
	// SMSRemindTemplate / SMSResultTemplate 是阿里云短信模板 code。
	// 模板正文由本赛事专用（抽签链接写死在正文里），留空则不发短信。
	SMSRemindTemplate string          `json:"sms_remind_template,omitempty"`
	SMSResultTemplate string          `json:"sms_result_template,omitempty"`
	Genres            []LotteryGenre  `json:"genres"`
	Groups            []LotteryGroup  `json:"groups"`
	Players           []LotteryPlayer `json:"players"`
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

// 抽签页面状态，由服务端按服务端时钟计算后下发。
const (
	LotteryStatusNotStarted = "not_started"
	LotteryStatusOngoing    = "ongoing"
	LotteryStatusEnded      = "ended"
)

// LotteryMeta 是 GET /h5/lottery/{token} 的响应：页面初始化所需的赛事信息。
//
// ServerTime 用于前端校准倒计时（家长手机时钟可能不准，不能只信客户端时间）。
type LotteryMeta struct {
	EventName  string         `json:"event_name"`
	Genres     []LotteryGenre `json:"genres"`
	StartAt    string         `json:"start_at,omitempty"`
	EndAt      string         `json:"end_at,omitempty"`
	ServerTime string         `json:"server_time"`
	Status     string         `json:"status"`
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
	// AutoDrawn 表示槽位由系统在截止后自动分配。
	AutoDrawn bool `json:"auto_drawn"`
}
