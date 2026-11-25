/**
 * 赛事抽签 mock：后台列表/导入/设置/导出 + 免登录抽签 H5。
 *
 * 真实后端的数据来自报名 Excel；mock 只保留一个演示赛事，便于本地调 UI。
 * 演示 token：LotteryDemo1（赛事 1，含「张三」古典舞/民族民间舞两条记录）。
 */
import type { Request, Response } from 'express';
import { fail, ok, randomToken, toInt } from './_utils';

interface MockSlot {
  letter: string;
  no: number;
}

interface MockLotteryPlayer {
  name: string;
  id_card: string;
  genre: 'classical' | 'folk';
  age_group: 'U11' | 'U13' | 'U15';
  slot: MockSlot | null;
  drawn_at?: string;
  auto_drawn?: boolean;
  drawn_openid?: string;
  drawn_unionid?: string;
}

interface MockLotterySession {
  id: number;
  token: string;
  source: 'excel' | 'event';
  event_id: number | null;
  event_name: string;
  start_at?: string;
  end_at?: string;
  remind_before_minutes: number;
  sms_remind_template?: string;
  sms_result_template?: string;
  created_at?: string;
  updated_at?: string;
  players: MockLotteryPlayer[];
}

const sessions: MockLotterySession[] = [
  {
    id: 1,
    token: 'LotteryDemo1',
    source: 'excel',
    event_id: 1,
    event_name: '2026 优舞盟上海甲级赛',
    start_at: '2026-11-01T10:00:00+08:00',
    end_at: '2026-12-31T22:00:00+08:00',
    remind_before_minutes: 120,
    created_at: '2026-10-20T09:00:00+08:00',
    updated_at: '2026-10-20T09:00:00+08:00',
    players: [
      { name: '张三', id_card: '310101201501011234', genre: 'classical', age_group: 'U11', slot: null },
      { name: '张三', id_card: '310101201501011234', genre: 'folk', age_group: 'U11', slot: null },
      { name: '李四', id_card: '310101201501015678', genre: 'classical', age_group: 'U13', slot: null },
    ],
  },
];

const canWrite = (req: Request) => !String(req.headers.authorization ?? '').includes('-cs-');

const groupLabels: Record<string, string> = { classical: '古典舞', folk: '民族民间舞' };

function findSession(token: string): MockLotterySession | undefined {
  return sessions.find((item) => item.token === token);
}

function drawnCount(session: MockLotterySession): number {
  return session.players.filter((player) => player.slot).length;
}

function summarize(session: MockLotterySession) {
  const drawn = drawnCount(session);
  return {
    id: session.id,
    token: session.token,
    event_id: session.event_id,
    event_name: session.event_name,
    source: session.source,
    start_at: session.start_at,
    end_at: session.end_at,
    remind_before_minutes: session.remind_before_minutes,
    sms_remind_template: session.sms_remind_template,
    sms_result_template: session.sms_result_template,
    player_count: session.players.length,
    drawn_count: drawn,
    pending_count: session.players.length - drawn,
    created_at: session.created_at,
    updated_at: session.updated_at,
    link: `/lottery/${session.token}`,
  };
}

function drawsOf(session: MockLotterySession) {
  const wechatCounts = new Map<string, number>();
  for (const player of session.players) {
    if (player.slot && player.drawn_openid) {
      wechatCounts.set(
        player.drawn_openid,
        (wechatCounts.get(player.drawn_openid) ?? 0) + 1,
      );
    }
  }
  return session.players
    .filter((player) => player.slot)
    .map((player) => ({
      player_name: player.name,
      genre: player.genre,
      genre_label: groupLabels[player.genre] ?? player.genre,
      age_group: player.age_group,
      slot: player.slot,
      drawn_at: player.drawn_at,
      auto_drawn: Boolean(player.auto_drawn),
      openid: player.drawn_openid,
      unionid: player.drawn_unionid,
      same_wechat_count: player.drawn_openid
        ? (wechatCounts.get(player.drawn_openid) ?? 1)
        : 0,
      user: player.drawn_unionid
        ? { id: 1, nickname: 'mock 家长', phone: '138****8000' }
        : undefined,
    }));
}

function groupStats(session: MockLotterySession) {
  const map = new Map<string, { genre: string; age_group: string; letter: string; size: number; drawn: number }>();
  for (const player of session.players) {
    const letter = player.slot?.letter ?? 'A';
    const key = `${player.genre}-${player.age_group}-${letter}`;
    const item = map.get(key) ?? {
      genre: player.genre,
      age_group: player.age_group,
      letter,
      size: 0,
      drawn: 0,
    };
    item.size += 1;
    if (player.slot) item.drawn += 1;
    map.set(key, item);
  }
  return [...map.values()].map((item) => ({
    ...item,
    genre_label: groupLabels[item.genre] ?? item.genre,
  }));
}

function entriesOf(session: MockLotterySession, idCard: string, name?: string) {
  return session.players
    .filter((player) => player.id_card === idCard && (!name || player.name === name))
    .map((player) => ({
      name: player.name,
      genre: player.genre,
      genre_label: groupLabels[player.genre],
      age_group: player.age_group,
      slot: player.slot,
      drawn_at: player.drawn_at,
      auto_drawn: Boolean(player.auto_drawn),
      drawn: Boolean(player.slot),
    }));
}

function nextSlot(session: MockLotterySession, player: MockLotteryPlayer): MockSlot {
  const used = new Set(
    session.players
      .filter((item) => item.slot && item.genre === player.genre && item.age_group === player.age_group)
      .map((item) => item.slot?.no),
  );
  let no = 1;
  while (used.has(no)) no += 1;
  return { letter: 'A', no };
}

const notFound = '未查询到报名信息：请确认身份证号与报名时填写的一致（18 位，末位 X 用大写）。若确认已报名，请把「姓名 + 身份证号」发给赛事客服核对。';

export default {
  'GET /api/v1/admin/lottery/sessions': (_req: Request, res: Response) =>
    ok(res, { list: sessions.map(summarize) }),

  'GET /api/v1/admin/lottery/sessions/:token': (req: Request, res: Response) => {
    const session = findSession(String(req.params.token));
    if (!session) return fail(res, 40401, '抽签不存在或链接无效');
    return ok(res, { ...summarize(session), groups: groupStats(session), draws: drawsOf(session) });
  },

  'POST /api/v1/admin/lottery/sessions': (req: Request, res: Response) => {
    if (!canWrite(req)) return fail(res, 40301, '无权限');
    const eventId = toInt(req.body?.event_id ?? 1) || 1;
    const existing = sessions.find((item) => item.event_id === eventId);
    if (existing) {
      const token = existing.token;
      existing.updated_at = new Date().toISOString();
      return ok(res, { ...summarize(existing), warnings: [`mock：覆盖演示赛事，token=${token}`] });
    }
    const token = randomToken();
    const session: MockLotterySession = {
      id: Math.max(...sessions.map((item) => item.id)) + 1,
      token,
      source: 'excel',
      event_id: eventId,
      event_name: `mock 赛事 ${eventId} 抽签`,
      start_at: req.body?.start_at,
      end_at: req.body?.end_at,
      remind_before_minutes: toInt(req.body?.remind_before_minutes ?? 120) || 120,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      players: [
        { name: '张三', id_card: '310101201501011234', genre: 'classical', age_group: 'U11', slot: null },
        { name: '李四', id_card: '310101201501015678', genre: 'classical', age_group: 'U11', slot: null },
      ],
    };
    sessions.unshift(session);
    return ok(res, { ...summarize(session), warnings: ['mock 数据：未读取上传的 Excel'] });
  },

  'PUT /api/v1/admin/lottery/sessions/:token': (req: Request, res: Response) => {
    if (!canWrite(req)) return fail(res, 40301, '无权限');
    const session = findSession(String(req.params.token));
    if (!session) return fail(res, 40401, '抽签不存在或链接无效');
    const body = req.body ?? {};
    session.start_at = body.start_at || undefined;
    session.end_at = body.end_at || undefined;
    session.remind_before_minutes = toInt(body.remind_before_minutes ?? 120) || 120;
    session.sms_remind_template = body.sms_remind_template || undefined;
    session.sms_result_template = body.sms_result_template || undefined;
    session.updated_at = new Date().toISOString();
    return ok(res, summarize(session));
  },

  'GET /api/v1/admin/lottery/sessions/:token/export': (_req: Request, res: Response) =>
    fail(res, 40001, 'mock 环境不生成 Excel，请连接真实后端导出', 50),

  'GET /h5/lottery/:token': (req: Request, res: Response) => {
    const session = findSession(String(req.params.token));
    if (!session) return fail(res, 40401, '抽签链接无效或已失效');
    const now = Date.now();
    const start = session.start_at ? Date.parse(session.start_at) : 0;
    const end = session.end_at ? Date.parse(session.end_at) : 0;
    const status = start && now < start ? 'not_started' : end && now >= end ? 'ended' : 'ongoing';
    return ok(res, {
      event_name: session.event_name,
      genres: [
        { key: 'classical', label: '古典舞' },
        { key: 'folk', label: '民族民间舞' },
      ],
      start_at: session.start_at,
      end_at: session.end_at,
      server_time: new Date().toISOString(),
      status,
      authorized: true,
    });
  },

  'POST /h5/lottery/:token/lookup': (req: Request, res: Response) => {
    const session = findSession(String(req.params.token));
    if (!session) return fail(res, 40401, '抽签链接无效或已失效');
    const entries = entriesOf(session, String(req.body?.id_card ?? ''), req.body?.name);
    if (entries.length === 0) return fail(res, 40401, notFound);
    return ok(res, { entries });
  },

  'POST /h5/lottery/:token/query': (req: Request, res: Response) => {
    const session = findSession(String(req.params.token));
    if (!session) return fail(res, 40401, '抽签链接无效或已失效');
    const entries = entriesOf(session, String(req.body?.id_card ?? ''));
    if (entries.length === 0) return fail(res, 40401, notFound);
    return ok(res, { entries });
  },

  'POST /h5/lottery/:token/draw': (req: Request, res: Response) => {
    const session = findSession(String(req.params.token));
    if (!session) return fail(res, 40401, '抽签链接无效或已失效');
    const name = String(req.body?.name ?? '');
    const idCard = String(req.body?.id_card ?? '');
    const genre = String(req.body?.genre ?? '');
    const matched = session.players.filter((player) => player.id_card === idCard && player.name === name);
    if (matched.length === 0) return fail(res, 40401, '信息不匹配：请核对姓名和身份证号是否与报名时一致（证件号末位 X 用大写）');

    if (genre) {
      const target = matched.find((player) => player.genre === genre);
      if (!target) return fail(res, 40401, '未找到该舞种的报名记录，请核对姓名、证件号和舞种');
      if (target.slot) {
        return ok(res, { status: 'already_drawn', entries: entriesOf(session, idCard, name) });
      }
      target.slot = nextSlot(session, target);
      target.drawn_at = new Date().toISOString();
      target.drawn_openid = 'oMockOpenid';
      target.drawn_unionid = 'uMockUnionid';
      return ok(res, { status: 'drawn', entries: entriesOf(session, idCard, name) });
    }

    const pending = matched.filter((player) => !player.slot);
    if (pending.length === 0) {
      return ok(res, { status: 'already_drawn', entries: entriesOf(session, idCard, name) });
    }
    if (pending.length > 1) {
      return ok(res, {
        status: 'choose_genre',
        entries: entriesOf(session, idCard, name),
        choices: pending.map((player) => ({ key: player.genre, label: groupLabels[player.genre] })),
      });
    }
    pending[0].slot = nextSlot(session, pending[0]);
    pending[0].drawn_at = new Date().toISOString();
    pending[0].drawn_openid = 'oMockOpenid';
    pending[0].drawn_unionid = 'uMockUnionid';
    return ok(res, { status: 'drawn', entries: entriesOf(session, idCard, name) });
  },
};
