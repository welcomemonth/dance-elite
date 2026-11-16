/** 自定义 tabBar 的最小调用契约（由 custom-tab-bar/index.ts 实现）。 */
export interface TabBarHandle {
  setSelected(index: number): void;
}

type PageWithTabBar = {
  getTabBar?: () => TabBarHandle | null | undefined;
};

/** 同步自定义 tabBar 的选中态；页面 onShow 时调用。 */
export function syncTabBar(page: unknown, index: number): void {
  const getTabBar = (page as PageWithTabBar).getTabBar;
  if (typeof getTabBar !== "function") {
    return;
  }
  getTabBar.call(page)?.setSelected(index);
}
