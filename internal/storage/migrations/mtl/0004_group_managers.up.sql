CREATE TABLE mtl_group_managers (
    user_id INTEGER NOT NULL REFERENCES mtl_users(id) ON DELETE CASCADE,
    group_id INTEGER NOT NULL REFERENCES mtl_groups(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, group_id)
);

CREATE INDEX mtl_group_managers_group_id ON mtl_group_managers (group_id);
