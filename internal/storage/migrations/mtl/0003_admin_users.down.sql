DROP TRIGGER IF EXISTS mtl_groups_set_insert_timestamp;
ALTER TABLE mtl_groups DROP COLUMN updated_at;
ALTER TABLE mtl_groups DROP COLUMN version;
ALTER TABLE mtl_users DROP COLUMN session_epoch;
