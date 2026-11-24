import type { ContentEntry, ContentKey } from "../types/models";
import { http } from "../utils/request";

/** 公开内容读取（GET /api/v1/contents/{key}，免登录，S5-4）。 */
export function getContent(key: ContentKey): Promise<ContentEntry> {
  return http.get<ContentEntry>(`/api/v1/contents/${key}`, undefined, { auth: false });
}
