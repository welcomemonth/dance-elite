import { DefineComponent, navigateTo, RootComponent } from "annil";

import {
  getInstitutionRanking,
  getRanking,
  getRankingConfig,
} from "../../api/ranking";
import type {
  InstitutionRankEntry,
  RankingConfig,
  RankingDirectEntry,
  RankingEntry,
  RankingOption,
} from "../../types/models";
import { formatEventFee } from "../../utils/event";
import { formatDateTime } from "../../utils/format";
import { syncTabBar } from "../../utils/tab-bar";
import type { $Dancer } from "../dancer/dancer";
import type { $Rules } from "../rules/rules";

interface DirectView extends RankingDirectEntry {
  awardText: string;
}

interface RankView extends RankingEntry {
  isNew: boolean;
  changeText: string;
  changeTone: "up" | "down" | "same";
  showAdvanceLine: boolean;
}

interface InstitutionView extends InstitutionRankEntry {
  honorText: string;
}

const AWARD_LABELS: Record<string, string> = {
  champion: "冠军",
  runner_up: "亚军",
  third: "季军",
  elite_dancer: "菁英舞者",
  special_gold: "特金奖",
  gold: "金奖",
  silver: "银奖",
};

function awardText(item: RankingDirectEntry["awards"]): string {
  return item
    .map((a) => {
      const label = AWARD_LABELS[a.award] ?? a.award;
      return a.prize_amount > 0
        ? `${a.event_name} · ${label} ${formatEventFee(a.prize_amount)}`
        : `${a.event_name} · ${label}`;
    })
    .join("；");
}

const rootComponent = RootComponent()({
  isPage: true,
  data: {
    loading: true,
    loadError: null as string | null,
    config: null as RankingConfig | null,
    activeLevel: "",
    activeDance: "",
    institutionMode: false,
    keyword: "",
    updatedText: "",
    direct: [] as DirectView[],
    ranking: [] as RankView[],
    institutions: [] as InstitutionView[],
  },
  computed: {
    levels(): RankingOption[] {
      return this.data.config?.levels ?? [];
    },
    danceStyles(): RankingOption[] {
      return this.data.config?.dance_styles ?? [];
    },
    seasonText(): string {
      return this.data.config?.season ?? "";
    },
    titleText(): string {
      return this.data.config?.title ?? "优舞盟 · 年度榜单";
    },
    subtitleText(): string {
      return this.data.config?.subtitle ?? "直通榜 · 积分榜 · 机构年度榜";
    },
    filteredDirect(): DirectView[] {
      return filterByName(this.data.direct, this.data.keyword);
    },
    filteredRanking(): RankView[] {
      return filterByName(this.data.ranking, this.data.keyword);
    },
    filteredInstitutions(): InstitutionView[] {
      return filterByName(this.data.institutions, this.data.keyword);
    },
    isEmpty(): boolean {
      if (this.data.loading) {
        return false;
      }
      if (this.data.institutionMode) {
        return filterByName(this.data.institutions, this.data.keyword).length === 0;
      }
      return (
        filterByName(this.data.ranking, this.data.keyword).length === 0 &&
        filterByName(this.data.direct, this.data.keyword).length === 0
      );
    },
  },
  pageLifetimes: {
    onShow() {
      syncTabBar(this, 1);
      if (!this.data.config) {
        void this.loadConfig();
      }
    },
    onPullDownRefresh() {
      void this.reload().finally(() => wx.stopPullDownRefresh());
    },
  },
  methods: {
    async loadConfig(): Promise<void> {
      this.setData({ loading: true, loadError: null });
      try {
        const config = await getRankingConfig();
        this.setData({
          config,
          activeLevel: this.data.activeLevel || config.levels[0]?.value || "",
          activeDance: this.data.activeDance || config.dance_styles[0]?.value || "",
        });
        await this.loadBoard();
      } catch (error) {
        this.setData({
          loading: false,
          loadError: error instanceof Error ? error.message : "榜单加载失败",
        });
      }
    },
    async loadBoard(): Promise<void> {
      const { activeLevel, activeDance, institutionMode } = this.data;
      this.setData({ loading: true, loadError: null });
      try {
        if (institutionMode) {
          const data = await getInstitutionRanking();
          this.setData({
            institutions: data.institutions.map((item) => ({
              ...item,
              honorText:
                item.honor === "elite_3"
                  ? "年度菁英机构奖"
                  : item.honor === "top_10"
                    ? "年度十佳机构奖"
                    : "",
            })),
            updatedText: data.updated_at ? formatDateTime(data.updated_at) : "",
          });
        } else {
          const board = await getRanking(activeLevel, activeDance);
          const quota = this.data.config?.advance_quota ?? 20;
          this.setData({
            direct: board.direct.map((item) => ({ ...item, awardText: awardText(item.awards) })),
            ranking: board.ranking.map((item) => rankView(item, quota)),
            updatedText: board.updated_at ? formatDateTime(board.updated_at) : "",
          });
        }
      } catch (error) {
        this.setData({
          loadError: error instanceof Error ? error.message : "榜单加载失败",
        });
      } finally {
        this.setData({ loading: false });
      }
    },
    async reload(): Promise<void> {
      if (!this.data.config) {
        await this.loadConfig();
        return;
      }
      await this.loadBoard();
    },
    onLevelTap(event: WechatMiniprogram.TouchEvent) {
      const value = String(event.currentTarget.dataset.value ?? "");
      if (!value || value === this.data.activeLevel) {
        return;
      }
      this.setData({ activeLevel: value, institutionMode: false });
      void this.loadBoard();
    },
    onDanceTap(event: WechatMiniprogram.TouchEvent) {
      const value = String(event.currentTarget.dataset.value ?? "");
      if (!value || value === this.data.activeDance) {
        return;
      }
      this.setData({ activeDance: value });
      void this.loadBoard();
    },
    onInstitutionTap() {
      if (this.data.institutionMode) {
        return;
      }
      this.setData({ institutionMode: true });
      void this.loadBoard();
    },
    onKeywordInput(event: WechatMiniprogram.Input) {
      this.setData({ keyword: event.detail.value });
    },
    onDancerTap(event: WechatMiniprogram.TouchEvent) {
      const entryId = Number(event.currentTarget.dataset.id ?? 0);
      if (entryId <= 0) {
        return;
      }
      void navigateTo<$Dancer>({
        url: "/pages/dancer/dancer",
        data: { entryId },
      });
    },
    onRulesTap() {
      void navigateTo<$Rules>({ url: "/pages/rules/rules" });
    },
    onRetry() {
      void this.reload();
    },
  },
});

/** 按姓名本地过滤（接口整榜返回，前端不额外请求）。 */
function filterByName<T extends { name: string }>(list: T[], keyword: string): T[] {
  const key = keyword.trim();
  if (!key) {
    return list;
  }
  return list.filter((item) => item.name.includes(key));
}

function rankView(item: RankingEntry, quota: number): RankView {
  const isNew = item.prev_rank <= 0;
  const change = item.rank_change;
  return {
    ...item,
    isNew,
    changeTone: change > 0 ? "up" : change < 0 ? "down" : "same",
    changeText: isNew ? "新上榜" : change > 0 ? `↑${change}` : change < 0 ? `↓${-change}` : "—",
    showAdvanceLine: item.rank === quota,
  };
}

const ranking = DefineComponent({
  path: "/pages/ranking/ranking",
  rootComponent,
});

export type $Ranking = typeof ranking;
