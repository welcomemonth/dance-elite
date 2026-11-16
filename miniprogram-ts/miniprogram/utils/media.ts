export interface ChooseImageOptions {
  count?: number;
  sizeType?: Array<"original" | "compressed">;
  sourceType?: Array<"album" | "camera">;
}

/** 选择图片，返回本地临时路径；用户取消时返回空数组。 */
export function chooseImages(options: ChooseImageOptions = {}): Promise<string[]> {
  return new Promise((resolve, reject) => {
    wx.chooseMedia({
      count: options.count ?? 1,
      mediaType: ["image"],
      sizeType: options.sizeType ?? ["compressed"],
      sourceType: options.sourceType ?? ["album", "camera"],
      success(result) {
        resolve(result.tempFiles.map((file) => file.tempFilePath));
      },
      fail(error) {
        if (error.errMsg.includes("cancel")) {
          resolve([]);
          return;
        }
        reject(new Error(`选择图片失败：${error.errMsg}`));
      },
    });
  });
}

export interface ChosenImage {
  path: string;
  size: number;
  fileType: string;
}

/** 选择单张图片并返回临时路径与大小（用于上传前校验）。 */
export function chooseImageFile(options: ChooseImageOptions = {}): Promise<ChosenImage | null> {
  return new Promise((resolve, reject) => {
    wx.chooseMedia({
      count: 1,
      mediaType: ["image"],
      sizeType: options.sizeType ?? ["compressed"],
      sourceType: options.sourceType ?? ["album", "camera"],
      success(result) {
        const file = result.tempFiles[0];
        if (!file) {
          resolve(null);
          return;
        }
        resolve({
          path: file.tempFilePath,
          size: file.size,
          fileType: file.fileType ?? "image",
        });
      },
      fail(error) {
        if (error.errMsg.includes("cancel")) {
          resolve(null);
          return;
        }
        reject(new Error(`选择图片失败：${error.errMsg}`));
      },
    });
  });
}

export interface ChosenAudio {
  path: string;
  name: string;
  size: number;
}

/** 从微信聊天记录中选择一个 mp3 音频文件（小程序唯一可用的文件选择入口）。 */
export function chooseAudioFile(): Promise<ChosenAudio | null> {
  return new Promise((resolve, reject) => {
    wx.chooseMessageFile({
      count: 1,
      type: "file",
      extension: ["mp3"],
      success(result) {
        const file = result.tempFiles[0];
        if (!file) {
          resolve(null);
          return;
        }
        resolve({ path: file.path, name: file.name, size: file.size });
      },
      fail(error) {
        if (error.errMsg.includes("cancel")) {
          resolve(null);
          return;
        }
        reject(new Error(`选择音频失败：${error.errMsg}`));
      },
    });
  });
}

/** 预览图片（支持远程 URL 与本地临时路径）。 */
export function previewImages(urls: string[], current?: string): void {
  if (urls.length === 0) {
    return;
  }
  wx.previewImage({ urls, current: current ?? urls[0] });
}

/** 保存图片到相册；未授权时抛出可提示的错误。 */
export function saveImageToPhotosAlbum(filePath: string): Promise<void> {
  return new Promise((resolve, reject) => {
    wx.saveImageToPhotosAlbum({
      filePath,
      success() {
        resolve();
      },
      fail(error) {
        const message = error.errMsg.includes("auth")
          ? "未授权保存到相册，请在设置中开启"
          : `保存失败：${error.errMsg}`;
        reject(new Error(message));
      },
    });
  });
}
