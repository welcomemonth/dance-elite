import { DefineComponent, RootComponent } from "annil";

/** 公众号二维码图片。 */
const QR_PATH = "/assets/images/gzh.jpg";

const rootComponent = RootComponent()({
  isPage: true,
  data: {
    QR_PATH: QR_PATH,
  },
});

const follow = DefineComponent({
  path: "/pages/follow/follow",
  rootComponent,
});

export type $Follow = typeof follow;
