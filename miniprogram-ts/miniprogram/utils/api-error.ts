/** 后端业务错误（HTTP 状态码非 2xx 或统一响应 code !== 0）。 */
export class ApiError extends Error {
  /** 统一响应里的业务 code；网络失败时为 -1。 */
  readonly code: number;
  /** HTTP 状态码；网络失败时为 0。 */
  readonly status: number;

  constructor(code: number, message: string, status = 0) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}
