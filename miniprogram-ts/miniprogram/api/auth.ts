import type { LoginResult } from "../types/models";
import { http } from "../utils/request";

interface WxLoginPayload {
  code: string;
  nickname?: string;
  avatar_url?: string;
}

function getWxLoginCode(): Promise<string> {
  return new Promise((resolve, reject) => {
    wx.login({
      success(result) {
        if (result.code) {
          resolve(result.code);
          return;
        }
        reject(new Error("微信登录失败：未获取到 code"));
      },
      fail(error) {
        reject(new Error(`微信登录失败：${error.errMsg}`));
      },
    });
  });
}

/**
 * 微信静默登录：`wx.login` 取 code，后端按 openid 创建/匹配账号并签发 JWT。
 * 对应接口：POST /api/v1/auth/wx-login
 */
export function loginWithWechat(payload: Omit<WxLoginPayload, "code"> = {}): Promise<LoginResult> {
  return getWxLoginCode().then((code) =>
    http.post<LoginResult>("/api/v1/auth/wx-login", { ...payload, code }, { auth: false }),
  );
}
