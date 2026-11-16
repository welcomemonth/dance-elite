import { CustomComponent, DefineComponent, RootComponent } from "annil";

import { cancelWaitlist, listAllWaitlists } from "../../api/waitlists";
import type { $EmptyState } from "../../components/empty-state/empty-state";
import type { WaitlistItem } from "../../types/models";
import { danceTypeText } from "../../utils/event";
import { formatDate } from "../../utils/format";
import { waitlistStatusView, type TagTone } from "../../utils/registration";

interface WaitlistView extends WaitlistItem {
  statusLabel: string;
  statusTone: TagTone;
  danceText: string;
  createdText: string;
  canCancel: boolean;
}

function waitlistView(item: WaitlistItem): WaitlistView {
  const status = waitlistStatusView(item.status);
  return {
    ...item,
    statusLabel: status.label,
    statusTone: status.tone,
    danceText: danceTypeText(item.dance_type),
    createdText: formatDate(item.created_at, "YYYY.MM.DD HH:mm"),
    canCancel: item.status === "waiting",
  };
}

const rootComponent = RootComponent()({
  isPage: true,
  data: {
    loading: true,
    loadError: null as string | null,
    list: [] as WaitlistItem[],
  },
  computed: {
    rows(): WaitlistView[] {
      return this.data.list.map((item) => waitlistView(item));
    },
    isEmpty(): boolean {
      return !this.data.loading && this.data.list.length === 0;
    },
  },
  pageLifetimes: {
    onShow() {
      void this.loadData();
    },
    onPullDownRefresh() {
      void this.loadData(true);
    },
  },
  methods: {
    async loadData(stopPullDown = false): Promise<void> {
      this.setData({ loading: true, loadError: null });
      try {
        const list = await listAllWaitlists();
        this.setData({ list });
      } catch (error) {
        this.setData({ loadError: error instanceof Error ? error.message : "加载候补失败" });
      } finally {
        this.setData({ loading: false });
        if (stopPullDown) {
          wx.stopPullDownRefresh();
        }
      }
    },
    onCancelTap(event: WechatMiniprogram.TouchEvent) {
      const waitlistId = Number(event.currentTarget.dataset.id ?? 0);
      if (waitlistId <= 0) {
        return;
      }
      wx.showModal({
        title: "取消候补",
        content: "取消后将移出候补队列，不涉及退款。确定取消吗？",
        confirmText: "取消候补",
        confirmColor: "#5423e7",
        success: (result) => {
          if (result.confirm) {
            void this.cancel(waitlistId);
          }
        },
      });
    },
    async cancel(waitlistId: number): Promise<void> {
      try {
        await cancelWaitlist(waitlistId);
        this.showToast("已取消候补", "success");
        await this.loadData();
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
      void this.loadData();
    },
  },
});

const waitlists = DefineComponent({
  path: "/pages/waitlists/waitlists",
  rootComponent,
  subComponents: [emptyState],
});

export type $Waitlists = typeof waitlists;
