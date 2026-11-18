/**
 * 抽签 H5 数据接口（免登录，URL token 鉴权）
 * 对应 docs/优舞盟报名功能/API接口设计.md 第 7.2 节
 */

export interface LotteryGenre {
  /** 舞种枚举值（classical / folk） */
  key: string;
  /** 舞种展示名 */
  label: string;
}

export interface LotterySlot {
  /** 组字母 A/B/C… */
  letter: string;
  /** 组内序号，从 1 开始 */
  no: number;
}

export interface LotteryMeta {
  /** 赛事名称（页面标题） */
  event_name: string;
  /** 可选舞种 */
  genres: LotteryGenre[];
}

export interface LotteryResult {
  /** 选手姓名 */
  name: string;
  /** 舞种枚举值 */
  genre: string;
  /** 舞种展示名 */
  genre_label: string;
  /** 年龄组（U11 / U13 / U15） */
  age_group: string;
  /** 剧目名称 */
  program?: string;
  /** 选送机构 */
  institution?: string;
  /** 抽签结果；null 表示尚未抽签 */
  slot: LotterySlot | null;
  /** 抽签时间（ISO 8601） */
  drawn_at?: string;
  /** 本次请求是否实际完成了抽签 */
  just_drawn: boolean;
}

export interface LotteryDrawPayload {
  name: string;
  id_card: string;
  genre: string;
}

export interface LotteryQueryPayload {
  id_card: string;
  genre: string;
}

interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

/** 读取统一响应包装；业务错误直接抛出服务端文案。 */
async function readData<T>(res: Response): Promise<T> {
  const body = (await res.json().catch(() => null)) as ApiResponse<T> | null;
  if (!body) {
    throw new Error(`请求失败（HTTP ${res.status}），请稍后重试`);
  }
  if (body.code !== 0) {
    throw new Error(body.message || `请求失败（code ${body.code}）`);
  }
  return body.data;
}

async function postJSON<T>(path: string, payload: unknown): Promise<T> {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return readData<T>(res);
}

/** GET /h5/lottery/{token}：页面初始化信息。 */
export async function fetchLotteryMeta(token: string): Promise<LotteryMeta> {
  const res = await fetch(`/h5/lottery/${encodeURIComponent(token)}`);
  return readData<LotteryMeta>(res);
}

/** POST /h5/lottery/{token}/draw：抽签（已抽过返回原结果）。 */
export async function drawLottery(
  token: string,
  payload: LotteryDrawPayload,
): Promise<LotteryResult> {
  return postJSON<LotteryResult>(
    `/h5/lottery/${encodeURIComponent(token)}/draw`,
    payload,
  );
}

/** POST /h5/lottery/{token}/query：按证件号 + 舞种查询。 */
export async function queryLottery(
  token: string,
  payload: LotteryQueryPayload,
): Promise<LotteryResult> {
  return postJSON<LotteryResult>(
    `/h5/lottery/${encodeURIComponent(token)}/query`,
    payload,
  );
}
