-- 0018: 统一积分/奖金规则 JSON 契约（S4，2026-11-23）。
--
-- 0007 种子里原用 participant_coefficient / level_weight / 数组式奖金，
-- 与前端/后端 DTO 的 size_bands / level_weights / 对象式奖金不一致。
-- 这里只更新 settings 中的默认模板（仍为旧结构时才覆盖，不覆盖用户已改的值）。
UPDATE settings SET value = '{
  "base_points": {"champion": 100, "runner_up": 85, "third": 75, "elite_dancer": 65,
    "special_gold": 55, "gold": 45, "silver": 30},
  "size_bands": [{"min_players": 40, "factor": 1.2}, {"min_players": 20, "factor": 1.0},
    {"min_players": 0, "factor": 0.8}],
  "level_weights": {"grade_a": 1.0, "super": 1.2, "final": 1.5}
}'
WHERE "key" = 'score_rules_template' AND value LIKE '%participant_coefficient%';

UPDATE settings SET value = '{
  "grade_a": {"ranks": [1], "threshold": 40, "above": [200000], "below": [100000]},
  "super": {"ranks": [1, 2], "threshold": 40, "above": [300000, 150000], "below": [200000, 100000]},
  "final": {"amounts": [1000000, 300000, 200000, 100000, 100000, 100000, 100000, 100000]},
  "institution_top3": 1000000
}'
WHERE "key" = 'prize_rules_template' AND value LIKE '%institution_rank%';
