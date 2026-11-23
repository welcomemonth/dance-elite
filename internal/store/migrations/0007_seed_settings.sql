-- 0007: 系统默认配置种子（键值均为 JSON 文本，后台「内容管理」可改）。
--
-- 说明：
--   group_rules          —— 系统级组别年龄规则，创建赛季/赛事组别时复制（2027 赛季口径）；
--   refund_policy        —— 退费政策文案（值本身是 JSON 字符串）；
--   score_rules_template —— 积分规则默认模板（名次基础分 × 人数系数 × 级别权重），赛事可覆盖；
--   prize_rules_template —— 奖金规则默认模板（金额单位：分），赛事可覆盖。
--   模板中的数值为可调整的初始默认值，发布赛事前须在后台按当期政策确认。

INSERT INTO settings ("key", value) VALUES (
    'group_rules',
    '{
        "age_cutoff": "2027-01-01",
        "levels": [
            {"level": "U11", "birth_year_from": 2016, "birth_year_to": 2018},
            {"level": "U13", "birth_year_from": 2014, "birth_year_to": 2015},
            {"level": "U15", "birth_year_from": 2012, "birth_year_to": 2013}
        ]
    }'
) ON CONFLICT ("key") DO NOTHING;

INSERT INTO settings ("key", value) VALUES (
    'refund_policy',
    '"抽签前可无偿退赛，报名费全额退回；抽签结束后不予更改：个人原因临时退赛扣除 50% 报名费，因伤病无法参赛凭医院诊断证明可顺延赛事、不扣费。报名截止后不再受理退款申请。"'
) ON CONFLICT ("key") DO NOTHING;

INSERT INTO settings ("key", value) VALUES (
    'score_rules_template',
    '{
        "base_points": {
            "champion": 100,
            "runner_up": 85,
            "third": 75,
            "elite_dancer": 65,
            "special_gold": 55,
            "gold": 45,
            "silver": 30
        },
        "size_bands": [
            {"min_players": 40, "factor": 1.2},
            {"min_players": 20, "factor": 1.0},
            {"min_players": 0, "factor": 0.8}
        ],
        "level_weights": {"grade_a": 1.0, "super": 1.2, "final": 1.5}
    }'
) ON CONFLICT ("key") DO NOTHING;

INSERT INTO settings ("key", value) VALUES (
    'prize_rules_template',
    '{
        "grade_a": {"ranks": [1], "threshold": 40, "above": [200000], "below": [100000]},
        "super": {"ranks": [1, 2], "threshold": 40, "above": [300000, 150000], "below": [200000, 100000]},
        "final": {"amounts": [1000000, 300000, 200000, 100000, 100000, 100000, 100000, 100000]},
        "institution_top3": 1000000
    }'
) ON CONFLICT ("key") DO NOTHING;
