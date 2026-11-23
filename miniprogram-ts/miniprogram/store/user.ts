import { makeAutoObservable } from "mobx";

import { bindPhone as bindPhoneApi, loginWithWechat } from "../api/auth";
import type { User } from "../types/models";
import { isTokenExpired, readSession, removeSession, writeSession } from "../utils/session";

const initialSession = readSession();
// 本地缓存 token 已过期时直接丢弃，确保启动后会重新静默登录。
const initialValid = initialSession && !isTokenExpired(initialSession.token) ? initialSession : null;
if (initialSession && !initialValid) {
  removeSession();
}

// 防止并发登录（启动静默登录 + 页面手动登录同时触发）。
let loginTask: Promise<User> | null = null;

class UserStore {
  token = initialValid?.token ?? "";
  profile: User | null = initialValid?.user ?? null;
  loggingIn = false;

  constructor() {
    makeAutoObservable(this, {}, { autoBind: true });
  }

  get isLoggedIn(): boolean {
    return this.token.length > 0;
  }

  get displayName(): string {
    return this.profile?.nickname?.trim() || "菁英舞者";
  }

  setSession(token: string, user: User): void {
    this.token = token;
    this.profile = user;
    writeSession({ token, user });
  }

  clearSession(): void {
    this.token = "";
    this.profile = null;
    removeSession();
  }

  /** 登录（已有进行中的登录请求时复用同一个 Promise）。 */
  login(): Promise<User> {
    if (loginTask) {
      return loginTask;
    }

    this.loggingIn = true;
    loginTask = loginWithWechat()
      .then((result) => {
        console.log("[userStore] 登录成功", result);
        this.setSession(result.token, result.user);
        return result.user;
      })
      .catch((error: unknown) => {
        console.error("[userStore] 登录失败", error);
        throw error;
      })
      .finally(() => {
        loginTask = null;
        this.loggingIn = false;
      });
    return loginTask;
  }

  /**
   * 已登录且 token 未过期则直接返回用户信息；token 缺失/过期时先清理
   * 本地会话再发起静默登录。页面在调需要鉴权的接口前都应 await 本方法。
   */
  ensureLogin(): Promise<User> {
    if (this.token && this.profile && !isTokenExpired(this.token)) {
      return Promise.resolve(this.profile);
    }
    if (this.token) {
      this.clearSession();
    }
    return this.login();
  }

  /** 绑定/换绑手机号，成功后同步本地用户信息（后端返回脱敏号码）。 */
  async bindPhone(phoneCode: string): Promise<string> {
    const result = await bindPhoneApi(phoneCode);
    if (this.profile) {
      this.profile = { ...this.profile, phone: result.phone };
      writeSession({ token: this.token, user: this.profile });
    }
    return result.phone;
  }
}

export const userStore = new UserStore();
