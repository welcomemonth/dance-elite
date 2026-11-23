-- 0017: 订单与报名单锁定 1:1（S0 决策，2026-11-23）。
--
-- 代码注释与查询均按 1:1 JOIN（orders.registration_id = registrations.id），
-- 但此前只有普通索引；补唯一索引防止并发/重试在同一报名单上挂多笔订单。
-- 每个报名单在创建时即写入唯一订单，历史数据不存在一报名多单情况。
CREATE UNIQUE INDEX uq_orders_registration ON orders(registration_id);
