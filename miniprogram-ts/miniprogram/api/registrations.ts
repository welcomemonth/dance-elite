import type { PagedList } from "../types/api";
import type {
  RegistrationDetail,
  RegistrationListItem,
  SubmitRegistrationPayload,
  SubmitRegistrationResult,
  UpdateMaterialsPayload,
  UpdateMaterialsResult,
} from "../types/models";
import { http } from "../utils/request";
import { uploadFile } from "../utils/upload";

/** 提交报名（POST /api/v1/events/{event_id}/registrations，API 文档 3.1）。 */
export function submitRegistration(
  eventId: number,
  payload: SubmitRegistrationPayload,
): Promise<SubmitRegistrationResult> {
  return http.post<SubmitRegistrationResult>(`/api/v1/events/${eventId}/registrations`, {
    ...payload,
  });
}

/** 我的报名列表（GET /api/v1/registrations，API 文档 3.3）。 */
export function listRegistrations(params: {
  page: number;
  page_size: number;
}): Promise<PagedList<RegistrationListItem>> {
  return http.get<PagedList<RegistrationListItem>>("/api/v1/registrations", params);
}

/**
 * 一次拉取本人全部报名：接口无业务筛选，且单个用户的报名量很小，
 * 「我的」页角标与报名列表的筛选需要完整数据，避免分页筛选缺项。
 */
export async function listAllRegistrations(): Promise<RegistrationListItem[]> {
  const pageSize = 100;
  const first = await listRegistrations({ page: 1, page_size: pageSize });
  const all = [...first.list];
  while (all.length < first.total) {
    const next = await listRegistrations({
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

/** 报名详情（GET /api/v1/registrations/{id}，API 文档 3.4）。 */
export function getRegistrationDetail(registrationId: number): Promise<RegistrationDetail> {
  return http.get<RegistrationDetail>(`/api/v1/registrations/${registrationId}`);
}

/** 更新 6 项文本资料（PUT /api/v1/registrations/{id}/materials，API 文档 4.1）。 */
export function updateMaterials(
  registrationId: number,
  payload: UpdateMaterialsPayload,
): Promise<UpdateMaterialsResult> {
  return http.put<UpdateMaterialsResult>(
    `/api/v1/registrations/${registrationId}/materials`,
    payload,
  );
}

/**
 * 上传/替换证件照（API 文档 4.2）。
 * 微信 wx.uploadFile 仅支持 POST，后端同时注册了 POST 与 PUT。
 */
export function uploadMaterialPhoto(
  registrationId: number,
  filePath: string,
): Promise<UpdateMaterialsResult> {
  return uploadFile<UpdateMaterialsResult>({
    path: `/api/v1/registrations/${registrationId}/materials/photo`,
    filePath,
    name: "photo",
  });
}

/** 上传/替换舞蹈音频（API 文档 4.3，后端同时接受 POST 与 PUT）。 */
export function uploadMaterialAudio(
  registrationId: number,
  filePath: string,
): Promise<UpdateMaterialsResult> {
  return uploadFile<UpdateMaterialsResult>({
    path: `/api/v1/registrations/${registrationId}/materials/audio`,
    filePath,
    name: "audio",
  });
}
