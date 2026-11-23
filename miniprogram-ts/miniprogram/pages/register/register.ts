import { DefineComponent, navigateTo, redirectTo, RootComponent } from "annil";

import { getEventDetail } from "../../api/events";
import { cancelOrder } from "../../api/orders";
import {
  getRegistrationDetail,
  listAllRegistrations,
  submitRegistration,
} from "../../api/registrations";
import { userStore } from "../../store/user";
import { ERROR_CODE } from "../../types/api";
import type {
  DanceType,
  EventDetail,
  RegistrationListItem,
} from "../../types/models";
import { isApiError } from "../../utils/api-error";
import {
  danceTypeText,
  eventLevelText,
  formatEventDate,
  formatEventDeadline,
  formatEventFee,
} from "../../utils/event";
import { groupForEvent, type IdentityGroup } from "../../utils/identity";
import { createIdempotencyKey } from "../../utils/id";
import { payRegistration, PaymentError } from "../../utils/payment";
import { clearPendingOrder, savePendingOrder } from "../../utils/pending-order";
import type { $RegisterSuccess } from "../register-success/register-success";
import type { $Waitlist } from "../waitlist/waitlist";

interface DanceOptionView {
  value: DanceType;
  label: string;
  hint: string;
  disabled: boolean;
}

interface HistoryOption {
  key: string;
  label: string;
  playerName: string;
  registrationId: number;
}

const ID_CARD_PATTERN = /^\d{17}[\dXx]$/;

function groupTextOf(info: IdentityGroup | null, idCard: string): string {
  if (!idCard.trim()) {
    return "填写身份证号后显示组别";
  }
  if (!info) {
    return "未匹配到年龄组，请核对身份证号出生年份";
  }
  return `所属组别：${info.level}（${info.birthYear} 年出生，${info.birthFrom}–${info.birthTo} 年区间）`;
}

function danceOptionsOf(
  event: EventDetail,
  info: IdentityGroup | null,
  registered: DanceType[],
): DanceOptionView[] {
  return (["classical", "folk"] as DanceType[]).map((value) => {
    const already = registered.includes(value);
    const group = info
      ? event.groups.find((item) => item.level === info.level && item.dance_type === value)
      : undefined;
    let hint = "填写身份证号后显示名额";
    if (already) {
      hint = "已报名，请勿重复提交";
    } else if (info) {
      if (!group) {
        hint = "该组别未开放";
      } else if (!group.enabled) {
        hint = "该组别已关闭";
      } else if (group.remaining <= 0) {
        hint = "名额已满，可提交候补";
      } else {
        hint = `剩余 ${Math.max(0, group.remaining)} 席`;
      }
    }
    return {
      value,
      label: danceTypeText(value),
      hint,
      disabled: already,
    };
  });
}

function historyOptionsOf(list: RegistrationListItem[]): HistoryOption[] {
  const seen = new Set<string>();
  const options: HistoryOption[] = [];
  for (const item of list) {
    const key = item.player_name;
    if (seen.has(key) || !item.player_name) {
      continue;
    }
    seen.add(key);
    options.push({
      key,
      label: `${item.player_name} · ${item.event_name}`,
      playerName: item.player_name,
      registrationId: item.registration_id,
    });
    if (options.length >= 6) {
      break;
    }
  }
  return options;
}

const rootComponent = RootComponent()({
  isPage: true,
  properties: {
    eventId: { type: Number, value: 0 },
  },
  data: {
    event: null as EventDetail | null,
    loading: true,
    loadError: null as string | null,
    playerName: "",
    idCardNo: "",
    institution: "",
    danceType: "classical" as DanceType,
    submitting: false,
    submitKey: "",
    history: [] as HistoryOption[],
    registeredDances: [] as DanceType[],
    noIdModal: false,
  },
  computed: {
    eventTitle(): string {
      return this.data.event?.name ?? "";
    },
    levelText(): string {
      return this.data.event ? eventLevelText(this.data.event.level) : "";
    },
    dateText(): string {
      return this.data.event ? formatEventDate(this.data.event.event_date) : "";
    },
    feeText(): string {
      return this.data.event ? formatEventFee(this.data.event.fee) : "";
    },
    groupInfo(): IdentityGroup | null {
      return this.data.event && this.data.idCardNo
        ? groupForEvent(this.data.event, this.data.idCardNo)
        : null;
    },
    groupText(): string {
      return groupTextOf(this.data.groupInfo, this.data.idCardNo);
    },
    danceOptions(): DanceOptionView[] {
      return this.data.event
        ? danceOptionsOf(this.data.event, this.data.groupInfo, this.data.registeredDances)
        : [];
    },
    selectedGroupId(): number {
      const event = this.data.event;
      const info = this.data.groupInfo;
      if (!event || !info) {
        return 0;
      }
      return (
        event.groups.find(
          (item) => item.level === info.level && item.dance_type === this.data.danceType,
        )?.group_id ?? 0
      );
    },
    quotaFull(): boolean {
      const event = this.data.event;
      const info = this.data.groupInfo;
      if (!event || !info) {
        return false;
      }
      const group = event.groups.find(
        (item) => item.level === info.level && item.dance_type === this.data.danceType,
      );
      return !!group && group.enabled && group.remaining <= 0;
    },
    submitText(): string {
      if (this.data.submitting) {
        return "提交中…";
      }
      if (this.data.quotaFull) {
        return "名额已满，提交候补";
      }
      const fee = this.data.event?.fee ?? 0;
      return fee > 0 ? `提交并支付 ${formatEventFee(fee)}` : "提交报名";
    },
    hasHistory(): boolean {
      return this.data.history.length > 0;
    },
  },
  pageLifetimes: {
    onLoad(props) {
      if (!props.eventId || props.eventId <= 0) {
        this.setData({ loading: false, loadError: "赛事参数不正确" });
        return;
      }
      void this.loadEvent();
      void this.loadContext();
    },
  },
  methods: {
    async loadEvent(): Promise<void> {
      this.setData({ loading: true, loadError: null });
      try {
        const event = await getEventDetail(this.data.eventId);
        this.setData({ event });
      } catch (error) {
        this.setData({ loadError: error instanceof Error ? error.message : "加载赛事失败" });
      } finally {
        this.setData({ loading: false });
      }
    },
    async loadContext(): Promise<void> {
      try {
        await userStore.ensureLogin();
        const list = await listAllRegistrations();
        const registered = list
          .filter(
            (item) =>
              item.event_id === this.data.eventId &&
              (item.status === "paid" || item.status === "refunding"),
          )
          .map((item) => item.dance_type);
        const registeredDances = Array.from(new Set(registered));
        let danceType = this.data.danceType;
        if (registeredDances.includes(danceType)) {
          const other: DanceType = danceType === "classical" ? "folk" : "classical";
          if (!registeredDances.includes(other)) {
            danceType = other;
          }
        }
        this.setData({
          history: historyOptionsOf(list),
          registeredDances,
          danceType,
        });
      } catch {
        // 登录或历史数据加载失败不阻塞报名，提交时会再次登录。
      }
    },
    onRetry() {
      void this.loadEvent();
      void this.loadContext();
    },
    onPlayerInput(event: WechatMiniprogram.Input) {
      this.setData({ playerName: event.detail.value });
    },
    onIdCardInput(event: WechatMiniprogram.Input) {
      this.setData({ idCardNo: event.detail.value.toUpperCase() });
    },
    onInstitutionInput(event: WechatMiniprogram.Input) {
      this.setData({ institution: event.detail.value });
    },
    onDanceTap(event: WechatMiniprogram.TouchEvent) {
      const value = String(event.currentTarget.dataset.value ?? "") as DanceType;
      if (value !== "classical" && value !== "folk") {
        return;
      }
      if (this.data.registeredDances.includes(value)) {
        this.showToast("该舞种已报名，请勿重复提交");
        return;
      }
      this.setData({ danceType: value });
    },
    onHistoryTap() {
      if (this.data.history.length === 0) {
        this.showToast("暂无历史报名");
        return;
      }
      wx.showActionSheet({
        itemList: this.data.history.map((item) => item.label),
        success: (result) => {
          const option = this.data.history[result.tapIndex];
          if (!option) {
            return;
          }
          this.setData({ playerName: option.playerName });
          void getRegistrationDetail(option.registrationId)
            .then((detail) => {
              if (detail.institution) {
                this.setData({ institution: detail.institution });
              }
            })
            .catch(() => undefined);
          this.showToast("已带出姓名与机构，请重新填写证件号");
        },
      });
    },
    onNoIdClose() {
      this.setData({ noIdModal: false });
    },
    async onSubmit(): Promise<void> {
      if (this.data.submitting) {
        return;
      }
      const event = this.data.event;
      if (!event) {
        return;
      }
      const playerName = this.data.playerName.trim();
      const idCardNo = this.data.idCardNo.trim().toUpperCase();
      const institution = this.data.institution.trim();
      if (!playerName) {
        this.showToast("请填写选手姓名");
        return;
      }
      if (!idCardNo) {
        this.setData({ noIdModal: true });
        return;
      }
      if (!ID_CARD_PATTERN.test(idCardNo)) {
        this.showToast("身份证号不正确，请检查后重试");
        return;
      }
      const info = groupForEvent(event, idCardNo);
      if (!info) {
        wx.showModal({
          title: "年龄不符合要求",
          content: "该选手不符合本次赛事年龄要求（U11/U13/U15），无法报名。",
          showCancel: false,
          confirmText: "我知道了",
          success: () => this.goBackToEvents(),
        });
        return;
      }
      if (this.data.registeredDances.includes(this.data.danceType)) {
        wx.showModal({
          title: "该舞种已报名",
          content: "该选手已报名本赛事该舞种，请勿重复提交；可为同组别选择另一舞种。",
          showCancel: false,
          confirmText: "返回修改",
        });
        return;
      }
      const group = event.groups.find(
        (item) => item.level === info.level && item.dance_type === this.data.danceType,
      );
      if (group && group.enabled && group.remaining <= 0) {
        void navigateTo<$Waitlist>({
          url: "/pages/waitlist/waitlist",
          data: {
            eventId: event.id,
            eventName: event.name,
            groupId: group.group_id,
            level: info.level,
            danceType: this.data.danceType,
            playerName,
            idCardNo,
            institutionText: institution,
          },
        });
        return;
      }

      const submitKey = this.data.submitKey || createIdempotencyKey();
      let orderId = 0;
      this.setData({ submitting: true, submitKey });
      try {
        await userStore.ensureLogin();
        const result = await submitRegistration(event.id, {
          player_name: playerName,
          id_card_no: idCardNo,
          institution: institution || undefined,
          dance_type: this.data.danceType,
          idempotency_key: submitKey,
        });
        orderId = result.order_id;
        savePendingOrder(orderId);

        const outcome = await payRegistration(result);
        if (outcome === "canceled") {
          await cancelOrder(result.order_id, "user_cancel").catch(() => undefined);
          clearPendingOrder(result.order_id);
          this.setData({ submitKey: "" });
          this.showToast("支付未完成，报名未成功");
          this.goBackToEvent();
          return;
        }

        clearPendingOrder(result.order_id);
        this.setData({ submitKey: "" });
        void redirectTo<$RegisterSuccess>({
          url: "/pages/register-success/register-success",
          data: {
            orderNo: result.order_no,
            registrationId: result.registration_id,
            eventName: event.name,
            levelText: result.level,
            danceTypeText: danceTypeText(result.dance_type),
            amountText: formatEventFee(result.amount),
            deadlineText: result.material_deadline
              ? formatEventDeadline(result.material_deadline)
              : "以赛事通知为准",
          },
        });
      } catch (error) {
        if (orderId > 0) {
          await cancelOrder(orderId, "pay_failed").catch(() => undefined);
          clearPendingOrder(orderId);
          this.setData({ submitKey: "" });
        }
        this.handleSubmitError(error);
      } finally {
        this.setData({ submitting: false });
      }
    },
    handleSubmitError(error: unknown): void {
      // 支付拉起失败：弹出微信返回的原始错误，便于用户反馈与联调定位。
      if (error instanceof PaymentError) {
        wx.showModal({
          title: "无法发起微信支付",
          content: `${error.errMsg || "未知错误"}\n\n请在真机重试；若仍失败，请截图联系客服。`,
          showCancel: false,
          confirmText: "我知道了",
        });
        return;
      }
      if (!isApiError(error)) {
        this.showError(error);
        return;
      }
      switch (error.code) {
        case ERROR_CODE.ID_CARD_INVALID:
          this.showToast(error.message);
          break;
        case ERROR_CODE.AGE_NOT_MATCH:
        case ERROR_CODE.DUPLICATE_GROUP:
          wx.showModal({
            title: "报名未完成",
            content: error.message,
            showCancel: false,
            confirmText: "我知道了",
            success: () => this.goBackToEvents(),
          });
          break;
        case ERROR_CODE.DUPLICATE_DANCE:
          wx.showModal({
            title: "该舞种已报名",
            content: error.message,
            showCancel: false,
            confirmText: "返回修改",
          });
          break;
        case ERROR_CODE.EVENT_NOT_REGISTERING:
        case ERROR_CODE.GROUP_CLOSED:
          wx.showModal({
            title: "报名未完成",
            content: error.message,
            showCancel: false,
            confirmText: "我知道了",
            success: () => this.goBackToEvent(),
          });
          break;
        case ERROR_CODE.QUOTA_FULL: {
          const event = this.data.event;
          const info = event && this.data.idCardNo
            ? groupForEvent(event, this.data.idCardNo)
            : null;
          const group = event && info
            ? event.groups.find(
                (item) => item.level === info.level && item.dance_type === this.data.danceType,
              )
            : undefined;
          if (event && info && group) {
            void navigateTo<$Waitlist>({
              url: "/pages/waitlist/waitlist",
              data: {
                eventId: event.id,
                eventName: event.name,
                groupId: group.group_id,
                level: info.level,
                danceType: this.data.danceType,
                playerName: this.data.playerName.trim(),
                idCardNo: this.data.idCardNo.trim().toUpperCase(),
                institutionText: this.data.institution.trim(),
              },
            });
          } else {
            this.showError(error);
          }
          break;
        }
        case ERROR_CODE.CONFLICT:
          this.setData({ submitKey: "" });
          wx.showModal({
            title: "报名状态已变化",
            content: error.message,
            showCancel: false,
            confirmText: "重新尝试",
          });
          break;
        case ERROR_CODE.UNAUTHORIZED:
          wx.showModal({
            title: "登录已过期",
            content: "请返回页面重新进入后再试",
            showCancel: false,
          });
          break;
        case ERROR_CODE.SERVER_ERROR:
          // 预下单/支付链路失败（如微信统一下单被拒）：用弹窗展示，避免 toast 一闪而过。
          wx.showModal({
            title: "支付未完成",
            content: `${error.message}\n\n如反复出现，请截图联系客服并提供操作时间。`,
            showCancel: false,
            confirmText: "我知道了",
          });
          break;
        default:
          this.showError(error);
      }
    },
    goBackToEvent() {
      wx.navigateBack({
        delta: 1,
        fail: () => {
          void wx.switchTab({ url: "/pages/events/events" });
        },
      });
    },
    goBackToEvents() {
      void wx.switchTab({ url: "/pages/events/events" });
    },
  },
});

const register = DefineComponent({
  path: "/pages/register/register",
  rootComponent,
});

export type $Register = typeof register;
