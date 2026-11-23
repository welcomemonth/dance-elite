import { DefineComponent, redirectTo, RootComponent } from "annil";

import { submitWaitlist } from "../../api/waitlists";
import { userStore } from "../../store/user";
import { ERROR_CODE } from "../../types/api";
import type { DanceType, GroupLevel } from "../../types/models";
import { isApiError } from "../../utils/api-error";
import { danceTypeText } from "../../utils/event";
import { maskIdCard } from "../../utils/identity";
import type { $WaitlistSuccess } from "../waitlist-success/waitlist-success";

const PHONE_PATTERN = /^1\d{10}$/;
const ID_CARD_PATTERN = /^\d{17}[\dXx]$/;

const rootComponent = RootComponent()({
  isPage: true,
  properties: {
    eventId: { type: Number, value: 0 },
    eventName: { type: String, value: "" },
    groupId: { type: Number, value: 0 },
    level: { type: String, value: "" },
    danceType: { type: String, value: "" },
    playerName: { type: String, value: "" },
    idCardNo: { type: String, value: "" },
    institutionText: { type: String, value: "" },
  },
  data: {
    name: "",
    idCard: "",
    institution: "",
    phone: "",
    submitting: false,
    invalid: false,
  },
  computed: {
    groupText(): string {
      const level = this.data.level as GroupLevel;
      const dance = this.data.danceType as DanceType;
      return level && dance ? `${level} · ${danceTypeText(dance)}` : "";
    },
    maskedIdCard(): string {
      return this.data.idCard ? maskIdCard(this.data.idCard) : "";
    },
    idReadonly(): boolean {
      return !!this.data.idCard;
    },
  },
  pageLifetimes: {
    onLoad() {
      if (!this.data.eventId || this.data.groupId <= 0 || !this.data.danceType) {
        this.setData({ invalid: true });
        return;
      }
      this.setData({
        name: this.data.playerName,
        idCard: this.data.idCardNo,
        institution: this.data.institutionText,
      });
    },
  },
  methods: {
    onBackTap() {
      wx.navigateBack({
        delta: 1,
        fail: () => {
          void wx.switchTab({ url: "/pages/events/events" });
        },
      });
    },
    onNameInput(event: WechatMiniprogram.Input) {
      this.setData({ name: event.detail.value });
    },
    onIdCardInput(event: WechatMiniprogram.Input) {
      this.setData({ idCard: event.detail.value.toUpperCase() });
    },
    onInstitutionInput(event: WechatMiniprogram.Input) {
      this.setData({ institution: event.detail.value });
    },
    onPhoneInput(event: WechatMiniprogram.Input) {
      this.setData({ phone: event.detail.value.replace(/\D/g, "").slice(0, 11) });
    },
    async onSubmit(): Promise<void> {
      if (this.data.submitting) {
        return;
      }
      const name = this.data.name.trim();
      const idCard = this.data.idCard.trim().toUpperCase();
      const phone = this.data.phone.trim();
      if (!name) {
        this.showToast("请填写选手姓名");
        return;
      }
      if (idCard && !ID_CARD_PATTERN.test(idCard)) {
        this.showToast("身份证号不正确，请检查后重试");
        return;
      }
      if (!PHONE_PATTERN.test(phone)) {
        this.showToast("请输入正确的 11 位手机号");
        return;
      }

      this.setData({ submitting: true });
      try {
        await userStore.ensureLogin();
        const result = await submitWaitlist(this.data.eventId, {
          group_id: this.data.groupId,
          name,
          id_card_no: idCard || undefined,
          institution: this.data.institution.trim() || undefined,
          phone,
        });
        const level = this.data.level as GroupLevel;
        const dance = this.data.danceType as DanceType;
        void redirectTo<$WaitlistSuccess>({
          url: "/pages/waitlist-success/waitlist-success",
          data: {
            eventName: this.data.eventName,
            groupText: level && dance ? `${level} · ${danceTypeText(dance)}` : "",
            playerName: name,
            positionNo: result.position_no,
          },
        });
      } catch (error) {
        this.handleError(error);
      } finally {
        this.setData({ submitting: false });
      }
    },
    handleError(error: unknown): void {
      if (!isApiError(error)) {
        this.showToast(error instanceof Error ? error.message : "候补提交失败，请重试");
        return;
      }
      switch (error.code) {
        case ERROR_CODE.WAITLIST_DUPLICATE:
        case ERROR_CODE.ID_CARD_INVALID:
        case ERROR_CODE.AGE_NOT_MATCH:
          wx.showModal({
            title: "无法提交候补",
            content: error.message,
            showCancel: false,
            confirmText: "我知道了",
          });
          break;
        case ERROR_CODE.INVALID_PARAM:
          wx.showModal({
            title: "名额已释放",
            content: "该组别当前已有空余名额，请返回赛事详情直接报名。",
            showCancel: false,
            confirmText: "我知道了",
          });
          break;
        default:
          wx.showModal({
            title: "候补提交失败",
            content: error.message,
            showCancel: false,
            confirmText: "我知道了",
          });
      }
    },
    showToast(title: string) {
      wx.showToast({ title, icon: "none" });
    },
  },
});

const waitlist = DefineComponent({
  path: "/pages/waitlist/waitlist",
  rootComponent,
});

export type $Waitlist = typeof waitlist;
