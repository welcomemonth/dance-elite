import { DefineComponent, RootComponent } from "annil";

import { userStore } from "../../store/user";
import { syncTabBar } from "../../utils/tab-bar";

const rootComponent = RootComponent()({
  isPage: true,
  data: {
    loading: false,
    manualLoggedOut: false,
  },
  store: {
    user: () => userStore.profile,
    loggedIn: () => userStore.isLoggedIn,
  },
  computed: {
    displayName(): string {
      return this.data.user?.nickname || "未登录";
    },
    avatarText(): string {
      return (this.data.user?.nickname || "我").slice(0, 1);
    },
    playerNoText(): string {
      return this.data.user?.player_no || "";
    },
  },
  pageLifetimes: {
    onShow() {
      syncTabBar(this, 2);
      if (!this.data.manualLoggedOut) {
        void this.ensureLogin();
      }
    },
  },
  methods: {
    async ensureLogin(): Promise<void> {
      if (this.data.loggedIn) {
        return;
      }
      this.setData({ loading: true });
      try {
        await userStore.ensureLogin();
      } catch (error) {
        this.showError(error);
      } finally {
        this.setData({ loading: false });
      }
    },
    onLoginTap() {
      this.setData({ manualLoggedOut: false });
      void this.ensureLogin();
    },
    onLogoutTap() {
      this.setData({ manualLoggedOut: true });
      userStore.clearSession();
      this.showToast("已退出登录");
    },
    onMenuTap(event: WechatMiniprogram.TouchEvent) {
      const label = String(event.currentTarget.dataset.label ?? "");
      if (label) {
        this.showToast(`${label}将在下一批接入`);
      }
    },
  },
});

const mine = DefineComponent({
  path: "/pages/mine/mine",
  rootComponent,
});

export type $Mine = typeof mine;
