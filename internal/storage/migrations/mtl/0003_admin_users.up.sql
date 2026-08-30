ALTER TABLE mtl_users
    ADD COLUMN session_epoch INTEGER NOT NULL DEFAULT 0 CHECK (session_epoch >= 0);

ALTER TABLE mtl_groups
    ADD COLUMN version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0);

ALTER TABLE mtl_groups
    ADD COLUMN updated_at DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00';

UPDATE mtl_groups SET updated_at = CURRENT_TIMESTAMP;

CREATE TRIGGER mtl_groups_set_insert_timestamp
AFTER INSERT ON mtl_groups
WHEN NEW.updated_at = '1970-01-01 00:00:00'
BEGIN
    UPDATE mtl_groups SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
