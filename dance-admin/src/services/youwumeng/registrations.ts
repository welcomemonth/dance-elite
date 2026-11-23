import { request } from '@umijs/max';
import { downloadFile } from './download';
import type { AdminRegistrationItem, AgeLevel, DanceType, PagedList } from './typings';

export interface AdminRegistrationListParams {
  event_id?: number;
  group_id?: number;
  level?: AgeLevel;
  dance_type?: DanceType;
  status?: string;
  material_state?: 'complete' | 'incomplete';
  missing_item?: string;
  keyword?: string;
  page?: number;
  page_size?: number;
}

/** GET /api/v1/admin/registrations（后台报名名单） */
export async function listAdminRegistrations(
  params: AdminRegistrationListParams,
): Promise<PagedList<AdminRegistrationItem>> {
  return request('/api/v1/admin/registrations', { method: 'GET', params });
}

/** GET /api/v1/admin/events/{event_id}/registrations/export（选手名单 Excel） */
export async function exportEventRegistrations(
  eventId: number,
  params?: { include_canceled?: boolean },
): Promise<void> {
  return downloadFile(`/api/v1/admin/events/${eventId}/registrations/export`, params);
}

/** POST /api/v1/admin/registrations/{registration_id}/withdraw（人工退赛，仅释放名额） */
export async function withdrawRegistration(
  registrationId: number,
  reason?: string,
): Promise<{ registration_id: number; status: string; released: boolean }> {
  return request(`/api/v1/admin/registrations/${registrationId}/withdraw`, {
    method: 'POST',
    data: { reason },
  });
}
