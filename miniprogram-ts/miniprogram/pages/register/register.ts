import { DefineComponent, redirectTo, RootComponent } from "annil";

import { getEventDetail } from "../../api/events";
import { cancelOrder } from "../../api/orders";
import { submitRegistration } from "../../api/registrations";
import { userStore } from "../../store/user";
import { ERROR_CODE } from "../../types/api";
import type { DanceType, EventDetail } from "../../types/models";
import { isApiError } from "../../utils/api-error";
import {
  danceTypeText,
  eventLevelText,
  formatEventDate,
  formatEventDeadline,
  formatEventFee,
  groupLevelText,
} from "../../utils/event";
import { createIdempotencyKey } from "../../utils/id";
import { payRegistration } from "../../utils/payment";
import type { $RegisterSuccess } from "../register-success/register-success";

interface DanceOptionView {
  value: DanceType;
  label: string;
  hint: string;
}

function danceOptions(event: EventDetail): DanceOptionView[] {
  return (["classical", "folk"] as DanceType[]).map((value) => {
    const remaining = event.groups
      .filter((group) => group.enabled && group.dance_type === value)
      .reduce((sum, group) => sum + Math.max(0, group.remaining), 0);
    return {
      value,
      label: danceTypeText(value),
      hint: remaining > 0 ? `全组别剩余 ${remaining} 席` : "名额已满，可联系客服",
    };
  });
}

const ID_CARD_PATTERN = /^\d{17}[\dXx]$/;

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
    danceOptions(): DanceOptionView[] {
      return this.data.event ? danceOptions(this.data.event) : [];
    },
    submitText(): string {
      if (this.data.submitting) {
        return "提交中…";
      }
      const fee = this.data.event?.fee ?? 0;
      return fee > 0 ? `提交并支付 ${formatEventFee(fee)}` : "提交报名";
    },
  },
  pageLifetimes: {
    onLoad(props) {
      if (!props.eventId || props.eventId <= 0) {
        this.setData({ loading: false, loadError: "赛事参数不正确" });
        return;
      }
      void this.loadEvent();
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
    onRetry() {
      void this.loadEvent();
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
      if (value === "classical" || value === "folk") {
        this.setData({ danceType: value });
      }
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
      if (!ID_CARD_PATTERN.test(idCardNo)) {
        this.showToast("请输入 18 位大陆身份证号");
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

        const outcome = await payRegistration(result);
        if (outcome === "canceled") {
          await cancelOrder(result.order_id, "user_cancel").catch(() => undefined);
          this.setData({ submitKey: "" });
          this.showToast("支付未完成，报名未成功");
          return;
        }

        this.setData({ submitKey: "" });
        void redirectTo<$RegisterSuccess>({
          url: "/pages/register-success/register-success",
          data: {
            orderNo: result.order_no,
            eventName: event.name,
            levelText: groupLevelText(result.level),
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
          this.setData({ submitKey: "" });
        }
        this.handleSubmitError(error);
      } finally {
        this.setData({ submitting: false });
      }
    },
    handleSubmitError(error: unknown): void {
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
        case ERROR_CODE.DUPLICATE_DANCE:
        case ERROR_CODE.EVENT_NOT_REGISTERING:
        case ERROR_CODE.GROUP_CLOSED:
          wx.showModal({
            title: "报名未完成",
            content: error.message,
            showCancel: false,
            confirmText: "我知道了",
          });
          break;
        case ERROR_CODE.QUOTA_FULL:
          wx.showModal({
            title: "名额已满",
            content: "该组别名额已满，请联系客服登记候补。",
            showCancel: false,
            confirmText: "我知道了",
          });
          break;
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
        default:
          this.showError(error);
      }
    },
  },
});

const register = DefineComponent({
  path: "/pages/register/register",
  rootComponent,
});

export type $Register = typeof register;
