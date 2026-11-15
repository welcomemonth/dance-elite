import { CustomComponent, DefineComponent, RootComponent } from "annil";

import type { $EmptyState } from "../../components/empty-state/empty-state";
import { userStore } from "../../store/user";

const rootComponent = RootComponent()({
  isPage: true,
  data: {
    loading: false,
  },
  store: {
    user: () => userStore.profile,
    loggedIn: () => userStore.isLoggedIn,
  },
  computed: {
    greeting(): string {
      const nickname = this.data.user?.nickname;
      return nickname ? `你好，${nickname}` : "你好，欢迎来到优舞盟";
    },
  },
  pageLifetimes: {
    onLoad() {
      // 启动时的静默登录可能尚未结束，这里复用同一个登录 Promise。
      void this.ensureLogin();
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
      void this.ensureLogin();
    },
    onLogoutTap() {
      userStore.clearSession();
      this.showToast("已退出登录");
    },
  },
});

type Root = typeof rootComponent;

const emptyState = CustomComponent<Root, $EmptyState>()({
  events: {
    emptyState_retry() {
      this.showToast("empty-state 的 retry 事件已触发");
    },
  },
});

const home = DefineComponent({
  path: "/pages/home/home",
  rootComponent,
  subComponents: [emptyState],
});

export type $Home = typeof home;
