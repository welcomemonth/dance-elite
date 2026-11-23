import type {
  Materials,
  MaterialStatus,
  RegistrationListItem,
  RegistrationStatus,
  WaitlistStatus,
} from "../types/models";
import { danceTypeText, eventLevelText } from "./event";
import { formatDate } from "./format";

/** 报名列表筛选标签（样式对齐 mini-template：「全部 / 进行中 / 退款记录」）。 */
export type RegistrationFilter = "all" | "active" | "refund";

export const REGISTRATION_FILTERS: Array<{ label: string; value: RegistrationFilter }> = [
  { label: "全部", value: "all" },
  { label: "进行中", value: "active" },
  { label: "退款记录", value: "refund" },
];

export type TagTone = "default" | "yellow" | "purple" | "muted" | "success";

export interface RegistrationStatusView {
  label: string;
  tone: TagTone;
}

/** 报名状态标签；paid 时按资料完成度区分「待补充资料 / 报名成功」。 */
export function registrationStatusView(
  status: RegistrationStatus,
  materialStatus?: MaterialStatus,
): RegistrationStatusView {
  if (status === "refunding") {
    return { label: "退款处理中", tone: "purple" };
  }
  if (status === "refunded") {
    return { label: "已退款", tone: "muted" };
  }
  if (status === "withdrawn") {
    return { label: "已退赛", tone: "muted" };
  }
  const missing = materialStatus?.missing.length ?? 0;
  return missing > 0
    ? { label: "待补充资料", tone: "yellow" }
    : { label: "报名成功", tone: "success" };
}

/** 资料完成度摘要（服务端已计算缺失项，前端只做展示）。 */
export function materialSummary(status: MaterialStatus): string {
  return status.missing.length > 0 ? `资料待补充 ${status.missing.length} 项` : "资料已齐";
}

export function registrationMatchesFilter(
  item: RegistrationListItem,
  filter: RegistrationFilter,
): boolean {
  if (filter === "active") {
    return item.status === "paid";
  }
  if (filter === "refund") {
    return item.status === "refunding" || item.status === "refunded";
  }
  return true;
}

/** 报名列表卡片展示模型。 */
export interface RegistrationCardView extends RegistrationListItem {
  levelText: string;
  dateText: string;
  danceText: string;
  statusLabel: string;
  statusTone: TagTone;
  materialText: string;
}

export function registrationCardView(item: RegistrationListItem): RegistrationCardView {
  const statusView = registrationStatusView(item.status, item.material_status);
  return {
    ...item,
    levelText: eventLevelText(item.level),
    dateText: item.event_date ? formatDate(item.event_date, "YYYY.MM.DD") : "日期待定",
    danceText: danceTypeText(item.dance_type),
    statusLabel: statusView.label,
    statusTone: statusView.tone,
    materialText: materialSummary(item.material_status),
  };
}

/** 参赛资料 8 项的展示顺序与名称（API 文档 3.4 materials 快照）。 */
export const MATERIAL_ITEMS: Array<{ key: keyof Materials; label: string; hint: string }> = [
  { key: "photo_url", label: "证件照", hint: "PNG / JPG，≤10MB" },
  { key: "audio_url", label: "舞蹈音频", hint: "MP3，≤100MB，时长 ≤4 分钟" },
  { key: "program_name", label: "节目名称", hint: "≤50 字" },
  { key: "teacher_name", label: "指导教师", hint: "选填，≤30 字" },
  { key: "contact_phone", label: "联系电话", hint: "11 位手机号" },
  { key: "express_address", label: "快递地址", hint: "选填，证书 / 奖牌邮寄" },
  { key: "work_intro", label: "作品说明", hint: "选填，≤500 字" },
  { key: "special_req", label: "特殊要求", hint: "选填，≤200 字" },
];

/** 资料项是否已上传（文本为空串也视为未上传）。 */
export function materialItemValue(materials: Materials, key: keyof Materials): string {
  const value = materials[key];
  return typeof value === "string" ? value.trim() : "";
}

/** 详情页资料清单展示模型。 */
export interface MaterialRowView {
  key: string;
  label: string;
  hint: string;
  value: string;
  uploaded: boolean;
}

export function materialRows(materials: Materials): MaterialRowView[] {
  return MATERIAL_ITEMS.map((item) => {
    const value = materialItemValue(materials, item.key);
    let display = value;
    if (item.key === "photo_url") {
      display = value ? "已上传" : "";
    } else if (item.key === "audio_url") {
      display = value ? materials.audio_filename || "已上传" : "";
    }
    return {
      key: item.key,
      label: item.label,
      hint: item.hint,
      value: display,
      uploaded: value.length > 0,
    };
  });
}

/** 候补状态标签。 */
export interface WaitlistStatusView {
  label: string;
  tone: TagTone;
}

export function waitlistStatusView(status: WaitlistStatus): WaitlistStatusView {
  switch (status) {
    case "notified":
      return { label: "已通知", tone: "purple" };
    case "converted":
      return { label: "已转报名", tone: "success" };
    case "given_up":
      return { label: "已放弃", tone: "muted" };
    case "removed":
      return { label: "已移除", tone: "muted" };
    default:
      return { label: "候补中", tone: "yellow" };
  }
}
