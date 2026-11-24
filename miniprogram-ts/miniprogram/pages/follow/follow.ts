import { DefineComponent, RootComponent } from "annil";

import { getContent } from "../../api/contents";
import { absoluteUrl } from "../../utils/url";

/** 公众号二维码默认图片（后台未配置时使用）。 */
const QR_PATH = "/assets/images/gzh.jpg";

const rootComponent = RootComponent()({
  isPage: true,
  data: {
    QR_PATH: QR_PATH,
  },
  pageLifetimes: {
    onLoad() {
      void this.loadQrCode();
    },
  },
  methods: {
    /** 读取后台内容管理里的公众号二维码（S5-5）；未配置时保留本地图片。 */
    async loadQrCode(): Promise<void> {
      try {
        const entry = await getContent("official_account");
        const url = absoluteUrl(entry.value.image_url);
        if (url) {
          this.setData({ QR_PATH: url });
        }
      } catch {
        // 内容接口不可用时使用本地兜底图片。
      }
    },
  },
});

const follow = DefineComponent({
  path: "/pages/follow/follow",
  rootComponent,
});

export type $Follow = typeof follow;
