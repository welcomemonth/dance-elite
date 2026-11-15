import { CustomComponent, DefineComponent, RootComponent } from "annil";

import type { $EmptyState } from "../../components/empty-state/empty-state";
import type { PagedList } from "../../types/api";
import { formatDate, formatMoney } from "../../utils/format";
import { PagedListController, type PagedListState, type PageRequest } from "../../utils/paged-list";

interface DemoEvent {
  id: number;
  name: string;
  city: string;
  event_date: string;
  /** 单位：分 */
  fee: number;
}

const CITIES = ["北京", "上海", "广州", "成都"];

// 模板示例数据；接入真实接口时把 fetchDemoEvents 换成 http.get + PagedList<T> 即可。
const DEMO_EVENTS: DemoEvent[] = Array.from({ length: 26 }, (_, index) => {
  const no = 26 - index;
  const month = String((index % 9) + 1).padStart(2, "0");
  const day = String((index % 18) + 10).padStart(2, "0");
  return {
    id: 1000 + no,
    name: `示例赛事 ${no}`,
    city: CITIES[index % CITIES.length],
    event_date: `2027-${month}-${day}T10:00:00+08:00`,
    fee: no % 3 === 0 ? 0 : (980 + index * 20) * 100,
  };
});

function fetchDemoEvents(params: PageRequest): Promise<PagedList<DemoEvent>> {
  const { page, page_size } = params;
  return new Promise((resolve) => {
    setTimeout(() => {
      const start = (page - 1) * page_size;
      resolve({
        list: DEMO_EVENTS.slice(start, start + page_size),
        total: DEMO_EVENTS.length,
        page,
        page_size,
      });
    }, 350);
  });
}

const list = new PagedListController<DemoEvent, Record<string, never>>({
  fetcher: fetchDemoEvents,
  pageSize: 10,
});

let unsubscribe: (() => void) | null = null;

function pickState(state: PagedListState<DemoEvent>) {
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
    list: [] as DemoEvent[],
    total: 0,
    loading: false,
    refreshing: false,
    finished: false,
    listError: null as string | null,
  },
  computed: {
    rows(): Array<DemoEvent & { feeText: string; dateText: string }> {
      return this.data.list.map((item) => ({
        ...item,
        feeText: item.fee === 0 ? "免费" : formatMoney(item.fee),
        dateText: formatDate(item.event_date),
      }));
    },
    summary(): string {
      return `共 ${this.data.total} 条，已加载 ${this.data.list.length} 条`;
    },
  },
  pageLifetimes: {
    onLoad() {
      unsubscribe = list.subscribe((state) => {
        this.setData(pickState(state));
      });
      void this.refreshList();
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

const listDemo = DefineComponent({
  path: "/pages/list-demo/list-demo",
  rootComponent,
  subComponents: [emptyState],
});

export type $ListDemo = typeof listDemo;
