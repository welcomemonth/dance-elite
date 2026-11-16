import { DefineComponent, RootComponent } from "annil";

import { syncTabBar } from "../../utils/tab-bar";

const rootComponent = RootComponent()({
  isPage: true,
  pageLifetimes: {
    onShow() {
      syncTabBar(this, 1);
    },
  },
});

const ranking = DefineComponent({
  path: "/pages/ranking/ranking",
  rootComponent,
});

export type $Ranking = typeof ranking;
