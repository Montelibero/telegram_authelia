package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/authelia/authelia/v4/internal/logging"
	"github.com/authelia/authelia/v4/internal/model"
)

// ListMTLAdminUsers returns all administrative user summaries in username order.
func (p *SQLProvider) ListMTLAdminUsers(ctx context.Context) ([]model.MTLAdminUserSummary, error) {
	var usernames []string
	if err := p.db.SelectContext(ctx, &usernames, `SELECT username FROM mtl_users ORDER BY username`); err != nil {
		return nil, fmt.Errorf("failed to list MTL admin users: %w", err)
	}
	users := make([]model.MTLAdminUserSummary, 0, len(usernames))
	for _, username := range usernames {
		details, found, err := p.LoadMTLAdminUser(ctx, username)
		if err != nil {
			return nil, err
		}
		if found {
			users = append(users, details.MTLAdminUserSummary)
		}
	}
	return users, nil
}

// LoadMTLAdminUser returns the safe administrative detail view of a user.
func (p *SQLProvider) LoadMTLAdminUser(ctx context.Context, username string) (details model.MTLAdminUserDetails, found bool, err error) {
	var user model.MTLUser
	query := p.db.Rebind(`SELECT id, username, display_name, status, password_hash, version, session_epoch, created_at, updated_at FROM mtl_users WHERE username = ?`)
	if err = p.db.GetContext(ctx, &user, query, username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return details, false, nil
		}
		return details, false, fmt.Errorf("failed to load MTL admin user: %w", err)
	}
	details.MTLAdminUserSummary = model.MTLAdminUserSummary{
		Username: user.Username, DisplayName: user.DisplayName, Status: user.Status, Version: user.Version,
		PasswordEnabled: user.PasswordHash.Valid, Groups: []string{},
	}
	details.SessionEpoch = user.SessionEpoch
	if err = p.db.SelectContext(ctx, &details.Emails, p.db.Rebind(`SELECT id, user_id, email, is_primary, verified, created_at, updated_at FROM mtl_user_emails WHERE user_id = ? ORDER BY email`), user.ID); err != nil {
		return details, false, fmt.Errorf("failed to load MTL admin user emails: %w", err)
	}
	for _, email := range details.Emails {
		if email.Primary {
			details.PrimaryEmail = email.Email
			break
		}
	}
	if details.PrimaryEmail == "" {
		return details, false, ErrMTLPrimaryEmailRequired
	}
	if err = p.db.SelectContext(ctx, &details.Identities, p.db.Rebind(`SELECT id, user_id, provider, provider_user_id, provider_username, created_at, updated_at FROM mtl_user_identities WHERE user_id = ? ORDER BY provider`), user.ID); err != nil {
		return details, false, fmt.Errorf("failed to load MTL admin user identities: %w", err)
	}
	if err = p.db.SelectContext(ctx, &details.Groups, p.db.Rebind(`SELECT g.name FROM mtl_groups g INNER JOIN mtl_group_memberships gm ON gm.group_id = g.id WHERE gm.user_id = ? ORDER BY g.name`), user.ID); err != nil {
		return details, false, fmt.Errorf("failed to load MTL admin user groups: %w", err)
	}
	return details, true, nil
}

// CreateMTLAdminUser creates one active passwordless user with a verified primary email.
func (p *SQLProvider) CreateMTLAdminUser(ctx context.Context, create model.MTLAdminUserCreate, actor string) (details model.MTLAdminUserDetails, err error) {
	if strings.TrimSpace(create.Username) == "" || strings.TrimSpace(create.Email) == "" {
		return details, ErrMTLPrimaryEmailRequired
	}
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return details, fmt.Errorf("failed to begin MTL admin user creation: %w", err)
	}
	defer rollbackMTLAdmin(tx, &err)
	actorID, err := loadOptionalMTLActor(ctx, tx, actor)
	if err != nil {
		return details, err
	}
	displayName := strings.TrimSpace(create.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(create.Username)
	}
	result, err := tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_users (username, display_name, status, password_hash) VALUES (?, ?, 'active', NULL)`), strings.TrimSpace(create.Username), displayName)
	if err != nil {
		return details, mapMTLConflict("failed to create MTL admin user", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		return details, fmt.Errorf("failed to obtain MTL admin user ID: %w", err)
	}
	if _, err = tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_user_emails (user_id, email, is_primary, verified) VALUES (?, ?, 1, 1)`), userID, strings.TrimSpace(create.Email)); err != nil {
		return details, mapMTLConflict("failed to create MTL admin user email", err)
	}
	if telegramID := strings.TrimSpace(create.TelegramID); telegramID != "" {
		if _, err = tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_user_identities (user_id, provider, provider_user_id, provider_username) VALUES (?, 'telegram', ?, '')`), userID, telegramID); err != nil {
			return details, mapMTLConflict("failed to link MTL admin user Telegram identity", err)
		}
	}
	groups := append([]string(nil), create.Groups...)
	sort.Strings(groups)
	for _, group := range groups {
		result, execErr := tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_group_memberships (user_id, group_id) SELECT ?, id FROM mtl_groups WHERE name = ?`), userID, group)
		if execErr != nil {
			return details, mapMTLConflict("failed to create MTL admin user membership", execErr)
		}
		if execErr = requireOneMTLRegistrationRow(result); execErr != nil {
			return details, ErrMTLConflict
		}
	}
	if err = auditMTLAdmin(ctx, tx, actorID, "user.created", "user", strings.TrimSpace(create.Username)); err != nil {
		return details, err
	}
	if err = tx.Commit(); err != nil {
		return details, fmt.Errorf("failed to commit MTL admin user creation: %w", err)
	}
	details, _, err = p.LoadMTLAdminUser(ctx, create.Username)
	return details, err
}

// LinkMTLAdminUserIdentity assigns a stable provider identity with optimistic concurrency.
func (p *SQLProvider) LinkMTLAdminUserIdentity(ctx context.Context, username string, link model.MTLAdminIdentityLink, actor string) (details model.MTLAdminUserDetails, err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return details, fmt.Errorf("failed to begin MTL admin identity link: %w", err)
	}
	defer rollbackMTLAdmin(tx, &err)
	userID, err := loadMTLAdminUserVersion(ctx, tx, username, link.ExpectedVersion)
	if err != nil {
		return details, err
	}
	actorID, err := loadOptionalMTLActor(ctx, tx, actor)
	if err != nil {
		return details, err
	}
	if _, err = tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_user_identities (user_id, provider, provider_user_id, provider_username) VALUES (?, ?, ?, '')`), userID, link.Provider, strings.TrimSpace(link.ProviderUserID)); err != nil {
		return details, mapMTLConflict("failed to link MTL admin identity", err)
	}
	if err = bumpMTLAdminUserVersion(ctx, tx, userID, link.ExpectedVersion, true); err != nil {
		return details, err
	}
	if err = auditMTLAdmin(ctx, tx, actorID, "identity.linked", "user", username); err != nil {
		return details, err
	}
	if err = tx.Commit(); err != nil {
		return details, fmt.Errorf("failed to commit MTL admin identity link: %w", err)
	}
	details, _, err = p.LoadMTLAdminUser(ctx, username)
	return details, err
}

// UpdateMTLAdminUser changes display name/status with optimistic concurrency.
func (p *SQLProvider) UpdateMTLAdminUser(ctx context.Context, username string, update model.MTLAdminUserUpdate, actor string) (details model.MTLAdminUserDetails, err error) {
	if update.Status != model.MTLUserStatusActive && update.Status != model.MTLUserStatusDisabled {
		return details, ErrMTLConflict
	}
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return details, fmt.Errorf("failed to begin MTL admin user update: %w", err)
	}
	defer rollbackMTLAdmin(tx, &err)
	userID, err := loadMTLAdminUserVersion(ctx, tx, username, update.ExpectedVersion)
	if err != nil {
		return details, err
	}
	actorID, err := loadOptionalMTLActor(ctx, tx, actor)
	if err != nil {
		return details, err
	}
	displayName := strings.TrimSpace(update.DisplayName)
	if displayName == "" {
		displayName = username
	}
	result, err := tx.ExecContext(ctx, tx.Rebind(`UPDATE mtl_users SET display_name = ?, status = ?, session_epoch = CASE WHEN status <> 'disabled' AND ? = 'disabled' THEN session_epoch + 1 ELSE session_epoch END, version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND version = ?`), displayName, update.Status, update.Status, userID, update.ExpectedVersion)
	if err != nil {
		return details, fmt.Errorf("failed to update MTL admin user: %w", err)
	}
	if err = requireMTLAdminRow(result); err != nil {
		return details, err
	}
	if err = auditMTLAdmin(ctx, tx, actorID, "user.updated", "user", username); err != nil {
		return details, err
	}
	if err = tx.Commit(); err != nil {
		return details, fmt.Errorf("failed to commit MTL admin user update: %w", err)
	}
	details, _, err = p.LoadMTLAdminUser(ctx, username)
	return details, err
}

// AddMTLAdminUserEmail adds a verified email and optionally makes it primary.
func (p *SQLProvider) AddMTLAdminUserEmail(ctx context.Context, username string, create model.MTLAdminEmailCreate, actor string) (model.MTLAdminUserDetails, error) {
	return p.mutateMTLAdminEmail(ctx, username, create.ExpectedVersion, actor, func(tx SQLXTx, userID int64) error {
		if create.Primary {
			if _, err := tx.ExecContext(ctx, tx.Rebind(`UPDATE mtl_user_emails SET is_primary = 0, updated_at = CURRENT_TIMESTAMP WHERE user_id = ? AND is_primary = 1`), userID); err != nil {
				return fmt.Errorf("failed to clear MTL primary email: %w", err)
			}
		}
		_, err := tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_user_emails (user_id, email, is_primary, verified) VALUES (?, ?, ?, 1)`), userID, strings.TrimSpace(create.Email), create.Primary)
		if err != nil {
			return mapMTLConflict("failed to add MTL admin user email", err)
		}
		return nil
	}, "email.added", create.Primary)
}

// SetMTLAdminPrimaryEmail selects an existing email as primary.
func (p *SQLProvider) SetMTLAdminPrimaryEmail(ctx context.Context, username, email string, expectedVersion int, actor string) (model.MTLAdminUserDetails, error) {
	return p.mutateMTLAdminEmail(ctx, username, expectedVersion, actor, func(tx SQLXTx, userID int64) error {
		var id int64
		if err := tx.GetContext(ctx, &id, tx.Rebind(`SELECT id FROM mtl_user_emails WHERE user_id = ? AND email = ?`), userID, email); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrMTLConflict
			}
			return fmt.Errorf("failed to load MTL primary email candidate: %w", err)
		}
		if _, err := tx.ExecContext(ctx, tx.Rebind(`UPDATE mtl_user_emails SET is_primary = CASE WHEN id = ? THEN 1 ELSE 0 END, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?`), id, userID); err != nil {
			return fmt.Errorf("failed to select MTL primary email: %w", err)
		}
		return nil
	}, "email.primary_changed", true)
}

// DeleteMTLAdminUserEmail removes a non-primary email.
func (p *SQLProvider) DeleteMTLAdminUserEmail(ctx context.Context, username, email string, expectedVersion int, actor string) (model.MTLAdminUserDetails, error) {
	return p.mutateMTLAdminEmail(ctx, username, expectedVersion, actor, func(tx SQLXTx, userID int64) error {
		var primary bool
		if err := tx.GetContext(ctx, &primary, tx.Rebind(`SELECT is_primary FROM mtl_user_emails WHERE user_id = ? AND email = ?`), userID, email); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrMTLConflict
			}
			return fmt.Errorf("failed to load MTL admin user email: %w", err)
		}
		if primary {
			return ErrMTLPrimaryEmailRequired
		}
		if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM mtl_user_emails WHERE user_id = ? AND email = ?`), userID, email); err != nil {
			return fmt.Errorf("failed to delete MTL admin user email: %w", err)
		}
		return nil
	}, "email.removed", false)
}

// UnlinkMTLAdminUserIdentity removes a provider identity with optimistic concurrency.
func (p *SQLProvider) UnlinkMTLAdminUserIdentity(ctx context.Context, username, provider string, expectedVersion int, actor string) (details model.MTLAdminUserDetails, err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return details, fmt.Errorf("failed to begin MTL admin identity unlink: %w", err)
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
	result, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM mtl_user_identities WHERE user_id = ? AND provider = ?`), userID, provider)
	if err != nil {
		return details, fmt.Errorf("failed to unlink MTL admin identity: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return details, ErrMTLIdentityNotFound
	}
	if err = bumpMTLAdminUserVersion(ctx, tx, userID, expectedVersion, true); err != nil {
		return details, err
	}
	if err = auditMTLAdmin(ctx, tx, actorID, "identity.unlinked", "user", username); err != nil {
		return details, err
	}
	if err = tx.Commit(); err != nil {
		return details, fmt.Errorf("failed to commit MTL admin identity unlink: %w", err)
	}
	details, _, err = p.LoadMTLAdminUser(ctx, username)
	return details, err
}

func (p *SQLProvider) mutateMTLAdminEmail(ctx context.Context, username string, expectedVersion int, actor string, mutation func(SQLXTx, int64) error, event string, revokeSessions bool) (details model.MTLAdminUserDetails, err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return details, fmt.Errorf("failed to begin MTL admin email mutation: %w", err)
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
	if err = mutation(tx, userID); err != nil {
		return details, err
	}
	if err = bumpMTLAdminUserVersion(ctx, tx, userID, expectedVersion, revokeSessions); err != nil {
		return details, err
	}
	if err = auditMTLAdmin(ctx, tx, actorID, event, "user", username); err != nil {
		return details, err
	}
	if err = tx.Commit(); err != nil {
		return details, fmt.Errorf("failed to commit MTL admin email mutation: %w", err)
	}
	details, _, err = p.LoadMTLAdminUser(ctx, username)
	return details, err
}

func loadMTLAdminUserVersion(ctx context.Context, tx SQLXTx, username string, expectedVersion int) (int64, error) {
	var row struct {
		ID      int64 `db:"id"`
		Version int   `db:"version"`
	}
	if err := tx.GetContext(ctx, &row, tx.Rebind(`SELECT id, version FROM mtl_users WHERE username = ?`), username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrMTLUserNotFound
		}
		return 0, fmt.Errorf("failed to load MTL admin user version: %w", err)
	}
	if row.Version != expectedVersion {
		return 0, ErrMTLVersionConflict
	}
	return row.ID, nil
}

func bumpMTLAdminUserVersion(ctx context.Context, tx SQLXTx, userID int64, expectedVersion int, revokeSessions bool) error {
	epochIncrement := 0
	if revokeSessions {
		epochIncrement = 1
	}
	result, err := tx.ExecContext(ctx, tx.Rebind(`UPDATE mtl_users SET version = version + 1, session_epoch = session_epoch + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND version = ?`), epochIncrement, userID, expectedVersion)
	if err != nil {
		return fmt.Errorf("failed to bump MTL admin user version: %w", err)
	}
	return requireMTLAdminRow(result)
}

func auditMTLAdmin(ctx context.Context, tx SQLXTx, actorID any, event, targetType, targetID string) error {
	if _, err := tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_audit_events (actor_user_id, event_type, target_type, target_id) VALUES (?, ?, ?, ?)`), actorID, event, targetType, targetID); err != nil {
		return fmt.Errorf("failed to audit MTL admin mutation: %w", err)
	}
	logging.Logger().WithField("actor_user_id", actorID).
		WithField("audit_event", event).
		WithField("target_type", targetType).
		WithField("target_id", targetID).
		Info("Administrator audit event recorded")
	return nil
}

func requireMTLAdminRow(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check MTL admin mutation: %w", err)
	}
	if rows != 1 {
		return ErrMTLVersionConflict
	}
	return nil
}

func rollbackMTLAdmin(tx SQLXTx, err *error) {
	if *err != nil {
		_ = tx.Rollback()
	}
}
