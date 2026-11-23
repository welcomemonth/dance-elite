import type { PagedList } from "../types/api";
import type {
  SubmitWaitlistPayload,
  SubmitWaitlistResult,
  WaitlistItem,
} from "../types/models";
import { http } from "../utils/request";

/** 提交候补（POST /api/v1/events/{event_id}/waitlists，API 文档 5.1）。 */
export function submitWaitlist(
  eventId: number,
  payload: SubmitWaitlistPayload,
): Promise<SubmitWaitlistResult> {
  return http.post<SubmitWaitlistResult>(`/api/v1/events/${eventId}/waitlists`, payload);
}

/** 我的候补列表（GET /api/v1/waitlists，API 文档 5.2）。 */
export function listWaitlists(params: {
  page: number;
  page_size: number;
}): Promise<PagedList<WaitlistItem>> {
  return http.get<PagedList<WaitlistItem>>("/api/v1/waitlists", params);
}

/** 一次拉取本人全部候补，供「我的」页角标与候补列表使用。 */
export async function listAllWaitlists(): Promise<WaitlistItem[]> {
  const pageSize = 100;
  const first = await listWaitlists({ page: 1, page_size: pageSize });
  const all = [...first.list];
  while (all.length < first.total) {
    const next = await listWaitlists({
      page: Math.floor(all.length / pageSize) + 1,
      page_size: pageSize,
    });
    if (next.list.length === 0) {
      break;
    }
    all.push(...next.list);
  }
  return all;
}

/** 取消候补（DELETE /api/v1/waitlists/{id}，API 文档 5.3；仅排队中可取消）。 */
export function cancelWaitlist(waitlistId: number): Promise<null> {
  return http.delete<null>(`/api/v1/waitlists/${waitlistId}`);
}
