import { DefineComponent, RootComponent } from "annil";

function readNavMetrics(): { statusBarHeight: number; navHeight: number; totalHeight: number } {
  let statusBarHeight = 20;
  try {
    statusBarHeight = wx.getWindowInfo().statusBarHeight ?? 20;
  } catch {
    try {
      statusBarHeight = wx.getSystemInfoSync().statusBarHeight ?? 20;
    } catch {
      statusBarHeight = 20;
    }
  }

  let navHeight = 44;
  try {
    const menu = wx.getMenuButtonBoundingClientRect();
    if (menu && menu.height > 0 && menu.top >= statusBarHeight) {
      navHeight = (menu.top - statusBarHeight) * 2 + menu.height;
    }
  } catch {
    navHeight = 44;
  }

  return { statusBarHeight, navHeight, totalHeight: statusBarHeight + navHeight };
}

const rootComponent = RootComponent()({
  properties: {
    title: { type: String, value: "" },
    showBack: { type: Boolean, value: true },
    bare: { type: Boolean, value: false },
    bg: { type: String, value: "#fffdf8" },
  },
  data: {
    statusBarHeight: 20,
    navHeight: 44,
    totalHeight: 64,
  },
  lifetimes: {
    attached() {
      const metrics = readNavMetrics();
      this.setData(metrics);
    },
  },
  methods: {
    onBackTap() {
      if (!this.data.showBack) {
        return;
      }
      wx.navigateBack({ delta: 1 });
    },
  },
});

const navBar = DefineComponent({
  name: "navBar",
  rootComponent,
});

export type $NavBar = typeof navBar;
