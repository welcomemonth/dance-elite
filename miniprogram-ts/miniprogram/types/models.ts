/** 小程序端用户信息（`POST /api/v1/auth/wx-login` 返回的 user）。 */
export interface User {
  id: number;
  nickname: string;
  avatar_url: string | null;
  /** 绑定手机号，未绑定为 null（后端脱敏返回）。 */
  phone: string | null;
  /** 默认选手编号，无则为 null。 */
  player_no: string | null;
}

export interface LoginResult {
  token: string;
  expires_in: number;
  user: User;
  is_new: boolean;
}

export interface Session {
  token: string;
  user: User;
}

/** 小程序端展示状态（后端按时间线推导，API 文档 2.1）。 */
export type EventStatus = "upcoming" | "registering" | "reg_closed" | "finished";

/** 赛事级别枚举（events.level）。 */
export type EventLevel = "grade_a" | "super" | "final";

/** 年龄组别枚举（event_groups.level）。 */
export type GroupLevel = "U11" | "U13" | "U15";

/** 舞种枚举。 */
export type DanceType = "classical" | "folk";

/** 赛事列表中的组别报名热度（API 文档 2.1）。 */
export interface GroupSummary {
  level: GroupLevel;
  dance_type: DanceType;
  registered: number;
  quota: number;
}

/** 赛事列表项（GET /api/v1/events，API 文档 2.1）。 */
export interface EventItem {
  id: number;
  name: string;
  level: EventLevel;
  city: string | null;
  venue: string | null;
  event_date: string | null;
  cover_url: string | null;
  /** 单位：分 */
  fee: number;
  status: EventStatus;
  reg_end_at: string | null;
  group_summary: GroupSummary[];
  registered_by_me: boolean;
}

/** 赛事详情时间线（API 文档 2.2）。 */
export interface EventTimeline {
  reg_start_at: string | null;
  reg_end_at: string | null;
  lottery_at: string | null;
  material_check_at: string | null;
  start_at: string | null;
}

/** 赛事详情中的报名单元（赛事 × 组别 × 舞种）。 */
export interface EventGroup {
  group_id: number;
  level: GroupLevel;
  dance_type: DanceType;
  birth_year_from: number;
  birth_year_to: number;
  quota: number;
  registered: number;
  remaining: number;
  enabled: boolean;
}

/** 赛事详情（GET /api/v1/events/{id}，API 文档 2.2）。 */
export interface EventDetail {
  id: number;
  name: string;
  level: EventLevel;
  city: string | null;
  venue: string | null;
  event_date: string | null;
  cover_url: string | null;
  /** 单位：分 */
  fee: number;
  status: EventStatus;
  timeline: EventTimeline;
  groups: EventGroup[];
  refund_policy: string | null;
  work_duration_limit: string | null;
  process_desc: string | null;
  honor_desc: string | null;
}

/** 提交报名请求体（POST /api/v1/events/{event_id}/registrations）。 */
export interface SubmitRegistrationPayload {
  player_name: string;
  id_card_no: string;
  institution?: string;
  dance_type: DanceType;
  idempotency_key: string;
}

/** 提交报名响应（API 文档 3.1）。 */
export interface SubmitRegistrationResult {
  order_no: string;
  order_id: number;
  registration_id: number;
  /** 单位：分 */
  amount: number;
  level: GroupLevel;
  dance_type: DanceType;
  material_deadline: string | null;
}

/** 微信支付参数（微信协议字段为 camelCase，文档级例外）。 */
export interface WxPayParams {
  timeStamp: string;
  nonceStr: string;
  package: string;
  signType: string;
  paySign: string;
}

/** 支付预下单响应（API 文档 3.1b）。 */
export interface PrepayResult {
  order_no: string;
  pay_params: WxPayParams;
}

/** 订单状态查询响应（API 文档 3.1c）。 */
export interface OrderStatusResult {
  order_id: number;
  order_no: string;
  status: "pending_payment" | "paid" | "canceled" | "refunding" | "refunded";
  /** 单位：分 */
  amount: number;
  paid_at: string | null;
  registration_id: number;
}

/** 取消订单响应（API 文档 3.5）。 */
export interface CancelOrderResult {
  order_id: number;
  order_no: string;
  status: "canceled";
  cancel_reason: "user_cancel" | "pay_failed" | "system";
  canceled_at: string;
}
