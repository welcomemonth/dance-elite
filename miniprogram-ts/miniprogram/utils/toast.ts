export type ToastIcon = "none" | "success" | "error" | "loading";

export function showToast(title: string, icon: ToastIcon = "none", duration = 2000): void {
  wx.showToast({ title, icon, duration });
}

/** 把任意异常转成用户可读的 toast。 */
export function showError(error: unknown): void {
  const message = error instanceof Error ? error.message : String(error ?? "操作失败");
  wx.showToast({ title: message || "操作失败", icon: "none", duration: 2500 });
}
