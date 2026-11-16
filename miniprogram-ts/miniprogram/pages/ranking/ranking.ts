import { DefineComponent, navigateTo, RootComponent } from "annil";

import { syncTabBar } from "../../utils/tab-bar";
import type { $Rules } from "../rules/rules";

const rootComponent = RootComponent()({
  isPage: true,
  pageLifetimes: {
    onShow() {
      syncTabBar(this, 1);
    },
  },
  methods: {
    onRulesTap() {
      void navigateTo<$Rules>({ url: "/pages/rules/rules" });
    },
  },
});

const ranking = DefineComponent({
  path: "/pages/ranking/ranking",
  rootComponent,
});

export type $Ranking = typeof ranking;
