import type { SubmitRegistrationPayload, SubmitRegistrationResult } from "../types/models";
import { http } from "../utils/request";

/** 提交报名（POST /api/v1/events/{event_id}/registrations，API 文档 3.1）。 */
export function submitRegistration(
  eventId: number,
  payload: SubmitRegistrationPayload,
): Promise<SubmitRegistrationResult> {
  return http.post<SubmitRegistrationResult>(`/api/v1/events/${eventId}/registrations`, {
    ...payload,
  });
}
