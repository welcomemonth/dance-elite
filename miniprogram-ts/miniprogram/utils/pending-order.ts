import { getOrderStatus } from "../api/orders";
import type { OrderStatusResult } from "../types/models";

const STORAGE_KEY = "dance_elite_pending_order";

interface PendingOrder {
  orderId: number;
  createdAt: number;
}

function read(): PendingOrder | null {
  try {
    const value = wx.getStorageSync(STORAGE_KEY) as PendingOrder | "";
    if (!value || typeof value.orderId !== "number" || value.orderId <= 0) {
      return null;
    }
    return value;
  } catch {
    return null;
  }
}

/** 记住最近一次提交报名产生的待支付订单，供重进「我的报名」时查单兜底。 */
export function savePendingOrder(orderId: number): void {
  if (orderId <= 0) {
    return;
  }
  try {
    wx.setStorageSync(STORAGE_KEY, { orderId, createdAt: Date.now() });
  } catch {
    // 存储失败不影响主流程。
  }
}

export function clearPendingOrder(orderId?: number): void {
  try {
    const current = read();
    if (orderId && current && current.orderId !== orderId) {
      return;
    }
    wx.removeStorageSync(STORAGE_KEY);
  } catch {
    // 忽略存储异常。
  }
}

/**
 * 重进「我的报名」时查询遗留待支付订单：
 * - paid/refunding/refunded/canceled：清掉本地记录；
 * - pending_payment：继续保留，等 15 分钟服务端回收；
 * - 40401：订单已不存在，清掉。
 * 返回查询结果（未登录/无记录时 null）。
 */
export async function reconcilePendingOrder(): Promise<OrderStatusResult | null> {
  const pending = read();
  if (!pending) {
    return null;
  }
  try {
    const status = await getOrderStatus(pending.orderId);
    if (status.status !== "pending_payment") {
      clearPendingOrder(pending.orderId);
    }
    return status;
  } catch (error) {
    const code = (error as { code?: number }).code;
    if (code === 40401) {
      clearPendingOrder(pending.orderId);
    }
    return null;
  }
}
