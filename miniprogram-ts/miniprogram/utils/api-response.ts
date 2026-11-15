import { ERROR_CODE, type ApiEnvelope } from "../types/api";
import { ApiError } from "./api-error";

/**
 * 解包统一响应 `{ code, message, data }`：
 * 成功返回 data，失败抛 ApiError（非 JSON / 网关错误页按 HTTP 状态码处理）。
 */
export function unwrapEnvelope<T>(payload: unknown, status: number): T {
  const body = payload as Partial<ApiEnvelope<T>> | null | undefined;
  if (body && body.code === ERROR_CODE.SUCCESS) {
    return body.data as T;
  }

  const code = typeof body?.code === "number" ? body.code : status;
  const message = body?.message || `请求失败（HTTP ${status}）`;
  throw new ApiError(code, message, status);
}
