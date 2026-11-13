-- 0015: 内容管理键值种子（API 文档 13.4：积分规则说明/赛事规则FAQ/公众号二维码/公告）。
-- 后台「内容管理」页读写这 4 个 key；小程序端展示接口从同一份 settings 表读取。

INSERT INTO settings ("key", value) VALUES
    ('points_rules_desc', '{"text":""}'),
    ('event_faq', '{"text":""}'),
    ('official_account', '{"text":""}'),
    ('announcement', '{"text":""}')
ON CONFLICT ("key") DO NOTHING;
