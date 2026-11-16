import { API_BASE_URL } from "../config/env";
import type {
  CancelOrderResult,
  OrderStatusResult,
  PrepayResult,
} from "../types/models";
import { nowISO } from "../utils/format";
import { http } from "../utils/request";

/** 支付预下单（POST /api/v1/orders/{order_id}/prepay，API 文档 3.1b）。 */
export function prepayOrder(orderId: number): Promise<PrepayResult> {
  return http.post<PrepayResult>(`/api/v1/orders/${orderId}/prepay`);
}

/** 用户主动取消待支付订单（POST /api/v1/orders/{order_id}/cancel，API 文档 3.5）。 */
export function cancelOrder(
  orderId: number,
  reason: "user_cancel" | "pay_failed" = "user_cancel",
): Promise<CancelOrderResult> {
  return http.post<CancelOrderResult>(`/api/v1/orders/${orderId}/cancel`, { reason });
}

/** 查询订单状态（GET /api/v1/orders/{order_id}，API 文档 3.1c）。 */
export function getOrderStatus(orderId: number): Promise<OrderStatusResult> {
  return http.get<OrderStatusResult>(`/api/v1/orders/${orderId}`);
}

/**
 * 本地联调专用：PAY_MOCK=true 时直接向支付回调接口提交明文成功报文。
 * 仅 develop 环境调用；生产环境不会走到这里。
 */
export function mockNotifyPayment(orderNo: string, amount: number): Promise<void> {
  return new Promise((resolve, reject) => {
    wx.request({
      url: `${API_BASE_URL}/api/v1/payments/wechat/notify`,
      method: "POST",
      timeout: 15000,
      header: { "Content-Type": "application/json" },
      data: {
        out_trade_no: orderNo,
        transaction_id: `MOCK${Date.now()}${Math.floor(Math.random() * 100000)}`,
        trade_state: "SUCCESS",
        success_time: nowISO(),
        amount: { total: amount, currency: "CNY" },
      },
      success(response) {
        if (response.statusCode === 200) {
          resolve();
          return;
        }
        reject(new Error(`mock 支付回调失败（HTTP ${response.statusCode}）`));
      },
      fail(error) {
        reject(new Error(error.errMsg || "mock 支付回调失败"));
      },
    });
  });
}
