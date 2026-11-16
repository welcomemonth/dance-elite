import { API_BASE_URL } from "../config/env";

/** 把后端返回的 /static/... 相对路径转成可访问的绝对地址。 */
export function absoluteUrl(path: string | null | undefined): string {
  if (!path) {
    return "";
  }
  if (/^https?:\/\//i.test(path)) {
    return path;
  }
  return `${API_BASE_URL}${path.startsWith("/") ? "" : "/"}${path}`;
}
