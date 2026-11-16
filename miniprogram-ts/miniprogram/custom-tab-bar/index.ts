const TAB_PATHS = [
  "/pages/events/events",
  "/pages/ranking/ranking",
  "/pages/mine/mine",
] as const;

Component({
  data: {
    selected: 0,
  },
  pageLifetimes: {
    show() {
      const pages = getCurrentPages();
      const current = pages[pages.length - 1];
      if (!current) {
        return;
      }
      const index = TAB_PATHS.findIndex((path) => path.slice(1) === current.route);
      if (index >= 0 && index !== this.data.selected) {
        this.setData({ selected: index });
      }
    },
  },
  methods: {
    setSelected(index: number) {
      if (index < 0 || index >= TAB_PATHS.length) {
        return;
      }
      this.setData({ selected: index });
    },
    onTap(event: WechatMiniprogram.TouchEvent) {
      const index = Number(event.currentTarget.dataset.index ?? 0);
      if (index < 0 || index >= TAB_PATHS.length || index === this.data.selected) {
        return;
      }
      wx.switchTab({
        url: TAB_PATHS[index],
        success: () => {
          this.setData({ selected: index });
        },
      });
    },
  },
});
