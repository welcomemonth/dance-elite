import type { Session } from "../types/models";

const SESSION_KEY = "dance_elite_session";

/** 读取本地登录态；数据损坏或缺失时返回 null。 */
export function readSession(): Session | null {
  const raw: unknown = wx.getStorageSync(SESSION_KEY);
  if (!raw || typeof raw !== "object") {
    return null;
  }
  const session = raw as Partial<Session>;
  if (typeof session.token !== "string" || !session.token || !session.user) {
    return null;
  }
  return { token: session.token, user: session.user };
}

export function readToken(): string {
  return readSession()?.token ?? "";
}

export function writeSession(session: Session): void {
  wx.setStorageSync(SESSION_KEY, session);
}

export function removeSession(): void {
  wx.removeStorageSync(SESSION_KEY);
}
