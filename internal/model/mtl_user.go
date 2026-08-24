package model

import "time"

// MTLUser is an overlay-owned local user.
type MTLUser struct {
	ID           int64     `db:"id"`
	Username     string    `db:"username"`
	DisplayName  string    `db:"display_name"`
	Status       string    `db:"status"`
	PasswordHash *string   `db:"password_hash"`
	Version      int       `db:"version"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// MTLUserEmail is an email address owned by an overlay user.
type MTLUserEmail struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	Email     string    `db:"email"`
	Primary   bool      `db:"is_primary"`
	Verified  bool      `db:"verified"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// MTLUserIdentity links a local user to a stable external identity.
type MTLUserIdentity struct {
	ID               int64     `db:"id"`
	UserID           int64     `db:"user_id"`
	Provider         string    `db:"provider"`
	ProviderUserID   string    `db:"provider_user_id"`
	ProviderUsername *string   `db:"provider_username"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

// MTLGroup is an overlay-owned Authelia group.
type MTLGroup struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

const (
	MTLUserStatusActive   = "active"
	MTLUserStatusDisabled = "disabled"
)
