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

/** 解析 JWT payload（不验签，仅用于本地判断过期时间）。 */
function readJwtPayload(token: string): { exp?: number } | null {
  const parts = token.split(".");
  if (parts.length !== 3) {
    return null;
  }
  try {
    const base64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = base64 + "=".repeat((4 - (base64.length % 4)) % 4);
    const bytes = new Uint8Array(wx.base64ToArrayBuffer(padded));
    let text = "";
    for (let i = 0; i < bytes.length; i += 1) {
      text += String.fromCharCode(bytes[i]);
    }
    return JSON.parse(text) as { exp?: number };
  } catch {
    return null;
  }
}

/**
 * token 是否已过期（默认提前 60 秒视为过期，避免边界请求失败）。
 * 无法解析 exp 时按已过期处理，强制重新登录。
 */
export function isTokenExpired(token: string, skewSeconds = 60): boolean {
  const payload = readJwtPayload(token);
  if (!payload || typeof payload.exp !== "number") {
    return true;
  }
  return payload.exp * 1000 <= Date.now() + skewSeconds * 1000;
}

/** 读取可用 token；本地已过期时返回空串，避免带着废 token 请求。 */
export function readToken(): string {
  const token = readSession()?.token ?? "";
  if (!token || isTokenExpired(token)) {
    return "";
  }
  return token;
}

export function writeSession(session: Session): void {
  wx.setStorageSync(SESSION_KEY, session);
}

export function removeSession(): void {
  wx.removeStorageSync(SESSION_KEY);
}
