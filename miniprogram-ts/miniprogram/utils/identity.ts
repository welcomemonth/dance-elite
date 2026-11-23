import type { EventDetail, GroupLevel } from "../types/models";

export interface IdentityGroup {
  level: GroupLevel;
  birthYear: number;
  birthFrom: number;
  birthTo: number;
}

const ID_CARD_PATTERN = /^\d{17}[\dXx]$/;

/**
 * 从 18 位大陆身份证号解析出生年份。
 * 只做前端即时提示，最终组别以服务端判定为准；格式或日期段非法时返回 null。
 */
export function birthYearFromIdCard(idCard: string): number | null {
  const value = idCard.trim().toUpperCase();
  if (!ID_CARD_PATTERN.test(value)) {
    return null;
  }
  const year = Number(value.slice(6, 10));
  const month = Number(value.slice(10, 12));
  const day = Number(value.slice(12, 14));
  if (year < 1900 || month < 1 || month > 12 || day < 1 || day > 31) {
    return null;
  }
  const date = new Date(year, month - 1, day);
  if (date.getFullYear() !== year || date.getMonth() !== month - 1 || date.getDate() !== day) {
    return null;
  }
  return year;
}

/** 按赛事组别配置（出生年份区间）匹配年龄组；未匹配返回 null。 */
export function groupForEvent(event: EventDetail, idCard: string): IdentityGroup | null {
  const birthYear = birthYearFromIdCard(idCard);
  if (birthYear === null) {
    return null;
  }
  for (const group of event.groups) {
    if (!group.enabled) {
      continue;
    }
    if (birthYear >= group.birth_year_from && birthYear <= group.birth_year_to) {
      return {
        level: group.level,
        birthYear,
        birthFrom: group.birth_year_from,
        birthTo: group.birth_year_to,
      };
    }
  }
  return null;
}

/** 身份证脱敏展示：前 6 后 4，中间 8 位星号。 */
export function maskIdCard(idCard: string): string {
  const value = idCard.trim();
  if (value.length < 10) {
    return value ? "****" : "";
  }
  return `${value.slice(0, 6)}********${value.slice(-4)}`;
}
