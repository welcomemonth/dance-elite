package model

// 抽签域模型与接口 DTO。
//
// 抽签数据存 SQLite（lottery_sessions / lottery_groups / lottery_players），
// 与报名库同一套数据层；一次赛事一个抽签 session，token 即免登录链接凭据。
// 证件号不存明文：id_card_hash 用于比对，id_card_enc 为 AES-GCM 密文
// （仅 Excel 导入来源有，导出时解密）；从赛事报名生成时只有哈希。

// 抽签创建来源。
const (
	LotterySourceExcel = "excel" // 上传报名 Excel 解析生成
	LotterySourceEvent = "event" // 从已截止赛事的有效报名生成
)

// LotteryGenre 是抽签页展示的舞种。
type LotteryGenre struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// LotteryGroup 是一个「组别 × 舞种」下的字母组（A/B/C…）及其人数。
type LotteryGroup struct {
	ID        int64  `json:"-"`
	SessionID int64  `json:"-"`
	Genre     string `json:"genre"`
	AgeGroup  string `json:"age_group"`
	Letter    string `json:"letter"`
	Size      int    `json:"size"`
	// Drawn 是已分配人数，仅在后台详情中统计。
	Drawn int `json:"drawn,omitempty"`
}

// LotterySlot 是抽签结果中的一个槽位：组字母 + 组内序号。
type LotterySlot struct {
	Letter string `json:"letter"`
	No     int    `json:"no"`
}

// LotterySMSState 记录一名选手的提醒短信与结果短信发送情况。
type LotterySMSState struct {
	RemindSentAt   *string `json:"-"`
	RemindAttempts int     `json:"-"`
	ResultSentAt   *string `json:"-"`
	ResultAttempts int     `json:"-"`
}

// LotterySession 是一次赛事抽签（lottery_sessions）。
type LotterySession struct {
	ID                  int64
	EventID             *int64
	EventName           string
	Token               string
	Source              string
	StartAt             *string
	EndAt               *string
	RemindBeforeMinutes int
	SMSRemindTemplate   *string
	SMSResultTemplate   *string
	CreatedBy           *int64
	CreatedAt           string
	UpdatedAt           string
	// PlayerCount / DrawnCount 是列表/详情用的聚合值。
	PlayerCount int
	DrawnCount  int
}

// LotteryPlayer 是一名选手在某舞种下的抽签记录（lottery_players）。
type LotteryPlayer struct {
	ID             int64
	SessionID      int64
	RegistrationID *int64
	Name           string
	IDCardHash     string
	IDCardMasked   *string
	IDCardEnc      *string
	Genre          string
	AgeGroup       string
	Institution    *string
	Program        *string
	Phone          *string
	GroupLetter    *string
	SlotNo         *int
	DrawnAt        *string
	AutoDrawn      bool
	// DrawnOpenID / DrawnUnionID 是手动抽签时的公众号身份（网页授权获取）；
	// 系统自动分配为 nil。
	DrawnOpenID  *string
	DrawnUnionID *string
	SMS          LotterySMSState
	CreatedAt    string
	UpdatedAt    string
}

// Slot 返回已分配槽位；尚未抽签返回 nil。
func (p *LotteryPlayer) Slot() *LotterySlot {
	if p.SlotNo == nil || p.GroupLetter == nil {
		return nil
	}
	return &LotterySlot{Letter: *p.GroupLetter, No: *p.SlotNo}
}

// LotteryLookupRequest 是 lookup / query 的请求体。
// Lookup 时可带姓名做二次核对；Query 只按证件号返回全部记录。
type LotteryLookupRequest struct {
	Name   string `json:"name"`
	IDCard string `json:"id_card" binding:"required"`
}

// LotteryDrawRequest 是 POST /h5/lottery/{token}/draw 的请求体。
// Genre 为空时：只有一条待抽记录直接抽；同一选手有多条待抽记录则返回
// choose_genre，由前端弹窗让家长选择舞种。
type LotteryDrawRequest struct {
	Name   string `json:"name" binding:"required"`
	IDCard string `json:"id_card" binding:"required"`
	Genre  string `json:"genre"`
}

// 抽签页面状态，由服务端按服务端时钟计算后下发。
const (
	LotteryStatusNotStarted = "not_started"
	LotteryStatusOngoing    = "ongoing"
	LotteryStatusEnded      = "ended"
)

// 抽签接口的业务结果状态。
const (
	// LotteryOutcomeDrawn 表示本次完成了抽签（含截止后自动分配）。
	LotteryOutcomeDrawn = "drawn"
	// LotteryOutcomeAlreadyDrawn 表示所查询记录均已抽签，未做任何修改。
	LotteryOutcomeAlreadyDrawn = "already_drawn"
	// LotteryOutcomeChooseGenre 表示同一姓名+证件号存在多个舞种待抽，需前端选择。
	LotteryOutcomeChooseGenre = "choose_genre"
)

// LotteryMeta 是 GET /h5/lottery/{token} 的响应：页面初始化所需的赛事信息。
type LotteryMeta struct {
	EventName  string         `json:"event_name"`
	Genres     []LotteryGenre `json:"genres"`
	StartAt    string         `json:"start_at,omitempty"`
	EndAt      string         `json:"end_at,omitempty"`
	ServerTime string         `json:"server_time"`
	Status     string         `json:"status"`
	// Authorized 表示当前浏览器已通过公众号网页授权（或服务端处于
	// 本地调试的 open 模式）。前端在 false 时跳转授权，不渲染表单。
	Authorized bool `json:"authorized"`
}

// LotteryEntry 是一名选手在某舞种下的抽签记录（一名选手可报多个舞种）。
type LotteryEntry struct {
	Name        string       `json:"name"`
	Genre       string       `json:"genre"`
	GenreLabel  string       `json:"genre_label"`
	AgeGroup    string       `json:"age_group"`
	Program     string       `json:"program,omitempty"`
	Institution string       `json:"institution,omitempty"`
	Slot        *LotterySlot `json:"slot"`
	DrawnAt     string       `json:"drawn_at,omitempty"`
	AutoDrawn   bool         `json:"auto_drawn"`
	// Drawn 等价于 Slot != nil，便于前端直接判断。
	Drawn bool `json:"drawn"`
}

// LotteryLookupResult 是 lookup / query 的响应：按证件号命中的全部记录。
type LotteryLookupResult struct {
	Entries []LotteryEntry `json:"entries"`
}

// LotteryDrawResult 是抽签接口的响应。
type LotteryDrawResult struct {
	Status  string         `json:"status"`
	Entries []LotteryEntry `json:"entries"`
	Choices []LotteryGenre `json:"choices,omitempty"`
}

// AdminLotterySession 是后台抽签列表/详情中的一条赛事抽签。
type AdminLotterySession struct {
	ID                  int64  `json:"id"`
	Token               string `json:"token"`
	EventID             *int64 `json:"event_id"`
	EventName           string `json:"event_name"`
	Source              string `json:"source"`
	StartAt             string `json:"start_at,omitempty"`
	EndAt               string `json:"end_at,omitempty"`
	RemindBeforeMinutes int    `json:"remind_before_minutes"`
	SMSRemindTemplate   string `json:"sms_remind_template,omitempty"`
	SMSResultTemplate   string `json:"sms_result_template,omitempty"`
	PlayerCount         int    `json:"player_count"`
	DrawnCount          int    `json:"drawn_count"`
	PendingCount        int    `json:"pending_count"`
	CreatedAt           string `json:"created_at,omitempty"`
	UpdatedAt           string `json:"updated_at,omitempty"`
	// Link 是拼接 public_base_url 后的免登录页面地址；未配置时返回相对路径。
	Link string `json:"link"`
}

// AdminLotteryListResponse 是 GET /api/v1/admin/lottery/sessions 的响应。
type AdminLotteryListResponse struct {
	List []AdminLotterySession `json:"list"`
}

// AdminLotteryGroupStat 是后台详情中的分组进度。
type AdminLotteryGroupStat struct {
	Genre      string `json:"genre"`
	GenreLabel string `json:"genre_label"`
	AgeGroup   string `json:"age_group"`
	Letter     string `json:"letter"`
	Size       int    `json:"size"`
	Drawn      int    `json:"drawn"`
}

// AdminLotteryDetail 是后台单场抽签详情。
type AdminLotteryDetail struct {
	AdminLotterySession
	Groups []AdminLotteryGroupStat `json:"groups"`
	// Draws 是已抽签选手的明细（含抽签微信身份与匹配到的小程序账号）。
	Draws []AdminLotteryDraw `json:"draws"`
}

// AdminLotteryDraw 是后台详情中的一条抽签记录。
type AdminLotteryDraw struct {
	PlayerName string       `json:"player_name"`
	Genre      string       `json:"genre"`
	GenreLabel string       `json:"genre_label"`
	AgeGroup   string       `json:"age_group"`
	Slot       *LotterySlot `json:"slot"`
	DrawnAt    string       `json:"drawn_at,omitempty"`
	AutoDrawn  bool         `json:"auto_drawn"`
	// OpenID / UnionID 是手动抽签时的公众号身份；系统自动分配为空。
	OpenID  string `json:"openid,omitempty"`
	UnionID string `json:"unionid,omitempty"`
	// SameWechatCount 是同一抽签微信在本场抽签中的操作人数（按选手去重，含本人），
	// 用于发现一个微信替多人抽签的情况；自动分配为 0。
	SameWechatCount int `json:"same_wechat_count,omitempty"`
	// IDCardHash 只用于统计同微信人数，不对外输出。
	IDCardHash string `json:"-"`
	// User 是按 unionid 匹配到的小程序账号；未匹配为 nil。
	User *AdminLotteryDrawUser `json:"user,omitempty"`
}

// AdminLotteryDrawUser 是抽签微信匹配到的小程序账号（手机号已掩码）。
type AdminLotteryDrawUser struct {
	ID       int64  `json:"id"`
	Nickname string `json:"nickname,omitempty"`
	Phone    string `json:"phone,omitempty"`
}

// AdminLotteryCreateResult 是导入后返回的结果摘要。
type AdminLotteryCreateResult struct {
	AdminLotterySession
	Warnings []string `json:"warnings,omitempty"`
}

// AdminLotterySettingsRequest 是修改抽签设置（时间窗口/短信模板）的请求体。
type AdminLotterySettingsRequest struct {
	EventID             *int64 `json:"event_id"`
	StartAt             string `json:"start_at"`
	EndAt               string `json:"end_at"`
	RemindBeforeMinutes int    `json:"remind_before_minutes"`
	SMSRemindTemplate   string `json:"sms_remind_template"`
	SMSResultTemplate   string `json:"sms_result_template"`
}
