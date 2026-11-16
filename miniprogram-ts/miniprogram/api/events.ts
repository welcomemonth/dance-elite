import type { PagedList } from "../types/api";
import type { EventDetail, EventItem, EventStatus } from "../types/models";
import { http } from "../utils/request";

/** 小程序赛事列表（GET /api/v1/events，API 文档 2.1）。 */
export function listEvents(params: {
  status?: EventStatus;
  page: number;
  page_size: number;
}): Promise<PagedList<EventItem>> {
  return http.get<PagedList<EventItem>>("/api/v1/events", {
    status: params.status,
    page: params.page,
    page_size: params.page_size,
  });
}

/** 小程序赛事详情（GET /api/v1/events/{event_id}，API 文档 2.2）。 */
export function getEventDetail(eventId: number): Promise<EventDetail> {
  return http.get<EventDetail>(`/api/v1/events/${eventId}`);
}
