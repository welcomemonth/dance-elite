import type { BindPhoneResult, LoginResult } from "../types/models";
import { DEBUG } from "../config/env";
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
        if (DEBUG) {
          console.log("[wx.login] 成功", result);
        }
        if (result.code) {
          resolve(result.code);
          return;
        }
        reject(new Error("微信登录失败：未获取到 code"));
      },
      fail(error) {
        // 微信登录在本地就失败了，请求不会发出；打印完整错误对象便于定位。
        console.error("[wx.login] 失败", error);
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

/**
 * 绑定/换绑手机号：`getPhoneNumber` 返回的 code 交给后端换取手机号。
 * 对应接口：PUT /api/v1/auth/phone（API 文档 1.2）。
 */
export function bindPhone(phoneCode: string): Promise<BindPhoneResult> {
  return http.put<BindPhoneResult>("/api/v1/auth/phone", { phone_code: phoneCode });
}
