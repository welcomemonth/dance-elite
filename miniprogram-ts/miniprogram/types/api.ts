/** 后端统一响应包装：`{"code": 0, "message": "success", "data": {}}` */
export interface ApiEnvelope<T> {
  code: number;
  message: string;
  data: T;
}

/** 统一分页结构。 */
export interface PagedList<T> {
  list: T[];
  total: number;
  page: number;
  page_size: number;
}

export const ERROR_CODE = {
  SUCCESS: 0,
  INVALID_PARAM: 40001,
  UNAUTHORIZED: 40101,
  FORBIDDEN: 40301,
  NOT_FOUND: 40401,
  CONFLICT: 40901,
  RATE_LIMITED: 42901,
  SERVER_ERROR: 50001,
  // 报名域
  ID_CARD_INVALID: 41001,
  AGE_NOT_MATCH: 41002,
  DUPLICATE_GROUP: 41003,
  DUPLICATE_DANCE: 41004,
  QUOTA_FULL: 41005,
  EVENT_NOT_REGISTERING: 41006,
  GROUP_CLOSED: 41007,
  MATERIAL_DEADLINE_PASSED: 41008,
  REGISTRATION_NOT_EDITABLE: 41009,
  WAITLIST_DUPLICATE: 41010,
  WAITLIST_NOT_CANCELABLE: 41011,
  EVENT_STATUS_NOT_ALLOWED: 41012,
  STATUS_NOT_ALLOWED: 41013,
  REFUND_API_FAILED: 41014,
  REFUND_CONDITION_NOT_MET: 41015,
} as const;

export type ErrorCode = (typeof ERROR_CODE)[keyof typeof ERROR_CODE];
