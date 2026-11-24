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

/** 绑定手机号响应（API 文档 1.2）。 */
export interface BindPhoneResult {
  phone: string;
}

/** 「我的报名」展示状态（API 文档 3.3/3.4，由订单状态映射）。 */
export type RegistrationStatus = "paid" | "refunding" | "refunded" | "withdrawn";

/** 资料完成度（服务端实时计算，missing 为空即资料已齐）。 */
export interface MaterialStatus {
  total: number;
  completed: number;
  missing: string[];
}

/** 「我的报名」列表项（GET /api/v1/registrations，API 文档 3.3）。 */
export interface RegistrationListItem {
  registration_id: number;
  event_id: number;
  event_name: string;
  level: EventLevel;
  event_date: string | null;
  level_group: GroupLevel;
  dance_type: DanceType;
  player_name: string;
  player_no: string;
  order_no: string;
  /** 单位：分 */
  amount: number;
  status: RegistrationStatus;
  material_status: MaterialStatus;
  material_deadline: string | null;
}

/** 报名详情中的赛事摘要（API 文档 3.4）。 */
export interface RegistrationEventSummary {
  id: number;
  name: string;
  level: EventLevel;
  event_date: string | null;
}

/** 报名详情中的退款信息（API 文档 3.4）。 */
export interface RegistrationRefund {
  ticket_no: string;
  status: "processing" | "refunded";
  reason: string;
}

/** 报名资料快照（API 文档 3.4，未传字段为 null）。 */
export interface Materials {
  photo_url: string | null;
  audio_url: string | null;
  audio_filename: string | null;
  program_name: string | null;
  teacher_name: string | null;
  contact_phone: string | null;
  express_address: string | null;
  work_intro: string | null;
  special_req: string | null;
  updated_at: string | null;
}

/** 报名详情（GET /api/v1/registrations/{id}，API 文档 3.4）。 */
export interface RegistrationDetail {
  status: RegistrationStatus;
  status_desc: string;
  material_readonly: boolean;
  material_status: MaterialStatus;
  event: RegistrationEventSummary;
  level_group: GroupLevel;
  dance_type: DanceType;
  player_name: string;
  player_no: string;
  institution: string | null;
  order_no: string;
  /** 单位：分 */
  amount: number;
  paid_at: string | null;
  refund: RegistrationRefund | null;
  materials: Materials;
}

/** 文本资料更新请求体（PUT /api/v1/registrations/{id}/materials，API 文档 4.1）。 */
export interface UpdateMaterialsPayload {
  program_name?: string;
  teacher_name?: string;
  contact_phone?: string;
  express_address?: string;
  work_intro?: string;
  special_req?: string;
}

/** 资料更新响应（API 文档 4.1）。 */
export interface UpdateMaterialsResult {
  material_status: MaterialStatus;
  materials: Materials;
}

/** 候补展示状态（API 文档 5.2；removed 为后台移出后保留的留档记录）。 */
export type WaitlistStatus = "waiting" | "notified" | "converted" | "given_up" | "removed";

/** 「我的候补」列表项（GET /api/v1/waitlists，API 文档 5.2）。 */
export interface WaitlistItem {
  waitlist_id: number;
  event_id: number;
  event_name: string;
  level: GroupLevel;
  dance_type: DanceType;
  position_no: number;
  status: WaitlistStatus;
  created_at: string;
}

/** 提交候补请求体（POST /api/v1/events/{event_id}/waitlists，API 文档 5.1）。 */
export interface SubmitWaitlistPayload {
  group_id: number;
  name: string;
  id_card_no?: string;
  institution?: string;
  phone: string;
}

/** 提交候补响应（API 文档 5.1）。 */
export interface SubmitWaitlistResult {
  waitlist_id: number;
  position_no: number;
  status: "waiting";
}

/** 榜单选项（年龄段/舞种）。 */
export interface RankingOption {
  value: string;
  label: string;
}

/** 榜单首页配置（API 文档 6.1）。 */
export interface RankingConfig {
  season: string;
  title: string;
  subtitle: string;
  levels: RankingOption[];
  dance_styles: RankingOption[];
  advance_quota: number;
  updated_at: string | null;
}

/** 直通榜条目（API 文档 6.2）。 */
export interface RankingAwardItem {
  event_name: string;
  award: string;
  prize_amount: number;
}

export interface RankingDirectEntry {
  id: number;
  name: string;
  avatar_url: string;
  awards: RankingAwardItem[];
}

/** 积分榜条目（API 文档 6.2）。 */
export interface RankingEntry {
  id: number;
  rank: number;
  prev_rank: number;
  rank_change: number;
  name: string;
  avatar_url: string;
  total_points: number;
  is_direct: boolean;
}

export interface RankingBoard {
  board_id: number;
  updated_at: string | null;
  direct: RankingDirectEntry[];
  ranking: RankingEntry[];
}

/** 机构榜条目（API 文档 6.3）。 */
export interface InstitutionRankEntry {
  rank: number;
  name: string;
  total_points: number;
  player_count: number;
  honor: string | null;
}

export interface InstitutionRanking {
  updated_at: string | null;
  institutions: InstitutionRankEntry[];
}

/** 选手积分详情（API 文档 6.4）。 */
export interface DancerEventItem {
  event_name: string;
  event_date: string | null;
  level: string;
  award: string;
  points: number;
}

export interface DancerDetail {
  id: number;
  name: string;
  avatar_url: string;
  board: { level: string; dance_type: string };
  rank: number;
  total_points: number;
  counted_events: DancerEventItem[];
  history: DancerEventItem[];
  honors: string[];
  qualification: { type: string; source: string } | null;
}

/** 公开内容配置 key（后台内容管理 / API 文档 13.4）。 */
export type ContentKey =
  | "points_rules_desc"
  | "event_faq"
  | "official_account"
  | "announcement";

/** 公开内容值：文案 + 可选图片与上下线时间（公告）。 */
export interface ContentValue {
  text: string;
  image_url?: string;
  online_at?: string;
  offline_at?: string;
}

export interface ContentEntry {
  key: ContentKey;
  value: ContentValue;
  updated_at: string;
}
