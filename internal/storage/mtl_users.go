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
	ErrMTLVersionConflict      = errors.New("MTL user version conflict")
	ErrMTLConflict             = errors.New("MTL user data conflict")
	ErrMTLPrimaryEmailRequired = errors.New("MTL user requires exactly one primary email")
)

// LoadMTLUser loads the authentication-facing details for a local user.
func (p *SQLProvider) LoadMTLUser(ctx context.Context, username string) (details model.MTLUserDetails, found bool, err error) {
	query := p.db.Rebind(`
SELECT id, username, display_name, status, password_hash, version, created_at, updated_at
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
