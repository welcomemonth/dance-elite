import { API_BASE_URL, DEBUG, REQUEST_TIMEOUT } from "../config/env";
import { ERROR_CODE, type ApiEnvelope } from "../types/api";
import { ApiError } from "./api-error";
import { unwrapEnvelope } from "./api-response";
import { readToken } from "./session";

export type HttpMethod = "GET" | "POST" | "PUT" | "DELETE";

export type QueryValue = string | number | boolean | null | undefined;
export type QueryParams = Record<string, QueryValue>;

export type RequestData = string | WechatMiniprogram.IAnyObject | ArrayBuffer;

export interface RequestConfig {
  method?: HttpMethod;
  query?: QueryParams;
  data?: RequestData;
  /** 是否携带 Authorization，默认 true。 */
  auth?: boolean;
  header?: Record<string, string>;
  timeout?: number;
}

type UnauthorizedHandler = () => void | Promise<void>;

let unauthorizedHandler: UnauthorizedHandler | null = null;

/**
 * 注册 40101 的统一处理（例如清理登录态并重新静默登录）。
 * 返回 Promise 时，请求层会在其完成后自动重试一次原请求。
 */
export function setUnauthorizedHandler(handler: UnauthorizedHandler | null): void {
  unauthorizedHandler = handler;
}

/** 供 upload 等其他请求通道在检测到 40101 时复用统一处理。 */
export function notifyUnauthorized(): void {
  void Promise.resolve(unauthorizedHandler?.()).catch(() => undefined);
}

function buildUrl(path: string, query?: QueryParams): string {
  const url = `${API_BASE_URL}${path}`;
  if (!query) {
    return url;
  }
  const search = Object.entries(query)
    .filter(([, value]) => value !== undefined && value !== null && value !== "")
    .map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`)
    .join("&");
  return search ? `${url}?${search}` : url;
}

/**
 * 统一请求封装：
 * - 自动拼接 API_BASE_URL、token；
 * - 解包 `{ code, message, data }`，只把 data 返回给调用方；
 * - 统一抛 ApiError，40101 触发全局 unauthorized 处理。
 */
function send<T>(path: string, config: RequestConfig = {}): Promise<T> {
  const { method = "GET", query, data, auth = true, header = {}, timeout = REQUEST_TIMEOUT } = config;
  const token = readToken();
  const url = buildUrl(path, query);

  if (DEBUG) {
    console.log(`[request] → ${method} ${url}`, data ?? "");
  }

  return new Promise<T>((resolve, reject) => {
    wx.request<ApiEnvelope<T>>({
      url,
      method,
      data,
      timeout,
      header: {
        "Content-Type": "application/json",
        ...(auth && token ? { Authorization: `Bearer ${token}` } : {}),
        ...header,
      },
      success(response) {
        if (DEBUG) {
          console.log(
            `[request] ← ${method} ${url} (HTTP ${response.statusCode})`,
            response.data,
          );
        }
        try {
          resolve(unwrapEnvelope<T>(response.data, response.statusCode));
        } catch (error) {
          reject(error);
        }
      },
      fail(error) {
        if (DEBUG) {
          console.error(`[request] ✕ ${method} ${url} 未收到响应`, error);
        }
        reject(new ApiError(-1, error.errMsg || "网络异常，请稍后重试"));
      },
    });
  });
}

/**
 * 统一请求入口：40101 时先走统一的重新登录处理，成功后自动重试一次，
 * 避免 token 过期后用户必须手动进入「我的」页才恢复登录。
 */
export function request<T>(path: string, config: RequestConfig = {}): Promise<T> {
  return send<T>(path, config).catch((error: unknown) => {
    if (
      error instanceof ApiError &&
      error.code === ERROR_CODE.UNAUTHORIZED &&
      config.auth !== false &&
      unauthorizedHandler
    ) {
      return Promise.resolve(unauthorizedHandler()).then(() => send<T>(path, config));
    }
    throw error;
  });
}

export const http = {
  get<T>(path: string, query?: QueryParams, config: RequestConfig = {}): Promise<T> {
    return request<T>(path, { ...config, method: "GET", query });
  },
  post<T>(path: string, data?: RequestData, config: RequestConfig = {}): Promise<T> {
    return request<T>(path, { ...config, method: "POST", data });
  },
  put<T>(path: string, data?: RequestData, config: RequestConfig = {}): Promise<T> {
    return request<T>(path, { ...config, method: "PUT", data });
  },
  delete<T>(path: string, data?: RequestData, config: RequestConfig = {}): Promise<T> {
    return request<T>(path, { ...config, method: "DELETE", data });
  },
};
