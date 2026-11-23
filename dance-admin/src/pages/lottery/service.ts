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

/** 抽签页面状态：未开始 / 进行中 / 已截止。 */
export type LotteryStatus = 'not_started' | 'ongoing' | 'ended';

/** 抽签接口业务结果。 */
export type LotteryOutcome = 'drawn' | 'already_drawn' | 'choose_genre';

export interface LotteryMeta {
  /** 赛事名称（页面标题） */
  event_name: string;
  /** 赛事包含的舞种（仅展示用，不再要求手动选择） */
  genres: LotteryGenre[];
  /** 抽签开始时间（ISO 8601）；缺省表示不限制 */
  start_at?: string;
  /** 抽签截止时间（ISO 8601）；缺省表示不限制 */
  end_at?: string;
  /** 服务端当前时间（ISO 8601），用于校准倒计时 */
  server_time: string;
  /** 服务端判定的页面状态 */
  status: LotteryStatus;
}

/** 一名选手在某舞种下的抽签记录（同一人可报多个舞种）。 */
export interface LotteryEntry {
  name: string;
  genre: string;
  genre_label: string;
  age_group: string;
  program?: string;
  institution?: string;
  /** 抽签结果；null 表示尚未抽签 */
  slot: LotterySlot | null;
  /** 抽签时间（ISO 8601） */
  drawn_at?: string;
  /** 槽位是否由系统在截止后自动分配 */
  auto_drawn: boolean;
  drawn: boolean;
}

export interface LotteryLookupResult {
  entries: LotteryEntry[];
}

export interface LotteryDrawResult {
  status: LotteryOutcome;
  entries: LotteryEntry[];
  /** status=choose_genre 时可选舞种 */
  choices?: LotteryGenre[];
}

export interface LotteryDrawPayload {
  name: string;
  id_card: string;
  /** 留空由服务端判断：唯一舞种直接抽，多舞种返回 choose_genre */
  genre?: string;
}

export interface LotteryQueryPayload {
  id_card: string;
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

/** POST /h5/lottery/{token}/lookup：按证件号（可选姓名）查全部记录。 */
export async function lookupLottery(
  token: string,
  payload: LotteryQueryPayload & { name?: string },
): Promise<LotteryLookupResult> {
  return postJSON<LotteryLookupResult>(
    `/h5/lottery/${encodeURIComponent(token)}/lookup`,
    payload,
  );
}

/** POST /h5/lottery/{token}/draw：抽签（已抽签返回原结果）。 */
export async function drawLottery(
  token: string,
  payload: LotteryDrawPayload,
): Promise<LotteryDrawResult> {
  return postJSON<LotteryDrawResult>(
    `/h5/lottery/${encodeURIComponent(token)}/draw`,
    payload,
  );
}

/** POST /h5/lottery/{token}/query：只凭证件号查询全部已抽签信息。 */
export async function queryLottery(
  token: string,
  payload: LotteryQueryPayload,
): Promise<LotteryLookupResult> {
  return postJSON<LotteryLookupResult>(
    `/h5/lottery/${encodeURIComponent(token)}/query`,
    payload,
  );
}
