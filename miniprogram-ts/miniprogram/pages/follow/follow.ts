import { DefineComponent, RootComponent } from "annil";

/** 二维码占位图案：正式公众号二维码配置后可替换为图片。 */
const QR_CELLS = Array.from({ length: 81 }, (_, index) => ({
  index,
  dark: (index * 7 + Math.floor(index / 9) * 3) % 5 < 2,
}));

const rootComponent = RootComponent()({
  isPage: true,
  data: {
    cells: QR_CELLS,
  },
});

const follow = DefineComponent({
  path: "/pages/follow/follow",
  rootComponent,
});

export type $Follow = typeof follow;
