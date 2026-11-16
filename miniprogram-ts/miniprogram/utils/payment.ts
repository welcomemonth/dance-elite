import { ENV_VERSION } from "../config/env";
import { getOrderStatus, mockNotifyPayment, prepayOrder } from "../api/orders";
import type { SubmitRegistrationResult, WxPayParams } from "../types/models";

export type PaymentOutcome = "paid" | "canceled";

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

/** 拉起微信支付收银台；用户取消与支付失败分开返回。 */
function requestWechatPayment(params: WxPayParams): Promise<PaymentOutcome> {
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
        const message = error.errMsg || "";
        if (message.includes("cancel")) {
          resolve("canceled");
          return;
        }
        reject(new Error(`支付失败：${message || "请稍后重试"}`));
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
