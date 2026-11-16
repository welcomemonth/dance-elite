-- 0016: 待支付订单超时回收（开发计划 T34）。
-- expire_at 由创建订单时写入（now + ORDER_EXPIRE_MINUTES）；
-- 历史订单该列为 NULL，超时判定按 created_at + ORDER_EXPIRE_MINUTES 兜底。

ALTER TABLE orders ADD COLUMN expire_at TEXT;

CREATE INDEX idx_orders_status_expire ON orders(status, expire_at);
