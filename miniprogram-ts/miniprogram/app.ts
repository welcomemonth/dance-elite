import "./inject";

import { userStore } from "./store/user";
import { setUnauthorizedHandler } from "./utils/request";

App<IAppOption>({
  globalData: {
    launchTime: Date.now(),
  },
  onLaunch() {
    // token 失效时统一清理本地登录态，页面通过 store 自动刷新。
    setUnauthorizedHandler(() => {
      userStore.clearSession();
    });

    // 微信静默登录：不阻塞启动。后端不可用时只记录日志，页面可手动重试。
    void userStore.ensureLogin().catch((error: unknown) => {
      console.warn("[app] 静默登录失败", error);
    });
  },
});
