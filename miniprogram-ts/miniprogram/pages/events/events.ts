import { CustomComponent, DefineComponent, navigateTo, RootComponent } from "annil";

import { listEvents } from "../../api/events";
import type { $EmptyState } from "../../components/empty-state/empty-state";
import { userStore } from "../../store/user";
import type { EventItem, EventStatus } from "../../types/models";
import {
  EVENT_STATUS_OPTIONS,
  eventCardView,
  type EventCardView,
} from "../../utils/event";
import { PagedListController, type PagedListState } from "../../utils/paged-list";
import { syncTabBar } from "../../utils/tab-bar";
import type { $EventDetail } from "../event-detail/event-detail";

const list = new PagedListController<EventItem, { status?: EventStatus }>({
  fetcher: (params) => listEvents(params),
  pageSize: 10,
  query: {},
});

let unsubscribe: (() => void) | null = null;

function pickState(state: PagedListState<EventItem>) {
  return {
    list: state.list,
    total: state.total,
    loading: state.loading,
    refreshing: state.refreshing,
    finished: state.finished,
    listError: state.error,
  };
}

const rootComponent = RootComponent()({
  isPage: true,
  data: {
    list: [] as EventItem[],
    total: 0,
    loading: false,
    refreshing: false,
    finished: false,
    listError: null as string | null,
    activeStatus: "" as "" | EventStatus,
    filters: EVENT_STATUS_OPTIONS,
  },
  computed: {
    rows(): EventCardView[] {
      const allFilter = this.data.activeStatus === "";
      return this.data.list.map((item, index) => eventCardView(item, index, allFilter));
    },
    isEmpty(): boolean {
      return !this.data.loading && !this.data.refreshing && this.data.list.length === 0;
    },
  },
  pageLifetimes: {
    onLoad() {
      unsubscribe = list.subscribe((state) => {
        this.setData(pickState(state));
      });
    },
    onShow() {
      syncTabBar(this, 0);
      // 落地页兜底静默登录后再拉列表，确保“已报名”角标与报名状态一致；
      // 登录失败也继续匿名加载，不阻塞赛事浏览。
      void userStore
        .ensureLogin()
        .catch(() => undefined)
        .then(() => this.refreshList());
    },
    onUnload() {
      unsubscribe?.();
      unsubscribe = null;
    },
    onPullDownRefresh() {
      void this.refreshList(true);
    },
    onReachBottom() {
      void this.loadMore();
    },
  },
  methods: {
    refreshList(stopPullDown = false): Promise<void> {
      return list
        .refresh()
        .catch((error: unknown) => {
          this.showError(error);
        })
        .finally(() => {
          if (stopPullDown) {
            wx.stopPullDownRefresh();
          }
        });
    },
    async loadMore(): Promise<void> {
      try {
        await list.loadMore();
      } catch (error) {
        this.showError(error);
      }
    },
    onFilterTap(event: WechatMiniprogram.TouchEvent) {
      const value = String(event.currentTarget.dataset.value ?? "") as "" | EventStatus;
      if (value === this.data.activeStatus) {
        return;
      }
      this.setData({ activeStatus: value });
      void list.setQuery({ status: value || undefined }).catch((error: unknown) => {
        this.showError(error);
      });
    },
    onRefreshTap() {
      void this.refreshList();
    },
    onEventTap(event: WechatMiniprogram.TouchEvent) {
      const eventId = Number(event.currentTarget.dataset.id ?? 0);
      if (eventId <= 0) {
        return;
      }
      void navigateTo<$EventDetail>({
        url: "/pages/event-detail/event-detail",
        data: { eventId },
      });
    },
  },
});

type Root = typeof rootComponent;

const emptyState = CustomComponent<Root, $EmptyState>()({
  events: {
    emptyState_retry() {
      void this.refreshList();
    },
  },
});

const events = DefineComponent({
  path: "/pages/events/events",
  rootComponent,
  subComponents: [emptyState],
});

export type $Events = typeof events;
