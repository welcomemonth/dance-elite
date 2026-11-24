import { DefineComponent, RootComponent } from "annil";

import { getContent } from "../../api/contents";

const RULE_SECTIONS = [
  {
    title: "积分如何计算",
    text: "积分按名次和参赛人数动态计算；同等奖项下，超级赛积分高于甲级赛。",
  },
  {
    title: "年度排名",
    text: "排名依据为本年度积分最高的 3 场积分赛，不足 3 场按实际计。每场积分赛结束后实时更新。",
  },
  {
    title: "总决赛晋级",
    text: "甲级赛第一名、超级赛前两名直通年度总决赛；其余选手按各榜单全年积分前 20 获得资格。",
  },
  {
    title: "总决赛荣誉",
    text: "年度总决赛设置 MVP、全国总冠军、全国八强、十六强与四十强荣誉，并按赛事规则分配个人及机构奖金。",
  },
  {
    title: "退赛与退款",
    text: "抽签前可无偿退赛；抽签后个人原因退赛扣除 50%；伤病凭医院诊断证明可顺延不扣费。退款请联系客服登记。",
  },
];

const rootComponent = RootComponent()({
  isPage: true,
  data: {
    sections: RULE_SECTIONS,
  },
  pageLifetimes: {
    onLoad() {
      void this.loadContents();
    },
  },
  methods: {
    /** 读取后台内容管理里的积分规则/赛事 FAQ（S5-4）；失败或为空时保留本地文案。 */
    async loadContents(): Promise<void> {
      try {
        const [points, faq] = await Promise.all([
          getContent("points_rules_desc"),
          getContent("event_faq"),
        ]);
        const sections: { title: string; text: string }[] = [];
        if (points.value.text.trim()) {
          sections.push({ title: "积分规则说明", text: points.value.text.trim() });
        }
        if (faq.value.text.trim()) {
          sections.push({ title: "赛事规则 / FAQ", text: faq.value.text.trim() });
        }
        if (sections.length > 0) {
          this.setData({ sections });
        }
      } catch {
        // 内容接口不可用时使用本地兜底文案，不阻塞页面展示。
      }
    },
  },
});

const rules = DefineComponent({
  path: "/pages/rules/rules",
  rootComponent,
});

export type $Rules = typeof rules;
