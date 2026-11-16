import type {
  DanceType,
  EventItem,
  EventLevel,
  EventStatus,
  GroupLevel,
} from "../types/models";
import { formatChineseDate, formatDate, formatMoney } from "./format";

export const EVENT_STATUS_OPTIONS: Array<{ label: string; value: "" | EventStatus }> = [
  { label: "全部", value: "" },
  { label: "报名中", value: "registering" },
  { label: "即将开始", value: "upcoming" },
  { label: "报名截止", value: "reg_closed" },
  { label: "已结束", value: "finished" },
];

const EVENT_STATUS_LABELS: Record<EventStatus, string> = {
  upcoming: "即将开始",
  registering: "报名中",
  reg_closed: "报名截止",
  finished: "已结束",
};

const EVENT_LEVEL_LABELS: Record<EventLevel, string> = {
  grade_a: "甲级赛",
  super: "超级赛",
  final: "年度总决赛",
};

const DANCE_TYPE_LABELS: Record<DanceType, string> = {
  classical: "古典舞",
  folk: "民族民间舞",
};

export function eventStatusText(status: EventStatus): string {
  return EVENT_STATUS_LABELS[status];
}

export function eventLevelText(level: EventLevel): string {
  return EVENT_LEVEL_LABELS[level];
}

export function danceTypeText(danceType: DanceType): string {
  return DANCE_TYPE_LABELS[danceType];
}

export function groupLevelText(level: GroupLevel): string {
  return level;
}

export function formatEventDate(value: string | null): string {
  return value ? formatDate(value, "YYYY.MM.DD") : "日期待定";
}

export function formatEventDeadline(value: string | null): string {
  return value ? formatChineseDate(value) : "";
}

export function formatEventFee(fee: number): string {
  if (fee <= 0) {
    return "免费";
  }
  const yuan = fee / 100;
  return Number.isInteger(yuan) ? `¥${yuan}` : formatMoney(fee);
}

/** 报名热度提示：优先展示剩余名额最少的组别。 */
export function eventHotText(event: EventItem): string {
  let hottest: { level: GroupLevel; danceType: DanceType; remaining: number } | null = null;
  for (const group of event.group_summary) {
    const remaining = Math.max(0, group.quota - group.registered);
    if (!hottest || remaining < hottest.remaining) {
      hottest = { level: group.level, danceType: group.dance_type, remaining };
    }
  }
  if (!hottest || hottest.remaining > 3) {
    return "";
  }
  return hottest.remaining === 0
    ? `${hottest.level} ${danceTypeText(hottest.danceType)}已满`
    : `${hottest.level} ${danceTypeText(hottest.danceType)}仅剩 ${hottest.remaining} 席`;
}

/** 赛事列表卡片的展示模型。 */
export interface EventCardView extends EventItem {
  levelText: string;
  statusText: string;
  dateText: string;
  feeText: string;
  hotText: string;
  featured: boolean;
}

export function eventCardView(event: EventItem, index: number, allFilter: boolean): EventCardView {
  return {
    ...event,
    levelText: eventLevelText(event.level),
    statusText: eventStatusText(event.status),
    dateText: formatEventDate(event.event_date),
    feeText: formatEventFee(event.fee),
    hotText: eventHotText(event),
    featured: allFilter && index === 0,
  };
}
