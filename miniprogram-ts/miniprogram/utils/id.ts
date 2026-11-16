/** 生成一次报名提交的幂等键；网络重试时复用同一值。 */
export function createIdempotencyKey(): string {
  const random = Math.random().toString(36).slice(2, 10);
  return `reg-${Date.now().toString(36)}-${random}`;
}
