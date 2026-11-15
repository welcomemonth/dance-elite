import { API_BASE_URL, REQUEST_TIMEOUT } from "../config/env";
import { ERROR_CODE } from "../types/api";
import { ApiError } from "./api-error";
import { unwrapEnvelope } from "./api-response";
import { notifyUnauthorized } from "./request";
import { readToken } from "./session";

export interface UploadConfig {
  /** 接口路径，例如 /api/v1/registrations/5001/avatar */
  path: string;
  /** wx.chooseMedia / chooseMessageFile 返回的本地临时路径 */
  filePath: string;
  /** multipart 字段名，默认 file */
  name?: string;
  /** 附加表单字段（值会被微信转成字符串） */
  formData?: Record<string, string | number | boolean>;
  /** 是否携带 Authorization，默认 true */
  auth?: boolean;
  header?: Record<string, string>;
  timeout?: number;
}

/**
 * 文件上传：与 utils/request 一致地拼接 baseURL、携带 token、解包统一响应、抛 ApiError。
 *
 * 示例：
 *   const [filePath] = await chooseImages({ count: 1 });
 *   await uploadFile<{ url: string }>({ path: "/api/v1/registrations/5001/id-card", filePath });
 */
export function uploadFile<T>(config: UploadConfig): Promise<T> {
  const token = readToken();

  return new Promise<T>((resolve, reject) => {
    wx.uploadFile({
      url: `${API_BASE_URL}${config.path}`,
      filePath: config.filePath,
      name: config.name ?? "file",
      formData: config.formData,
      timeout: config.timeout ?? REQUEST_TIMEOUT,
      header: {
        ...(config.auth !== false && token ? { Authorization: `Bearer ${token}` } : {}),
        ...config.header,
      },
      success(response) {
        let payload: unknown = response.data;
        if (typeof response.data === "string") {
          try {
            payload = JSON.parse(response.data);
          } catch {
            reject(new ApiError(response.statusCode, `上传失败（HTTP ${response.statusCode}）`, response.statusCode));
            return;
          }
        }

        try {
          resolve(unwrapEnvelope<T>(payload, response.statusCode));
        } catch (error) {
          if (error instanceof ApiError && error.code === ERROR_CODE.UNAUTHORIZED) {
            notifyUnauthorized();
          }
          reject(error);
        }
      },
      fail(error) {
        reject(new ApiError(-1, error.errMsg || "上传失败，请稍后重试"));
      },
    });
  });
}
