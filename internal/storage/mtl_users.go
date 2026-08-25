package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/mattn/go-sqlite3"

	"github.com/authelia/authelia/v4/internal/model"
)

var (
	ErrMTLVersionConflict          = errors.New("MTL user version conflict")
	ErrMTLConflict                 = errors.New("MTL user data conflict")
	ErrMTLPrimaryEmailRequired     = errors.New("MTL user requires exactly one primary email")
	ErrMTLUserNotFound             = errors.New("MTL user not found")
	ErrMTLIdentityNotFound         = errors.New("MTL user identity not found")
	ErrMTLTelegramIdentityRequired = errors.New("MTL Telegram identity required")
	ErrMTLLastPasswordAdmin        = errors.New("MTL last password administrator")
)

// LinkMTLUserIdentity links a stable provider identity to an existing local user.
func (p *SQLProvider) LinkMTLUserIdentity(ctx context.Context, username, provider, providerUserID, providerUsername string) (err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin MTL identity link: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var userID int64
	if err = tx.GetContext(ctx, &userID, tx.Rebind(`SELECT id FROM mtl_users WHERE username = ?`), username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrMTLUserNotFound
		}
		return fmt.Errorf("failed to load MTL identity user: %w", err)
	}

	var display any
	if providerUsername != "" {
		display = providerUsername
	}
	if _, err = tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_user_identities (user_id, provider, provider_user_id, provider_username) VALUES (?, ?, ?, ?)`), userID, provider, providerUserID, display); err != nil {
		return mapMTLConflict("failed to link MTL user identity", err)
	}
	if _, err = tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_audit_events (actor_user_id, event_type, target_type, target_id) VALUES (?, 'identity.linked', 'user', ?)`), userID, username); err != nil {
		return fmt.Errorf("failed to audit MTL identity link: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit MTL identity link: %w", err)
	}
	return nil
}

// LoadMTLUserIdentity loads one provider identity for a local username.
func (p *SQLProvider) LoadMTLUserIdentity(ctx context.Context, username, provider string) (identity model.MTLUserIdentity, found bool, err error) {
	query := p.db.Rebind(`SELECT i.id, i.user_id, i.provider, i.provider_user_id, i.provider_username, i.created_at, i.updated_at FROM mtl_user_identities i INNER JOIN mtl_users u ON u.id = i.user_id WHERE u.username = ? AND i.provider = ?`)
	if err = p.db.GetContext(ctx, &identity, query, username, provider); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return identity, false, nil
		}
		return identity, false, fmt.Errorf("failed to load MTL user identity: %w", err)
	}
	return identity, true, nil
}

// LoadMTLUserByIdentity resolves a stable provider identity to local user details.
func (p *SQLProvider) LoadMTLUserByIdentity(ctx context.Context, provider, providerUserID string) (details model.MTLUserDetails, found bool, err error) {
	var username string
	query := p.db.Rebind(`SELECT u.username FROM mtl_users u INNER JOIN mtl_user_identities i ON i.user_id = u.id WHERE i.provider = ? AND i.provider_user_id = ?`)
	if err = p.db.GetContext(ctx, &username, query, provider, providerUserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return details, false, nil
		}
		return details, false, fmt.Errorf("failed to resolve MTL user identity: %w", err)
	}
	return p.LoadMTLUser(ctx, username)
}

// UnlinkMTLUserIdentity removes a provider identity from the exact local username.
func (p *SQLProvider) UnlinkMTLUserIdentity(ctx context.Context, username, provider string) (err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin MTL identity unlink: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var userID int64
	if err = tx.GetContext(ctx, &userID, tx.Rebind(`SELECT id FROM mtl_users WHERE username = ?`), username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrMTLUserNotFound
		}
		return fmt.Errorf("failed to load MTL identity user: %w", err)
	}
	result, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM mtl_user_identities WHERE user_id = ? AND provider = ?`), userID, provider)
	if err != nil {
		return fmt.Errorf("failed to unlink MTL user identity: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check MTL identity unlink: %w", err)
	}
	if affected != 1 {
		return ErrMTLIdentityNotFound
	}
	if _, err = tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_audit_events (actor_user_id, event_type, target_type, target_id) VALUES (?, 'identity.unlinked', 'user', ?)`), userID, username); err != nil {
		return fmt.Errorf("failed to audit MTL identity unlink: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit MTL identity unlink: %w", err)
	}
	return nil
}

// LoadMTLUser loads the authentication-facing details for a local user.
func (p *SQLProvider) LoadMTLUser(ctx context.Context, username string) (details model.MTLUserDetails, found bool, err error) {
	query := p.db.Rebind(`
SELECT id, username, display_name, status, password_hash, version, session_epoch, created_at, updated_at
FROM mtl_users
WHERE username = ?`)

	if err = p.db.GetContext(ctx, &details.User, query, username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return details, false, nil
		}

		return details, false, fmt.Errorf("failed to load MTL user: %w", err)
	}

	query = p.db.Rebind(`SELECT email FROM mtl_user_emails WHERE user_id = ? AND is_primary = 1`)
	if err = p.db.GetContext(ctx, &details.PrimaryEmail, query, details.User.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return details, false, ErrMTLPrimaryEmailRequired
		}

		return details, false, fmt.Errorf("failed to load MTL user primary email: %w", err)
	}

	query = p.db.Rebind(`
SELECT g.name
FROM mtl_groups AS g
INNER JOIN mtl_group_memberships AS gm ON gm.group_id = g.id
WHERE gm.user_id = ?
ORDER BY g.name`)
	if err = p.db.SelectContext(ctx, &details.Groups, query, details.User.ID); err != nil {
		return details, false, fmt.Errorf("failed to load MTL user groups: %w", err)
	}

	return details, true, nil
}

// FindMTLUserByEmail resolves the owner of a normalized email address.
func (p *SQLProvider) FindMTLUserByEmail(ctx context.Context, email string) (username string, found bool, err error) {
	query := p.db.Rebind(`
SELECT u.username
FROM mtl_users AS u
INNER JOIN mtl_user_emails AS e ON e.user_id = u.id
WHERE e.email = ?`)
	if err = p.db.GetContext(ctx, &username, query, email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}

		return "", false, fmt.Errorf("failed to find MTL user by email: %w", err)
	}

	return username, true, nil
}

// UpdateMTLUserPassword updates a password using optimistic version checking.
func (p *SQLProvider) UpdateMTLUserPassword(ctx context.Context, userID int64, passwordHash *string, expectedVersion int) (err error) {
	query := p.db.Rebind(`
UPDATE mtl_users
SET password_hash = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND version = ?`)
	result, err := p.db.ExecContext(ctx, query, passwordHash, userID, expectedVersion)
	if err != nil {
		return fmt.Errorf("failed to update MTL user password: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check MTL user password update: %w", err)
	}

	if affected != 1 {
		return ErrMTLVersionConflict
	}

	return nil
}

// SetMTLSelfServicePassword stores a password, rotates other sessions, and audits the mutation.
func (p *SQLProvider) SetMTLSelfServicePassword(ctx context.Context, username, passwordHash string, expectedVersion int, actor string) (details model.MTLAdminUserDetails, err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return details, fmt.Errorf("failed to begin MTL self-service password set: %w", err)
	}
	defer rollbackMTLAdmin(tx, &err)

	userID, err := loadMTLAdminUserVersion(ctx, tx, username, expectedVersion)
	if err != nil {
		return details, err
	}
	actorID, err := loadOptionalMTLActor(ctx, tx, actor)
	if err != nil {
		return details, err
	}
	result, err := tx.ExecContext(ctx, tx.Rebind(`UPDATE mtl_users SET password_hash = ?, version = version + 1, session_epoch = session_epoch + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND version = ?`), passwordHash, userID, expectedVersion)
	if err != nil {
		return details, fmt.Errorf("failed to set MTL self-service password: %w", err)
	}
	if err = requireMTLAdminRow(result); err != nil {
		return details, err
	}
	if err = auditMTLAdmin(ctx, tx, actorID, "password.set", "user", username); err != nil {
		return details, err
	}
	if err = tx.Commit(); err != nil {
		return details, fmt.Errorf("failed to commit MTL self-service password set: %w", err)
	}

	details, _, err = p.LoadMTLAdminUser(ctx, username)
	return details, err
}

// RemoveMTLSelfServicePassword disables password login when Telegram remains available.
func (p *SQLProvider) RemoveMTLSelfServicePassword(ctx context.Context, username string, expectedVersion int, actor string) (details model.MTLAdminUserDetails, err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return details, fmt.Errorf("failed to begin MTL self-service password removal: %w", err)
	}
	defer rollbackMTLAdmin(tx, &err)

	userID, err := loadMTLAdminUserVersion(ctx, tx, username, expectedVersion)
	if err != nil {
		return details, err
	}
	actorID, err := loadOptionalMTLActor(ctx, tx, actor)
	if err != nil {
		return details, err
	}

	query := tx.Rebind(`
UPDATE mtl_users
SET password_hash = NULL, version = version + 1, session_epoch = session_epoch + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND version = ?
  AND EXISTS (SELECT 1 FROM mtl_user_identities WHERE user_id = ? AND provider = 'telegram')
  AND (
    NOT EXISTS (
      SELECT 1 FROM mtl_group_memberships gm
      INNER JOIN mtl_groups g ON g.id = gm.group_id
      WHERE gm.user_id = ? AND g.name = 'admins'
    )
    OR EXISTS (
      SELECT 1 FROM mtl_users other
      INNER JOIN mtl_group_memberships gm ON gm.user_id = other.id
      INNER JOIN mtl_groups g ON g.id = gm.group_id
      WHERE other.id <> ? AND other.status = 'active' AND other.password_hash IS NOT NULL AND g.name = 'admins'
    )
  )`)
	result, err := tx.ExecContext(ctx, query, userID, expectedVersion, userID, userID, userID)
	if err != nil {
		return details, fmt.Errorf("failed to remove MTL self-service password: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return details, fmt.Errorf("failed to check MTL self-service password removal: %w", err)
	}
	if rows != 1 {
		var telegramCount int
		if err = tx.GetContext(ctx, &telegramCount, tx.Rebind(`SELECT COUNT(*) FROM mtl_user_identities WHERE user_id = ? AND provider = 'telegram'`), userID); err != nil {
			return details, fmt.Errorf("failed to check MTL Telegram identity: %w", err)
		}
		if telegramCount == 0 {
			return details, ErrMTLTelegramIdentityRequired
		}
		return details, ErrMTLLastPasswordAdmin
	}
	if err = auditMTLAdmin(ctx, tx, actorID, "password.removed", "user", username); err != nil {
		return details, err
	}
	if err = tx.Commit(); err != nil {
		return details, fmt.Errorf("failed to commit MTL self-service password removal: %w", err)
	}

	details, _, err = p.LoadMTLAdminUser(ctx, username)
	return details, err
}

// ImportMTLUsers creates complete user records in one transaction.
func (p *SQLProvider) ImportMTLUsers(ctx context.Context, users []model.MTLUserImport) (err error) {
	for _, user := range users {
		primary := 0
		for _, email := range user.Emails {
			if email.Primary {
				primary++
			}
		}

		if primary != 1 {
			return ErrMTLPrimaryEmailRequired
		}
	}

	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin MTL user import: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, user := range users {
		status := user.Status
		if status == "" {
			status = model.MTLUserStatusActive
		}

		query := tx.Rebind(`INSERT INTO mtl_users (username, display_name, status, password_hash) VALUES (?, ?, ?, ?)`)
		result, execErr := tx.ExecContext(ctx, query, user.Username, user.DisplayName, status, user.PasswordHash)
		if execErr != nil {
			return mapMTLConflict("failed to insert MTL user", execErr)
		}

		userID, idErr := result.LastInsertId()
		if idErr != nil {
			return fmt.Errorf("failed to obtain imported MTL user ID: %w", idErr)
		}

		for _, email := range user.Emails {
			query = tx.Rebind(`INSERT INTO mtl_user_emails (user_id, email, is_primary, verified) VALUES (?, ?, ?, ?)`)
			if _, execErr = tx.ExecContext(ctx, query, userID, email.Email, email.Primary, email.Verified); execErr != nil {
				return mapMTLConflict("failed to insert MTL user email", execErr)
			}
		}

		groups := append([]string(nil), user.Groups...)
		sort.Strings(groups)
		for _, group := range groups {
			query = tx.Rebind(`INSERT INTO mtl_groups (name) VALUES (?) ON CONFLICT(name) DO NOTHING`)
			if _, execErr = tx.ExecContext(ctx, query, group); execErr != nil {
				return mapMTLConflict("failed to insert MTL group", execErr)
			}

			var groupID int64
			query = tx.Rebind(`SELECT id FROM mtl_groups WHERE name = ?`)
			if execErr = tx.GetContext(ctx, &groupID, query, group); execErr != nil {
				return fmt.Errorf("failed to load imported MTL group: %w", execErr)
			}

			query = tx.Rebind(`INSERT INTO mtl_group_memberships (user_id, group_id) VALUES (?, ?)`)
			if _, execErr = tx.ExecContext(ctx, query, userID, groupID); execErr != nil {
				return mapMTLConflict("failed to insert MTL group membership", execErr)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit MTL user import: %w", err)
	}

	return nil
}

func mapMTLConflict(operation string, err error) error {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && sqliteErr.Code == sqlite3.ErrConstraint {
		return fmt.Errorf("%s: %w", operation, ErrMTLConflict)
	}

	return fmt.Errorf("%s: %w", operation, err)
}
