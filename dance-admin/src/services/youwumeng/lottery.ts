import { request } from '@umijs/max';
import { downloadFile } from './download';
import type {
  LotteryImportResult,
  LotterySession,
  LotterySessionDetail,
  LotterySettingsInput,
} from './typings';

/** GET /api/v1/admin/lottery/sessions */
export async function listLotterySessions(): Promise<{ list: LotterySession[] }> {
  return request('/api/v1/admin/lottery/sessions', { method: 'GET' });
}

/** GET /api/v1/admin/lottery/sessions/{token} */
export async function getLotterySession(token: string): Promise<LotterySessionDetail> {
  return request(`/api/v1/admin/lottery/sessions/${token}`, { method: 'GET' });
}

/** POST /api/v1/admin/lottery/sessions（multipart：Excel + 赛事 + 时间窗口） */
export async function importLotterySession(form: FormData): Promise<LotteryImportResult> {
  // 不手动设置 Content-Type，交给浏览器生成 multipart boundary。
  const res = await request('/api/v1/admin/lottery/sessions', {
    method: 'POST',
    data: form,
  } as never);
  return res as unknown as LotteryImportResult;
}

/** PUT /api/v1/admin/lottery/sessions/{token} */
export async function updateLotterySession(
  token: string,
  data: LotterySettingsInput,
): Promise<LotterySession> {
  return request(`/api/v1/admin/lottery/sessions/${token}`, { method: 'PUT', data });
}

/** GET /api/v1/admin/lottery/sessions/{token}/export：导出抽签结果 Excel。 */
export async function exportLotterySession(token: string, filename: string): Promise<void> {
  await downloadFile(`/api/v1/admin/lottery/sessions/${token}/export`, undefined, filename);
}
