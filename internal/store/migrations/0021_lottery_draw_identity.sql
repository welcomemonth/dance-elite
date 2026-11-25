-- 0021: 抽签身份记录（公众号网页授权）。
--
-- H5 抽签改为微信内强制网页授权（scope=snsapi_base）后，手动抽签时把
-- 当前微信的公众号 openid/unionid 写在选手记录上，供后台追溯。
-- 系统自动分配（auto_drawn=1）没有操作人，两列为 NULL。
-- unionid 与 users.unionid 同源（公众号与小程序绑定同一开放平台），
-- 后台据此关联到报名时登录的小程序账号。
ALTER TABLE lottery_players ADD COLUMN drawn_openid TEXT;
ALTER TABLE lottery_players ADD COLUMN drawn_unionid TEXT;

CREATE INDEX idx_lottery_players_drawn_openid
    ON lottery_players(drawn_openid)
    WHERE drawn_openid IS NOT NULL;

CREATE INDEX idx_lottery_players_drawn_unionid
    ON lottery_players(drawn_unionid)
    WHERE drawn_unionid IS NOT NULL;
