import { DefineComponent, navigateTo, RootComponent } from "annil";

import type { $Materials } from "../materials/materials";
import type { $RegistrationDetail } from "../registration-detail/registration-detail";

const rootComponent = RootComponent()({
  isPage: true,
  properties: {
    orderNo: { type: String, value: "" },
    registrationId: { type: Number, value: 0 },
    eventName: { type: String, value: "" },
    levelText: { type: String, value: "" },
    danceTypeText: { type: String, value: "" },
    amountText: { type: String, value: "" },
    deadlineText: { type: String, value: "" },
  },
  methods: {
    onMaterials() {
      if (this.data.registrationId <= 0) {
        this.showToast("报名信息同步中，请稍后从「我的报名」进入");
        return;
      }
      void navigateTo<$Materials>({
        url: "/pages/materials/materials",
        data: { registrationId: this.data.registrationId },
      });
    },
    onGoMine() {
      if (this.data.registrationId > 0) {
        void navigateTo<$RegistrationDetail>({
          url: "/pages/registration-detail/registration-detail",
          data: { registrationId: this.data.registrationId },
        });
        return;
      }
      void wx.switchTab({ url: "/pages/mine/mine" });
    },
    onGoEvents() {
      void wx.switchTab({ url: "/pages/events/events" });
    },
    showToast(title: string) {
      wx.showToast({ title, icon: "none" });
    },
  },
});

const registerSuccess = DefineComponent({
  path: "/pages/register-success/register-success",
  rootComponent,
});

export type $RegisterSuccess = typeof registerSuccess;
