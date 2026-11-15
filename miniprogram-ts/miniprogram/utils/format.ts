export type DateInput = string | number | Date;

function toDate(input: DateInput): Date | null {
  if (input instanceof Date) {
    return Number.isNaN(input.getTime()) ? null : input;
  }
  if (typeof input === "number") {
    const date = new Date(input);
    return Number.isNaN(date.getTime()) ? null : date;
  }
  // iOS 对 "YYYY-MM-DD HH:mm:ss" 解析不稳定，统一转成 ISO 风格。
  const normalized = input.includes("T") ? input : input.replace(" ", "T");
  const date = new Date(normalized);
  return Number.isNaN(date.getTime()) ? null : date;
}

function pad(value: number): string {
  return String(value).padStart(2, "0");
}

/**
 * 日期格式化，支持 YYYY / MM / DD / HH / mm / ss。
 * 无效日期返回空字符串。
 */
export function formatDate(input: DateInput, pattern = "YYYY-MM-DD"): string {
  const date = toDate(input);
  if (!date) {
    return "";
  }
  return pattern.replace(/YYYY|MM|DD|HH|mm|ss/g, (token) => {
    switch (token) {
      case "YYYY":
        return String(date.getFullYear());
      case "MM":
        return pad(date.getMonth() + 1);
      case "DD":
        return pad(date.getDate());
      case "HH":
        return pad(date.getHours());
      case "mm":
        return pad(date.getMinutes());
      case "ss":
        return pad(date.getSeconds());
      default:
        return token;
    }
  });
}

export function formatDateTime(input: DateInput): string {
  return formatDate(input, "YYYY-MM-DD HH:mm");
}

export function formatTime(input: DateInput): string {
  return formatDate(input, "HH:mm");
}

/**
 * 金额格式化：后端金额单位为分。
 * formatMoney(98000) => "¥980.00"
 */
export function formatMoney(cents: number, withSymbol = true): string {
  const value = Math.round(Number(cents) || 0);
  const sign = value < 0 ? "-" : "";
  const abs = Math.abs(value);
  const yuan = Math.floor(abs / 100);
  const fen = abs % 100;
  return `${sign}${withSymbol ? "¥" : ""}${yuan}.${pad(fen)}`;
}

/** 手机号脱敏：13800138000 => 138****8000 */
export function formatPhone(phone: string | null | undefined): string {
  if (!phone) {
    return "";
  }
  if (phone.length !== 11) {
    return phone;
  }
  return `${phone.slice(0, 3)}****${phone.slice(7)}`;
}

/** 秒数格式化为 mm:ss，用于音频时长等展示。 */
export function formatDuration(totalSeconds: number): string {
  const seconds = Math.max(0, Math.floor(totalSeconds));
  return `${pad(Math.floor(seconds / 60))}:${pad(seconds % 60)}`;
}
