package model

// AdminRefundTicketListQuery 是退款工单列表筛选（API 文档 11.4.2）。
// CreatedBy > 0 时仅返回该管理员登记的工单（cs 角色限制为自己登记的）。
type AdminRefundTicketListQuery struct {
	Status      string
	EventID     int64
	CreatedFrom string
	CreatedTo   string
	Keyword     string // 工单号 / 订单号 / 选手姓名
	CreatedBy   int64
	Page        int
	PageSize    int
}

// AdminRefundTicketItem 是退款工单列表单条记录（API 文档 11.4.2）。
type AdminRefundTicketItem struct {
	TicketID    int64   `json:"ticket_id"`
	TicketNo    string  `json:"ticket_no"`
	OrderNo     string  `json:"order_no"`
	EventID     int64   `json:"event_id"`
	EventName   string  `json:"event_name"`
	PlayerName  string  `json:"player_name"`
	Reason      string  `json:"reason"`
	Applicant   string  `json:"applicant"`
	Status      string  `json:"status"`
	HandlerName *string `json:"handler_name"`
	CreatedAt   string  `json:"created_at"`
	HandledAt   *string `json:"handled_at"`
}

// AdminRefundTicketListResponse 是退款工单列表响应（统一分页结构）。
type AdminRefundTicketListResponse struct {
	List     []AdminRefundTicketItem `json:"list"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

// AdminRefundOrderBrief 是退款工单详情中的订单摘要（API 文档 11.4.2b）。
type AdminRefundOrderBrief struct {
	OrderNo       string  `json:"order_no"`
	TransactionID *string `json:"transaction_id"`
	EventID       int64   `json:"event_id"`
	EventName     string  `json:"event_name"`
	Level         string  `json:"level"`
	DanceType     string  `json:"dance_type"`
	PlayerName    string  `json:"player_name"`
	IDCardMasked  *string `json:"id_card_masked"` // 系统仅存哈希，物理上无明文，恒 null
	Amount        int64   `json:"amount"`
	Status        string  `json:"status"`
	Source        string  `json:"source"`
	PaidAt        *string `json:"paid_at"`
	CreatedAt     string  `json:"created_at"`
}

// RefundHistoryEntry 是退款工单详情中的处理留痕（API 文档 11.4.2b）。
type RefundHistoryEntry struct {
	Action    string `json:"action"`
	Operator  string `json:"operator"`
	CreatedAt string `json:"created_at"`
	Detail    string `json:"detail"` // 操作详情 JSON 的紧凑文本
}

// AdminRefundTicketDetail 是退款工单详情（API 文档 11.4.2b）。
type AdminRefundTicketDetail struct {
	AdminRefundTicketItem
	Content       string                `json:"content"`
	Order         AdminRefundOrderBrief `json:"order"`
	PolicyStage   string                `json:"policy_stage"` // lottery_before / lottery_after
	SuggestAmount int64                 `json:"suggest_amount"`
	PaidAmount    int64                 `json:"paid_amount"`
	History       []RefundHistoryEntry  `json:"history"`
}
