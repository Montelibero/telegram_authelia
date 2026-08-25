CREATE TABLE mtl_registration_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL COLLATE NOCASE,
    provider_user_id TEXT NOT NULL,
    provider_username TEXT NULL,
    display_name TEXT NULL,
    proposed_username TEXT NULL COLLATE NOCASE,
    proposed_email TEXT NULL COLLATE NOCASE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    requested_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at DATETIME NULL,
    resolved_by_user_id INTEGER NULL REFERENCES mtl_users(id) ON DELETE SET NULL,
    approved_user_id INTEGER NULL REFERENCES mtl_users(id) ON DELETE SET NULL,
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX mtl_registration_requests_status_requested
    ON mtl_registration_requests (status, requested_at);
