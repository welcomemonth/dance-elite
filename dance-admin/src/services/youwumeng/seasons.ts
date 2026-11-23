import { request } from '@umijs/max';
import type {
  AgeLevel,
  DanceType,
  FinalsRosterRow,
  InstitutionRankRow,
  Leaderboard,
  PrizeRules,
  PrizeStatementItem,
  QualificationCard,
  ScoreEntry,
  ScorePreview,
  ScoreRules,
  Season,
} from './typings';

/**
 * 榜单中心接口（赛季/成绩/积分榜单/晋级/机构/奖金）。
 * *待定*：API 接口设计文档尚未覆盖本模块，路径按统一约定设计，
 * 接入真实后端时以最终文档为准，前端只需改这里。
 */

// ---------- 赛季 ----------

export async function listSeasons(): Promise<Season[]> {
  const data = await request<{ list: Season[] }>('/api/v1/admin/seasons', { method: 'GET' });
  return data.list;
}

export async function createSeason(data: {
  name: string;
  start_date: string;
  end_date: string;
}): Promise<Season> {
  return request('/api/v1/admin/seasons', { method: 'POST', data });
}

export async function updateSeason(
  seasonId: number,
  data: { name?: string; start_date?: string; end_date?: string },
): Promise<Season> {
  return request(`/api/v1/admin/seasons/${seasonId}`, { method: 'PUT', data });
}

/** 归档赛季：PUT /seasons/{id} 携带 status=archived（与 API 文档 10.7 一致，name 必填）。 */
export async function archiveSeason(season: Season): Promise<Season> {
  return request(`/api/v1/admin/seasons/${season.id}`, {
    method: 'PUT',
    data: { name: season.name, status: 'archived' },
  });
}

// ---------- 积分规则 ----------

export async function getScoreRules(seasonId: number): Promise<ScoreRules> {
  return request(`/api/v1/admin/seasons/${seasonId}/score-rules`, { method: 'GET' });
}

export async function putScoreRules(seasonId: number, rules: ScoreRules): Promise<ScoreRules> {
  return request(`/api/v1/admin/seasons/${seasonId}/score-rules`, { method: 'PUT', data: rules });
}

// ---------- 成绩录入 ----------

export async function listScoreEntries(
  eventId: number,
  level: AgeLevel,
  danceType: DanceType,
): Promise<ScoreEntry[]> {
  return request(`/api/v1/admin/events/${eventId}/scores`, {
    method: 'GET',
    params: { level, dance_type: danceType },
  });
}

export interface ScorePreviewInput {
  rows: {
    player_name: string;
    id_card_no?: string;
    award?: string;
    advanced_to_final?: boolean;
    remark?: string;
  }[];
}

/** 解析预览：Excel 内容（前端解析后上传行数据）或在线录入行，按证件号匹配选手 */
export async function previewScores(
  eventId: number,
  level: AgeLevel,
  danceType: DanceType,
  data: ScorePreviewInput,
): Promise<ScorePreview> {
  return request(`/api/v1/admin/events/${eventId}/scores/preview`, {
    method: 'POST',
    params: { level, dance_type: danceType },
    data,
  });
}

/** 确认发布成绩（二次确认后写入并触发积分重算） */
export async function publishScores(
  eventId: number,
  level: AgeLevel,
  danceType: DanceType,
  rows: ScorePreview['rows'],
): Promise<{ published: number }> {
  return request(`/api/v1/admin/events/${eventId}/scores`, {
    method: 'POST',
    params: { level, dance_type: danceType },
    data: { rows },
  });
}

/** 成绩纠错：旧成绩置 corrected，写入新成绩并自动重算（返回新成绩 ID）。 */
export async function correctScore(
  entryId: number,
  data: { award?: string; advanced_to_final?: boolean; remark?: string },
): Promise<{ entry_id: number }> {
  return request(`/api/v1/admin/scores/${entryId}`, { method: 'PUT', data });
}

// ---------- 积分与榜单 ----------

export async function listLeaderboards(seasonId: number): Promise<Leaderboard[]> {
  const data = await request<{ list: Leaderboard[] }>(
    `/api/v1/admin/seasons/${seasonId}/leaderboards`,
    { method: 'GET' },
  );
  return data.list;
}

/** 更新上榜行：展示名 / 隐藏恢复 */
export async function updateLeaderboardRow(
  seasonId: number,
  level: AgeLevel,
  danceType: DanceType,
  playerId: number,
  data: { display_name?: string; hidden?: boolean },
): Promise<{ player_id: number }> {
  return request(
    `/api/v1/admin/seasons/${seasonId}/leaderboards/${level}/${danceType}/players/${playerId}`,
    { method: 'PUT', data },
  );
}

// ---------- 晋级管理 ----------

export async function listQualificationCards(seasonId: number): Promise<QualificationCard[]> {
  const data = await request<{ list: QualificationCard[] }>(
    `/api/v1/admin/seasons/${seasonId}/qualification-cards`,
    { method: 'GET' },
  );
  return data.list;
}

export async function grantQualificationCard(
  seasonId: number,
  data: { player_id: number; board_id: number; reason: string },
): Promise<{ card_id: number }> {
  return request(`/api/v1/admin/seasons/${seasonId}/qualification-cards`, { method: 'POST', data });
}

export async function revokeQualificationCard(
  seasonId: number,
  cardId: number,
): Promise<{ card_id: number }> {
  return request(`/api/v1/admin/seasons/${seasonId}/qualification-cards/${cardId}`, {
    method: 'DELETE',
  });
}

export async function getFinalsRoster(seasonId: number): Promise<FinalsRosterRow[]> {
  const data = await request<{ list: FinalsRosterRow[] }>(
    `/api/v1/admin/seasons/${seasonId}/finals-roster`,
    { method: 'GET' },
  );
  return data.list;
}

/** 总决赛名单手动调整（增补/移除，留痕） */
export async function adjustFinalsRoster(
  seasonId: number,
  data: { board_id: number; player_id: number; action: 'add' | 'remove'; reason: string },
): Promise<{ adjusted: boolean }> {
  return request(`/api/v1/admin/seasons/${seasonId}/finals-roster/adjust`, { method: 'POST', data });
}

// ---------- 机构年度榜 ----------

export async function getInstitutionRanking(seasonId: number): Promise<InstitutionRankRow[]> {
  const data = await request<{ list: InstitutionRankRow[] }>(
    `/api/v1/admin/seasons/${seasonId}/institution-ranking`,
    { method: 'GET' },
  );
  return data.list;
}

// ---------- 奖金管理 ----------

export async function getPrizeRules(seasonId: number): Promise<PrizeRules> {
  return request(`/api/v1/admin/seasons/${seasonId}/prize-rules`, { method: 'GET' });
}

export async function putPrizeRules(seasonId: number, rules: PrizeRules): Promise<PrizeRules> {
  return request(`/api/v1/admin/seasons/${seasonId}/prize-rules`, { method: 'PUT', data: rules });
}

export interface PrizeStatementParams {
  season_id: number;
  event_id?: number;
  paid?: boolean;
  page?: number;
  page_size?: number;
}

export async function listPrizeStatements(params: PrizeStatementParams): Promise<{
  list: PrizeStatementItem[];
  total: number;
  total_amount: number;
}> {
  return request('/api/v1/admin/prize-statements', { method: 'GET', params });
}

/** 人工微调奖金金额（留痕） */
export async function adjustPrizeStatement(
  itemId: number,
  data: { amount: number; reason: string },
): Promise<{ item_id: number }> {
  return request(`/api/v1/admin/prize-statements/${itemId}`, { method: 'PUT', data });
}

/** 发放登记 */
export async function payPrizeStatement(
  itemId: number,
  data: { payee: string; note?: string },
): Promise<{ item_id: number; paid: boolean }> {
  return request(`/api/v1/admin/prize-statements/${itemId}/pay`, { method: 'POST', data });
}
