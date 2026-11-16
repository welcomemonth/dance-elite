import { DefineComponent, navigateTo, RootComponent } from "annil";

import { listAllRegistrations } from "../../api/registrations";
import { listAllWaitlists } from "../../api/waitlists";
import { userStore } from "../../store/user";
import type { RegistrationListItem, WaitlistItem } from "../../types/models";
import { danceTypeText } from "../../utils/event";
import { formatPhone } from "../../utils/format";
import { syncTabBar } from "../../utils/tab-bar";
import type { $Follow } from "../follow/follow";
import type { $Materials } from "../materials/materials";
import type { $Registrations } from "../registrations/registrations";
import type { $Rules } from "../rules/rules";
import type { $Waitlists } from "../waitlists/waitlists";

const DEFAULT_NAME = "菁英舞者";
const DEFAULT_AVATAR = "/assets/avatar/avatar.png";

const rootComponent = RootComponent()({
  isPage: true,
  data: {
    loading: false,
    loadError: null as string | null,
    registrations: [] as RegistrationListItem[],
    waitlists: [] as WaitlistItem[],
  },
  store: {
    user: () => userStore.profile,
    loggedIn: () => userStore.isLoggedIn,
  },
  computed: {
    displayName(): string {
      return this.data.user?.nickname?.trim() || DEFAULT_NAME;
    },
    avatarUrl(): string {
      return this.data.user?.avatar_url?.trim() || DEFAULT_AVATAR;
    },
    phoneText(): string {
      const phone = this.data.user?.phone;
      return phone ? formatPhone(phone) : "未绑定手机号";
    },
    registrationCount(): number {
      return this.data.registrations.length;
    },
    waitlistCount(): number {
      return this.data.waitlists.filter((item) => item.status === "waiting").length;
    },
    incompleteCount(): number {
      return this.data.registrations.filter(
        (item) => item.status === "paid" && item.material_status.missing.length > 0,
      ).length;
    },
    incompleteHint(): string {
      const first = this.data.registrations.find(
        (item) => item.status === "paid" && item.material_status.missing.length > 0,
      );
      return first
        ? `${first.event_name} · ${first.level_group} ${danceTypeText(first.dance_type)}`
        : "";
    },
  },
  pageLifetimes: {
    onShow() {
      syncTabBar(this, 2);
      void this.loadData();
    },
  },
  methods: {
    async loadData(): Promise<void> {
      if (this.data.loading) {
        return;
      }
      this.setData({ loading: true, loadError: null });
      try {
        await userStore.ensureLogin();
        const [registrations, waitlists] = await Promise.all([
          listAllRegistrations(),
          listAllWaitlists(),
        ]);
        this.setData({ registrations, waitlists });
      } catch (error) {
        this.setData({
          loadError: error instanceof Error ? error.message : "加载失败，请稍后重试",
        });
      } finally {
        this.setData({ loading: false });
      }
    },
    onLoginTap() {
      void userStore.ensureLogin().catch((error: unknown) => {
        this.showError(error);
      });
    },
    async onGetPhoneNumber(event: WechatMiniprogram.CustomEvent): Promise<void> {
      const detail = event.detail as { code?: string };
      if (!detail.code) {
        this.showToast("未获取到手机号授权，请重试");
        return;
      }
      try {
        await userStore.ensureLogin();
        await userStore.bindPhone(detail.code);
        this.showToast("手机号已绑定", "success");
      } catch (error) {
        this.showError(error);
      }
    },
    onReminderTap() {
      const items = this.data.registrations.filter(
        (item) => item.status === "paid" && item.material_status.missing.length > 0,
      );
      if (items.length === 1) {
        void navigateTo<$Materials>({
          url: "/pages/materials/materials",
          data: { registrationId: items[0].registration_id },
        });
        return;
      }
      void navigateTo<$Registrations>({ url: "/pages/registrations/registrations" });
    },
    onMenuTap(event: WechatMiniprogram.TouchEvent) {
      const target = String(event.currentTarget.dataset.target ?? "");
      switch (target) {
        case "registrations":
          void navigateTo<$Registrations>({ url: "/pages/registrations/registrations" });
          break;
        case "waitlists":
          void navigateTo<$Waitlists>({ url: "/pages/waitlists/waitlists" });
          break;
        case "follow":
          void navigateTo<$Follow>({ url: "/pages/follow/follow" });
          break;
        case "rules":
          void navigateTo<$Rules>({ url: "/pages/rules/rules" });
          break;
        default:
          break;
      }
    },
  },
});

const mine = DefineComponent({
  path: "/pages/mine/mine",
  rootComponent,
});

export type $Mine = typeof mine;
