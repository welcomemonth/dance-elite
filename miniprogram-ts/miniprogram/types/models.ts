/** 小程序端用户信息（`POST /api/v1/auth/wx-login` 返回的 user）。 */
export interface User {
  id: number;
  nickname: string;
  avatar_url: string | null;
  /** 绑定手机号，未绑定为 null（后端脱敏返回）。 */
  phone: string | null;
  /** 默认选手编号，无则为 null。 */
  player_no: string | null;
}

export interface LoginResult {
  token: string;
  expires_in: number;
  user: User;
  is_new: boolean;
}

export interface Session {
  token: string;
  user: User;
}
