import { request } from '@umijs/max';
import type { ContentEntry, ContentKey, ContentValue } from './typings';

/** GET /api/v1/admin/contents/{key} */
export async function getContent(key: ContentKey): Promise<ContentEntry> {
  return request(`/api/v1/admin/contents/${key}`, { method: 'GET' });
}

/** PUT /api/v1/admin/contents/{key} */
export async function putContent(key: ContentKey, value: ContentValue): Promise<ContentEntry> {
  return request(`/api/v1/admin/contents/${key}`, { method: 'PUT', data: { value } });
}

/** POST /api/v1/admin/contents/{key}/image（公众号二维码图片上传，S5-5） */
export async function uploadContentImage(key: ContentKey, file: File): Promise<ContentEntry> {
  const form = new FormData();
  form.append('file', file);
  return request(`/api/v1/admin/contents/${key}/image`, { method: 'POST', data: form });
}
