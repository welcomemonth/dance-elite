import { ENV_VERSION } from "../config/env";
import { getOrderStatus, mockNotifyPayment, prepayOrder } from "../api/orders";
import type { SubmitRegistrationResult, WxPayParams } from "../types/models";

export type PaymentOutcome = "paid" | "canceled";

/**
 * 支付拉起失败（非用户取消）：携带微信返回的原始 errMsg，
 * 供页面展示与开发联调排查（如 access denied / 支付验证签名失败）。
 */
export class PaymentError extends Error {
  readonly errMsg: string;

  constructor(errMsg: string) {
    super(errMsg ? `无法发起支付：${errMsg}` : "无法发起支付，请稍后重试");
    this.errMsg = errMsg;
  }
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

/** 仅把微信的“用户取消/关闭收银台”视为取消，其余失败一律按错误抛出。 */
function isUserCancel(errMsg: string): boolean {
  return errMsg.includes("cancel") || errMsg.includes("取消");
}

/** 拉起微信支付收银台；用户取消与支付失败分开返回。 */
function requestWechatPayment(params: WxPayParams): Promise<PaymentOutcome> {
  // prepay_id 等信息不含密钥，打印出来便于真机/开发者工具联调定位。
  console.log("[pay] 拉起微信支付", {
    timeStamp: params.timeStamp,
    package: params.package,
    signType: params.signType,
    hasPaySign: Boolean(params.paySign),
  });
  return new Promise((resolve, reject) => {
    wx.requestPayment({
      timeStamp: params.timeStamp,
      nonceStr: params.nonceStr,
      package: params.package,
      signType: "RSA",
      paySign: params.paySign,
      success() {
        resolve("paid");
      },
      fail(error) {
        const errMsg = error.errMsg || "";
        console.error("[pay] requestPayment 失败", errMsg || error);
        if (isUserCancel(errMsg)) {
          resolve("canceled");
          return;
        }
        reject(new PaymentError(errMsg));
      },
    });
  });
}

/** 轮询订单状态，等待支付回调落库（mock 通道与回调延迟场景共用）。 */
async function waitForPaid(orderId: number, attempts = 8): Promise<PaymentOutcome> {
  for (let i = 0; i < attempts; i += 1) {
    const order = await getOrderStatus(orderId);
    if (order.status === "paid") {
      return "paid";
    }
    if (order.status === "canceled") {
      return "canceled";
    }
    await delay(300);
  }
  throw new Error("支付结果确认中，请稍后在「我的报名」查看");
}

/**
 * 报名支付：
 * - 生产/体验环境拉起微信支付；
 * - 本地 PAY_MOCK=true 时直接提交 mock 回调报文并轮询订单状态。
 */
export async function payRegistration(result: SubmitRegistrationResult): Promise<PaymentOutcome> {
  const prepay = await prepayOrder(result.order_id);
  const isMock =
    prepay.pay_params.paySign === "MOCK-SIGN" ||
    prepay.pay_params.package.includes("mock-prepay-");
  if (ENV_VERSION === "develop" && isMock) {
    await mockNotifyPayment(prepay.order_no, result.amount);
    return waitForPaid(result.order_id);
  }
  return requestWechatPayment(prepay.pay_params);
}
