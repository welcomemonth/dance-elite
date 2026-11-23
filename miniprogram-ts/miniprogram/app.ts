import "./inject";

import { userStore } from "./store/user";
import { setUnauthorizedHandler } from "./utils/request";

function silentLogin(reason: string): void {
  void userStore.ensureLogin().catch((error: unknown) => {
    console.warn(`[app] ${reason}静默登录失败`, error);
  });
}

App<IAppOption>({
  globalData: {
    launchTime: Date.now(),
  },
  onLaunch() {
    // token 失效/过期时统一清理本地登录态，并立即重新静默登录；
    // 请求层会在重新登录成功后自动重试原请求一次。
    setUnauthorizedHandler(async () => {
      userStore.clearSession();
      await userStore.ensureLogin();
    });

    // 启动即静默登录：本地无会话或 token 已过期时都会真正请求 wx-login。
    silentLogin("启动");
  },
  onShow() {
    // 从后台切回前台时检查 token；过期则自动重新登录（未过期时是空操作）。
    silentLogin("回前台");
  },
});
