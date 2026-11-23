package model

// AdminWithdrawRegistrationRequest 是后台人工退赛的请求体（设计文档 3.1）。
// 退赛 ≠ 退款：仅把报名置为退赛并释放名额，订单与资金状态不变。
type AdminWithdrawRegistrationRequest struct {
	Reason string `json:"reason" binding:"omitempty,max=200"`
}

// AdminWithdrawRegistrationResponse 是人工退赛的响应。
type AdminWithdrawRegistrationResponse struct {
	RegistrationID int64  `json:"registration_id"`
	Status         string `json:"status"`
	Released       bool   `json:"released"`
}
