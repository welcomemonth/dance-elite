import { DefineComponent, navigateTo, RootComponent } from "annil";

import type { $Waitlists } from "../waitlists/waitlists";

const rootComponent = RootComponent()({
  isPage: true,
  properties: {
    eventName: { type: String, value: "" },
    groupText: { type: String, value: "" },
    playerName: { type: String, value: "" },
    positionNo: { type: Number, value: 0 },
  },
  methods: {
    onGoWaitlists() {
      void navigateTo<$Waitlists>({ url: "/pages/waitlists/waitlists" });
    },
    onGoEvents() {
      void wx.switchTab({ url: "/pages/events/events" });
    },
  },
});

const waitlistSuccess = DefineComponent({
  path: "/pages/waitlist-success/waitlist-success",
  rootComponent,
});

export type $WaitlistSuccess = typeof waitlistSuccess;
