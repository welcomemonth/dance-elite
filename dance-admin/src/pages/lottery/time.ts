const pad = (value: number) => String(value).padStart(2, '0');

/** 把 ISO 时间格式化为「11月23日 10:12」；非法值原样返回。 */
export function formatDrawnAt(iso?: string): string {
  if (!iso) {
    return '';
  }
  const ts = Date.parse(iso);
  if (Number.isNaN(ts)) {
    return iso;
  }
  const date = new Date(ts);
  return `${date.getMonth() + 1}月${date.getDate()}日 ${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}`;
}
