import { DefineComponent, RootComponent } from "annil";

const rootComponent = RootComponent()({
  isPage: true,
  properties: {
    orderNo: { type: String, value: "" },
    eventName: { type: String, value: "" },
    levelText: { type: String, value: "" },
    danceTypeText: { type: String, value: "" },
    amountText: { type: String, value: "" },
    deadlineText: { type: String, value: "" },
  },
  methods: {
    onGoMine() {
      wx.switchTab({ url: "/pages/mine/mine" });
    },
    onGoEvents() {
      wx.switchTab({ url: "/pages/events/events" });
    },
  },
});

const registerSuccess = DefineComponent({
  path: "/pages/register-success/register-success",
  rootComponent,
});

export type $RegisterSuccess = typeof registerSuccess;
