import { makeAutoObservable } from "mobx";

import { loginWithWechat } from "../api/auth";
import type { User } from "../types/models";
import { readSession, removeSession, writeSession } from "../utils/session";

const initialSession = readSession();

// 防止并发登录（启动静默登录 + 页面手动登录同时触发）。
let loginTask: Promise<User> | null = null;

class UserStore {
  token = initialSession?.token ?? "";
  profile: User | null = initialSession?.user ?? null;
  loggingIn = false;

  constructor() {
    makeAutoObservable(this, {}, { autoBind: true });
  }

  get isLoggedIn(): boolean {
    return this.token.length > 0;
  }

  get displayName(): string {
    return this.profile?.nickname || "未登录";
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

  /** 已登录则直接返回用户信息，否则发起静默登录。 */
  ensureLogin(): Promise<User> {
    if (this.profile) {
      return Promise.resolve(this.profile);
    }
    return this.login();
  }
}

export const userStore = new UserStore();
