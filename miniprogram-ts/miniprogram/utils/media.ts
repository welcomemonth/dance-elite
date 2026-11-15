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
