import { CustomComponent, DefineComponent, navigateTo, RootComponent } from "annil";

import { listAllRegistrations } from "../../api/registrations";
import type { $EmptyState } from "../../components/empty-state/empty-state";
import type { RegistrationListItem } from "../../types/models";
import {
  REGISTRATION_FILTERS,
  registrationCardView,
  registrationMatchesFilter,
  type RegistrationCardView,
  type RegistrationFilter,
} from "../../utils/registration";
import type { $RegistrationDetail } from "../registration-detail/registration-detail";

const rootComponent = RootComponent()({
  isPage: true,
  data: {
    loading: true,
    loadError: null as string | null,
    filter: "all" as RegistrationFilter,
    filters: REGISTRATION_FILTERS,
    list: [] as RegistrationListItem[],
  },
  computed: {
    rows(): RegistrationCardView[] {
      return this.data.list
        .filter((item) => registrationMatchesFilter(item, this.data.filter))
        .map((item) => registrationCardView(item));
    },
    isEmpty(): boolean {
      return (
        !this.data.loading &&
        !this.data.list.some((item) => registrationMatchesFilter(item, this.data.filter))
      );
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
        const list = await listAllRegistrations();
        this.setData({ list });
      } catch (error) {
        this.setData({ loadError: error instanceof Error ? error.message : "加载失败" });
      } finally {
        this.setData({ loading: false });
        if (stopPullDown) {
          wx.stopPullDownRefresh();
        }
      }
    },
    onFilterTap(event: WechatMiniprogram.TouchEvent) {
      const value = String(event.currentTarget.dataset.value ?? "") as RegistrationFilter;
      if (value && value !== this.data.filter) {
        this.setData({ filter: value });
      }
    },
    onOpen(event: WechatMiniprogram.TouchEvent) {
      const registrationId = Number(event.currentTarget.dataset.id ?? 0);
      if (registrationId > 0) {
        void navigateTo<$RegistrationDetail>({
          url: "/pages/registration-detail/registration-detail",
          data: { registrationId },
        });
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

const registrations = DefineComponent({
  path: "/pages/registrations/registrations",
  rootComponent,
  subComponents: [emptyState],
});

export type $Registrations = typeof registrations;
