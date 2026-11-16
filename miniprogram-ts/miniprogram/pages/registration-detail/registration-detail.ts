import { CustomComponent, DefineComponent, navigateTo, RootComponent } from "annil";

import { getRegistrationDetail } from "../../api/registrations";
import type { $EmptyState } from "../../components/empty-state/empty-state";
import type {
  MaterialStatus,
  RegistrationDetail,
  RegistrationStatus,
} from "../../types/models";
import { danceTypeText, formatEventDate } from "../../utils/event";
import { formatDateTime, formatMoney } from "../../utils/format";
import {
  materialRows,
  registrationStatusView,
  type MaterialRowView,
  type TagTone,
} from "../../utils/registration";
import type { $Materials } from "../materials/materials";

function statusToneClass(status: RegistrationStatus, missingCount: number): string {
  if (status === "refunding") {
    return "status-card--purple";
  }
  if (status === "refunded") {
    return "status-card--muted";
  }
  return missingCount > 0 ? "status-card--yellow" : "status-card--success";
}

/**
 * 报名详情的资料完成度。
 * 正常由服务端 material_status 返回；旧版/未重启的后端可能缺少该字段，
 * 此时按资料快照兜底，避免页面因字段缺失直接崩溃。
 */
function materialStatusOf(detail: RegistrationDetail): MaterialStatus {
  const status = detail.material_status as MaterialStatus | undefined;
  if (status) {
    return status;
  }
  const missing = materialRows(detail.materials)
    .filter((row) => !row.uploaded)
    .map((row) => row.key);
  return {
    total: 8,
    completed: 8 - missing.length,
    missing,
  };
}

function missingCountOf(detail: RegistrationDetail | null): number {
  return detail ? materialStatusOf(detail).missing.length : 0;
}

const rootComponent = RootComponent()({
  isPage: true,
  properties: {
    registrationId: { type: Number, value: 0 },
  },
  data: {
    loading: true,
    loadError: null as string | null,
    detail: null as RegistrationDetail | null,
  },
  computed: {
    missingCount(): number {
      return missingCountOf(this.data.detail);
    },
    statusLabel(): string {
      const detail = this.data.detail;
      return detail
        ? registrationStatusView(detail.status, materialStatusOf(detail)).label
        : "";
    },
    statusTone(): TagTone {
      const detail = this.data.detail;
      return detail
        ? registrationStatusView(detail.status, materialStatusOf(detail)).tone
        : "default";
    },
    statusClass(): string {
      return this.data.detail
        ? statusToneClass(this.data.detail.status, missingCountOf(this.data.detail))
        : "";
    },
    dateText(): string {
      return this.data.detail ? formatEventDate(this.data.detail.event.event_date) : "";
    },
    danceText(): string {
      return this.data.detail ? danceTypeText(this.data.detail.dance_type) : "";
    },
    amountText(): string {
      return this.data.detail ? formatMoney(this.data.detail.amount) : "";
    },
    paidAtText(): string {
      const paidAt = this.data.detail?.paid_at;
      return paidAt ? formatDateTime(paidAt) : "—";
    },
    institutionText(): string {
      return this.data.detail?.institution || "—";
    },
    materialRows(): MaterialRowView[] {
      return this.data.detail ? materialRows(this.data.detail.materials) : [];
    },
    materialSummaryText(): string {
      const missing = missingCountOf(this.data.detail);
      return missing > 0 ? `待补充 ${missing} 项` : "资料已齐";
    },
    editable(): boolean {
      const detail = this.data.detail;
      return !!detail && detail.status === "paid" && !detail.material_readonly;
    },
  },
  pageLifetimes: {
    onShow() {
      void this.loadDetail();
    },
    onPullDownRefresh() {
      void this.loadDetail(true);
    },
  },
  methods: {
    async loadDetail(stopPullDown = false): Promise<void> {
      if (!this.data.registrationId || this.data.registrationId <= 0) {
        this.setData({ loading: false, loadError: "报名参数不正确" });
        return;
      }
      this.setData({ loading: true, loadError: null });
      try {
        const detail = await getRegistrationDetail(this.data.registrationId);
        this.setData({ detail });
      } catch (error) {
        this.setData({ loadError: error instanceof Error ? error.message : "加载报名详情失败" });
      } finally {
        this.setData({ loading: false });
        if (stopPullDown) {
          wx.stopPullDownRefresh();
        }
      }
    },
    onRetry() {
      void this.loadDetail();
    },
    onMaterialTap() {
      this.onUploadTap();
    },
    onUploadTap() {
      const detail = this.data.detail;
      if (!detail || detail.status !== "paid" || detail.material_readonly) {
        return;
      }
      void navigateTo<$Materials>({
        url: "/pages/materials/materials",
        data: { registrationId: this.data.registrationId },
      });
    },
  },
});

type Root = typeof rootComponent;

const emptyState = CustomComponent<Root, $EmptyState>()({
  events: {
    emptyState_retry() {
      void this.loadDetail();
    },
  },
});

const registrationDetail = DefineComponent({
  path: "/pages/registration-detail/registration-detail",
  rootComponent,
  subComponents: [emptyState],
});

export type $RegistrationDetail = typeof registrationDetail;
