import type { RegistrationDetail, UpdateMaterialsPayload } from "../../types/models";

/** 资料编辑表单的 6 项文本字段（与文案字段名解耦，WXML 用 camelCase）。 */
export const EMPTY_FORM = {
  programName: "",
  teacherName: "",
  contactPhone: "",
  expressAddress: "",
  workIntro: "",
  specialReq: "",
};

export type MaterialForm = typeof EMPTY_FORM;
export type FormField = keyof MaterialForm;

/** 联系电话格式（与服务端 4.1 校验一致，允许留空）。 */
const PHONE_PATTERN = /^1[3-9][0-9]{9}$/;

export function formFromDetail(detail: RegistrationDetail): MaterialForm {
  const materials = detail.materials;
  return {
    programName: materials.program_name ?? "",
    teacherName: materials.teacher_name ?? "",
    contactPhone: materials.contact_phone ?? "",
    expressAddress: materials.express_address ?? "",
    workIntro: materials.work_intro ?? "",
    specialReq: materials.special_req ?? "",
  };
}

/** 表单提交前的本地校验；返回错误文案，合法时返回 null。 */
export function validateForm(form: MaterialForm): string | null {
  const phone = form.contactPhone.trim();
  if (phone && !PHONE_PATTERN.test(phone)) {
    return "联系电话格式不正确（11 位手机号）";
  }
  return null;
}

/** 表单转 API 请求体；空串表示清空该字段。 */
export function formPayload(form: MaterialForm): UpdateMaterialsPayload {
  return {
    program_name: form.programName.trim(),
    teacher_name: form.teacherName.trim(),
    contact_phone: form.contactPhone.trim(),
    express_address: form.expressAddress.trim(),
    work_intro: form.workIntro.trim(),
    special_req: form.specialReq.trim(),
  };
}
