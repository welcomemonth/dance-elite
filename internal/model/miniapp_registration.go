package model

// 小程序端「我的报名」展示状态（API 文档 3.3/3.4）。
// 由订单状态映射：paid / refunding / refunded；待支付与已取消订单不进入小程序页面。
const (
	MiniappRegistrationStatusPaid      = "paid"      // 已支付
	MiniappRegistrationStatusRefunding = "refunding" // 退款处理中
	MiniappRegistrationStatusRefunded  = "refunded"  // 已退款
)

// MiniappRegistrationItem 是「我的报名」列表项（API 文档 3.3）。
type MiniappRegistrationItem struct {
	RegistrationID   int64          `json:"registration_id"`
	EventID          int64          `json:"event_id"`
	EventName        string         `json:"event_name"`
	Level            string         `json:"level"`
	EventDate        *string        `json:"event_date"`
	LevelGroup       string         `json:"level_group"`
	DanceType        string         `json:"dance_type"`
	PlayerName       string         `json:"player_name"`
	PlayerNo         string         `json:"player_no"`
	OrderNo          string         `json:"order_no"`
	Amount           int64          `json:"amount"`
	Status           string         `json:"status"`
	MaterialStatus   MaterialStatus `json:"material_status"`
	MaterialDeadline *string        `json:"material_deadline"`
}

// MiniappRegistrationListResponse 是「我的报名」分页响应（API 文档 3.3）。
type MiniappRegistrationListResponse struct {
	List     []MiniappRegistrationItem `json:"list"`
	Total    int64                     `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
}

// MiniappRegistrationEvent 是报名详情中的赛事摘要（API 文档 3.4）。
type MiniappRegistrationEvent struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Level     string  `json:"level"`
	EventDate *string `json:"event_date"`
}

// MiniappRegistrationRefund 是报名详情中的退款信息（API 文档 3.4）。
// status 仅向小程序暴露 processing（处理中）/ refunded（已退款）两个展示态。
type MiniappRegistrationRefund struct {
	TicketNo string `json:"ticket_no"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
}

// MiniappRegistrationDetail 是「报名详情」响应（API 文档 3.4）。
type MiniappRegistrationDetail struct {
	Status           string                     `json:"status"`
	StatusDesc       string                     `json:"status_desc"`
	MaterialReadonly bool                       `json:"material_readonly"`
	MaterialStatus   MaterialStatus             `json:"material_status"`
	Event            MiniappRegistrationEvent   `json:"event"`
	LevelGroup       string                     `json:"level_group"`
	DanceType        string                     `json:"dance_type"`
	PlayerName       string                     `json:"player_name"`
	PlayerNo         string                     `json:"player_no"`
	Institution      *string                    `json:"institution"`
	OrderNo          string                     `json:"order_no"`
	Amount           int64                      `json:"amount"`
	PaidAt           *string                    `json:"paid_at"`
	Refund           *MiniappRegistrationRefund `json:"refund"`
	Materials        MaterialsSnapshot          `json:"materials"`
}
