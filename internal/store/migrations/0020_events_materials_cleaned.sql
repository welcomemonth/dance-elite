-- 0020: 赛事媒体清理标记（S5-3，报名功能 Q22）。
--
-- 赛事进入 finished 且超过配置的宽限期后，定时任务删除 uploads/{event_id}/，
-- 并把报名的 photo_path/audio_path 置 NULL（文本资料保留、头像不受影响）。
-- materials_cleaned_at 用于幂等：已清理的赛事不再重复处理。
ALTER TABLE events ADD COLUMN materials_cleaned_at TEXT;
