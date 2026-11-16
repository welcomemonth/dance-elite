import { DefineComponent, RootComponent } from "annil";

import { getRegistrationDetail, updateMaterials } from "../../api/registrations";
import type { RegistrationDetail, UpdateMaterialsResult } from "../../types/models";
import { previewImages } from "../../utils/media";
import { absoluteUrl } from "../../utils/url";
import {
  EMPTY_FORM,
  formFromDetail,
  formPayload,
  validateForm,
  type FormField,
  type MaterialForm,
} from "./materials_form";
import { uploadAudioFromPicker, uploadPhotoFromPicker } from "./materials_upload";

let audioContext: WechatMiniprogram.InnerAudioContext | null = null;

const rootComponent = RootComponent()({
  isPage: true,
  properties: {
    registrationId: { type: Number, value: 0 },
  },
  data: {
    loading: true,
    loadError: null as string | null,
    detail: null as RegistrationDetail | null,
    ...EMPTY_FORM,
    submitting: false,
    uploadingPhoto: false,
    uploadingAudio: false,
    playing: false,
  },
  computed: {
    readonly(): boolean {
      const detail = this.data.detail;
      return !!detail && (detail.material_readonly || detail.status !== "paid");
    },
    photoUrl(): string {
      return absoluteUrl(this.data.detail?.materials.photo_url);
    },
    audioFilename(): string {
      return this.data.detail?.materials.audio_filename || "已上传的音频";
    },
    hasAudio(): boolean {
      return !!this.data.detail?.materials.audio_url;
    },
  },
  pageLifetimes: {
    onLoad() {
      void this.loadDetail();
    },
    onUnload() {
      if (audioContext) {
        audioContext.destroy();
        audioContext = null;
      }
    },
  },
  methods: {
    async loadDetail(): Promise<void> {
      if (!this.data.registrationId || this.data.registrationId <= 0) {
        this.setData({ loading: false, loadError: "报名参数不正确" });
        return;
      }
      this.setData({ loading: true, loadError: null });
      try {
        const detail = await getRegistrationDetail(this.data.registrationId);
        this.setData({ detail, ...formFromDetail(detail) });
      } catch (error) {
        this.setData({ loadError: error instanceof Error ? error.message : "加载报名资料失败" });
      } finally {
        this.setData({ loading: false });
      }
    },
    onRetry() {
      void this.loadDetail();
    },
    onFieldInput(event: WechatMiniprogram.Input) {
      const field = String(event.currentTarget.dataset.field ?? "") as FormField;
      const value = event.detail.value;
      switch (field) {
        case "programName":
          this.setData({ programName: value });
          break;
        case "teacherName":
          this.setData({ teacherName: value });
          break;
        case "contactPhone":
          this.setData({ contactPhone: value });
          break;
        case "expressAddress":
          this.setData({ expressAddress: value });
          break;
        case "workIntro":
          this.setData({ workIntro: value });
          break;
        case "specialReq":
          this.setData({ specialReq: value });
          break;
        default:
          break;
      }
    },
    onPreviewPhoto() {
      const url = absoluteUrl(this.data.detail?.materials.photo_url);
      if (url) {
        previewImages([url]);
      }
    },
    isReadonly(): boolean {
      const detail = this.data.detail;
      return !!detail && (detail.material_readonly || detail.status !== "paid");
    },
    form(): MaterialForm {
      return {
        programName: this.data.programName,
        teacherName: this.data.teacherName,
        contactPhone: this.data.contactPhone,
        expressAddress: this.data.expressAddress,
        workIntro: this.data.workIntro,
        specialReq: this.data.specialReq,
      };
    },
    async onChoosePhoto(): Promise<void> {
      if (this.isReadonly() || this.data.uploadingPhoto) {
        return;
      }
      this.setData({ uploadingPhoto: true });
      try {
        const outcome = await uploadPhotoFromPicker(this.data.registrationId);
        if (outcome.status === "invalid") {
          this.showToast(outcome.message);
        } else if (outcome.status === "uploaded") {
          this.applyResult(outcome.result);
          this.showToast("证件照已更新", "success");
        }
      } catch (error) {
        this.showError(error);
      } finally {
        this.setData({ uploadingPhoto: false });
      }
    },
    async onChooseAudio(): Promise<void> {
      if (this.isReadonly() || this.data.uploadingAudio) {
        return;
      }
      this.setData({ uploadingAudio: true });
      try {
        const outcome = await uploadAudioFromPicker(this.data.registrationId);
        if (outcome.status === "invalid") {
          this.showToast(outcome.message);
        } else if (outcome.status === "uploaded") {
          this.applyResult(outcome.result);
          this.stopAudio();
          this.showToast("舞蹈音频已更新", "success");
        }
      } catch (error) {
        this.showError(error);
      } finally {
        this.setData({ uploadingAudio: false });
      }
    },
    onToggleAudio() {
      const url = absoluteUrl(this.data.detail?.materials.audio_url);
      if (!url) {
        return;
      }
      if (this.data.playing) {
        this.stopAudio();
        return;
      }
      if (!audioContext) {
        audioContext = wx.createInnerAudioContext();
        audioContext.onEnded(() => this.setData({ playing: false }));
        audioContext.onError(() => {
          this.setData({ playing: false });
          this.showToast("音频播放失败");
        });
      }
      audioContext.src = url;
      audioContext.play();
      this.setData({ playing: true });
    },
    stopAudio() {
      if (audioContext) {
        audioContext.stop();
      }
      this.setData({ playing: false });
    },
    async onSave(): Promise<void> {
      if (this.isReadonly() || this.data.submitting) {
        return;
      }
      const form = this.form();
      const invalid = validateForm(form);
      if (invalid) {
        this.showToast(invalid);
        return;
      }

      this.setData({ submitting: true });
      try {
        const result = await updateMaterials(this.data.registrationId, formPayload(form));
        this.applyResult(result);
        this.showToast("资料已保存", "success");
        setTimeout(() => {
          wx.navigateBack();
        }, 400);
      } catch (error) {
        this.showError(error);
      } finally {
        this.setData({ submitting: false });
      }
    },
    applyResult(result: UpdateMaterialsResult): void {
      const detail = this.data.detail;
      if (!detail) {
        return;
      }
      this.setData({
        detail: {
          ...detail,
          materials: result.materials,
          material_status: result.material_status,
        },
      });
    },
  },
});

const materials = DefineComponent({
  path: "/pages/materials/materials",
  rootComponent,
});

export type $Materials = typeof materials;
