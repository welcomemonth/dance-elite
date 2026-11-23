-- 0017: 赛事抽签域。
--
-- 抽签从 JSON 文件迁入数据库：一次抽签 = 一个 lottery_sessions，
-- 选手记录在 lottery_players，组别切分在 lottery_groups。
-- 证件号不存明文：id_card_hash 为 HMAC（与 players 一致，用于比对），
-- id_card_enc 为 AES-GCM 密文（仅 Excel 导入来源有，导出时解密）；
-- 从赛事报名生成时只能拿到报名库的哈希，id_card_enc 为空。

CREATE TABLE lottery_sessions (
    id                    INTEGER PRIMARY KEY,
    event_id              INTEGER REFERENCES events(id),
    event_name            TEXT NOT NULL,
    token                 TEXT NOT NULL UNIQUE,
    source                TEXT NOT NULL CHECK (source IN ('excel', 'event')),
    start_at              TEXT,
    end_at                TEXT,
    remind_before_minutes INTEGER NOT NULL DEFAULT 120 CHECK (remind_before_minutes >= 0),
    sms_remind_template   TEXT,
    sms_result_template   TEXT,
    created_by            INTEGER REFERENCES admins(id),
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%S', 'now', 'localtime') || '+08:00'),
    updated_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%S', 'now', 'localtime') || '+08:00')
);

CREATE INDEX idx_lottery_sessions_event ON lottery_sessions(event_id);
CREATE INDEX idx_lottery_sessions_end_at ON lottery_sessions(end_at);

CREATE TRIGGER trg_lottery_sessions_updated_at
AFTER UPDATE ON lottery_sessions
FOR EACH ROW
WHEN NEW.updated_at IS OLD.updated_at
BEGIN
    UPDATE lottery_sessions
    SET updated_at = strftime('%Y-%m-%dT%H:%M:%S', 'now', 'localtime') || '+08:00'
    WHERE id = NEW.id;
END;

CREATE TABLE lottery_groups (
    id         INTEGER PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES lottery_sessions(id) ON DELETE CASCADE,
    genre      TEXT NOT NULL CHECK (genre IN ('classical', 'folk')),
    age_group  TEXT NOT NULL CHECK (age_group IN ('U11', 'U13', 'U15')),
    letter     TEXT NOT NULL,
    size       INTEGER NOT NULL CHECK (size > 0),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%S', 'now', 'localtime') || '+08:00'),
    UNIQUE (session_id, genre, age_group, letter)
);

CREATE INDEX idx_lottery_groups_session ON lottery_groups(session_id);

CREATE TABLE lottery_players (
    id               INTEGER PRIMARY KEY,
    session_id       INTEGER NOT NULL REFERENCES lottery_sessions(id) ON DELETE CASCADE,
    registration_id  INTEGER REFERENCES registrations(id),
    name             TEXT NOT NULL,
    id_card_hash     TEXT NOT NULL,
    id_card_masked   TEXT,
    id_card_enc      TEXT,
    genre            TEXT NOT NULL CHECK (genre IN ('classical', 'folk')),
    age_group        TEXT NOT NULL CHECK (age_group IN ('U11', 'U13', 'U15')),
    institution      TEXT,
    program          TEXT,
    phone            TEXT,
    group_letter     TEXT,
    slot_no          INTEGER CHECK (slot_no IS NULL OR slot_no > 0),
    drawn_at         TEXT,
    auto_drawn       INTEGER NOT NULL DEFAULT 0 CHECK (auto_drawn IN (0, 1)),
    remind_sent_at   TEXT,
    remind_attempts  INTEGER NOT NULL DEFAULT 0 CHECK (remind_attempts >= 0),
    result_sent_at   TEXT,
    result_attempts  INTEGER NOT NULL DEFAULT 0 CHECK (result_attempts >= 0),
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%S', 'now', 'localtime') || '+08:00'),
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%S', 'now', 'localtime') || '+08:00')
);

-- 同一赛事同一选手的同一舞种只能有一条抽签记录。
CREATE UNIQUE INDEX uq_lottery_players_entry
    ON lottery_players(session_id, id_card_hash, genre);

-- 已分配的槽位必须唯一；未抽签（slot_no IS NULL）不参与。
CREATE UNIQUE INDEX uq_lottery_players_slot
    ON lottery_players(session_id, genre, age_group, group_letter, slot_no)
    WHERE slot_no IS NOT NULL;

-- 从赛事报名生成时，一条报名只能对应一条抽签记录。
CREATE UNIQUE INDEX uq_lottery_players_registration
    ON lottery_players(session_id, registration_id)
    WHERE registration_id IS NOT NULL;

CREATE INDEX idx_lottery_players_session ON lottery_players(session_id);
CREATE INDEX idx_lottery_players_hash ON lottery_players(session_id, id_card_hash);

CREATE TRIGGER trg_lottery_players_updated_at
AFTER UPDATE ON lottery_players
FOR EACH ROW
WHEN NEW.updated_at IS OLD.updated_at
BEGIN
    UPDATE lottery_players
    SET updated_at = strftime('%Y-%m-%dT%H:%M:%S', 'now', 'localtime') || '+08:00'
    WHERE id = NEW.id;
END;
