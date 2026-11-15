import { instanceConfig } from "annil";

import { ENV_VERSION } from "./config/env";
import { showError, showToast } from "./utils/toast";

const injectData = {
  appName: "优舞盟",
  appVersion: "0.1.0",
  envVersion: ENV_VERSION,
};

const injectMethods = {
  showToast,
  showError,
};

// 让所有组件/页面的 this.data 与 this 获得注入字段的类型提示。
declare module "annil" {
  interface IInjectInfo {
    data: typeof injectData;
    methods: typeof injectMethods;
  }
}

// 必须在任何 DefineComponent 之前调用（app.ts 首行 import 本文件）。
instanceConfig.setInjectInfo({
  data: injectData,
  methods: injectMethods,
  options: {
    addGlobalClass: true,
    multipleSlots: true,
  },
});
