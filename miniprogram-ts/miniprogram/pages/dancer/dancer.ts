import { DefineComponent, RootComponent } from "annil";

import { getDancerDetail } from "../../api/ranking";
import type { DancerDetail, DancerEventItem } from "../../types/models";
import { danceTypeText } from "../../utils/event";
import { formatDate } from "../../utils/format";

const AWARD_LABELS: Record<string, string> = {
  champion: "冠军",
  runner_up: "亚军",
  third: "季军",
  elite_dancer: "菁英舞者",
  special_gold: "特金奖",
  gold: "金奖",
  silver: "银奖",
};

interface EventView extends DancerEventItem {
  awardLabel: string;
  dateText: string;
}

const rootComponent = RootComponent()({
  isPage: true,
  properties: {
    entryId: { type: Number, value: 0 },
  },
  data: {
    loading: true,
    loadError: null as string | null,
    detail: null as DancerDetail | null,
  },
  computed: {
    boardText(): string {
      const board = this.data.detail?.board;
      return board ? `${board.level} · ${danceTypeText(board.dance_type as "classical" | "folk")}` : "";
    },
    qualificationText(): string {
      const q = this.data.detail?.qualification;
      if (!q) {
        return "";
      }
      return q.type === "direct" ? `总决赛直通（${q.source}）` : `积分晋级（${q.source}）`;
    },
    countedEvents(): EventView[] {
      return viewEvents(this.data.detail?.counted_events ?? []);
    },
    historyEvents(): EventView[] {
      return viewEvents(this.data.detail?.history ?? []);
    },
    honorText(): string {
      const honors = (this.data.detail?.honors ?? []).map((h) => AWARD_LABELS[h] ?? h);
      return Array.from(new Set(honors)).join("、");
    },
  },
  pageLifetimes: {
    onLoad() {
      void this.loadDetail();
    },
  },
  methods: {
    async loadDetail(): Promise<void> {
      if (!this.data.entryId || this.data.entryId <= 0) {
        this.setData({ loading: false, loadError: "选手参数不正确" });
        return;
      }
      this.setData({ loading: true, loadError: null });
      try {
        const detail = await getDancerDetail(this.data.entryId);
        this.setData({ detail });
      } catch (error) {
        this.setData({
          loadError: error instanceof Error ? error.message : "加载选手详情失败",
        });
      } finally {
        this.setData({ loading: false });
      }
    },
    onRetry() {
      void this.loadDetail();
    },
  },
});

function viewEvents(list: DancerEventItem[]): EventView[] {
  return list.map((item) => ({
    ...item,
    awardLabel: AWARD_LABELS[item.award] ?? item.award,
    dateText: item.event_date ? formatDate(item.event_date, "MM.DD") : "",
  }));
}

const dancer = DefineComponent({
  path: "/pages/dancer/dancer",
  rootComponent,
});

export type $Dancer = typeof dancer;
