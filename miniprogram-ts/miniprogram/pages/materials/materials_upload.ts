import { uploadMaterialAudio, uploadMaterialPhoto } from "../../api/registrations";
import type { UpdateMaterialsResult } from "../../types/models";
import { chooseAudioFile, chooseImageFile } from "../../utils/media";

const MAX_PHOTO_BYTES = 10 * 1024 * 1024;
const MAX_AUDIO_BYTES = 100 * 1024 * 1024;

export type UploadOutcome =
  | { status: "canceled" }
  | { status: "invalid"; message: string }
  | { status: "uploaded"; result: UpdateMaterialsResult };

/** 选择证件照并上传；本地先校验大小与扩展名，网络/服务端错误向上抛出。 */
export async function uploadPhotoFromPicker(registrationId: number): Promise<UploadOutcome> {
  const file = await chooseImageFile();
  if (!file) {
    return { status: "canceled" };
  }
  if (file.size > MAX_PHOTO_BYTES) {
    return { status: "invalid", message: "证件照不能超过 10MB" };
  }
  const ext = file.path.split(".").pop()?.toLowerCase() ?? "";
  if (ext && ext !== "jpg" && ext !== "jpeg" && ext !== "png") {
    return { status: "invalid", message: "证件照仅支持 png/jpg 格式" };
  }

  const result = await uploadMaterialPhoto(registrationId, file.path);
  return { status: "uploaded", result };
}

/** 从聊天文件选择 mp3 并上传；时长由服务端解析校验。 */
export async function uploadAudioFromPicker(registrationId: number): Promise<UploadOutcome> {
  const file = await chooseAudioFile();
  if (!file) {
    return { status: "canceled" };
  }
  if (!file.name.toLowerCase().endsWith(".mp3")) {
    return { status: "invalid", message: "音频仅支持 mp3 格式" };
  }
  if (file.size > MAX_AUDIO_BYTES) {
    return { status: "invalid", message: "音频不能超过 100MB" };
  }

  const result = await uploadMaterialAudio(registrationId, file.path);
  return { status: "uploaded", result };
}
