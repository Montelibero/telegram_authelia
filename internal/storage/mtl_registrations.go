package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/authelia/authelia/v4/internal/model"
)

var (
	ErrMTLRegistrationNotFound   = errors.New("MTL registration request not found")
	ErrMTLRegistrationIncomplete = errors.New("MTL registration approval requires a username and email")
	ErrMTLRegistrationTerminal   = errors.New("MTL registration request is already resolved")
)

const selectMTLRegistration = `
SELECT id, provider, provider_user_id, provider_username, display_name, proposed_username, proposed_email,
       status, version, requested_at, updated_at, resolved_at, resolved_by_user_id, approved_user_id
FROM mtl_registration_requests`

// UpsertMTLRegistration records current provider data without reopening resolved requests.
func (p *SQLProvider) UpsertMTLRegistration(ctx context.Context, candidate model.MTLRegistrationCandidate) (request model.MTLRegistrationRequest, err error) {
	query := p.db.Rebind(`
INSERT INTO mtl_registration_requests
    (provider, provider_user_id, provider_username, display_name, proposed_username, proposed_email)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(provider, provider_user_id) DO UPDATE SET
    provider_username = excluded.provider_username,
    display_name = excluded.display_name,
    proposed_username = excluded.proposed_username,
    proposed_email = excluded.proposed_email,
    version = mtl_registration_requests.version + 1,
    updated_at = CURRENT_TIMESTAMP`)
	if _, err = p.db.ExecContext(ctx, query,
		candidate.Provider, candidate.ProviderUserID, nullableMTLString(candidate.ProviderUsername),
		nullableMTLString(candidate.DisplayName), nullableMTLString(candidate.ProposedUsername), nullableMTLString(candidate.ProposedEmail)); err != nil {
		return request, mapMTLConflict("failed to upsert MTL registration", err)
	}

	query = p.db.Rebind(selectMTLRegistration + ` WHERE provider = ? AND provider_user_id = ?`)
	if err = p.db.GetContext(ctx, &request, query, candidate.Provider, candidate.ProviderUserID); err != nil {
		return request, fmt.Errorf("failed to load upserted MTL registration: %w", err)
	}
	return request, nil
}

// LoadMTLRegistration loads a registration request by its public CLI identifier.
func (p *SQLProvider) LoadMTLRegistration(ctx context.Context, id int64) (request model.MTLRegistrationRequest, found bool, err error) {
	query := p.db.Rebind(selectMTLRegistration + ` WHERE id = ?`)
	if err = p.db.GetContext(ctx, &request, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return request, false, nil
		}
		return request, false, fmt.Errorf("failed to load MTL registration: %w", err)
	}
	return request, true, nil
}

// ListMTLRegistrations lists requests in deterministic request order, optionally filtered by status.
func (p *SQLProvider) ListMTLRegistrations(ctx context.Context, status model.MTLRegistrationStatus) (requests []model.MTLRegistrationRequest, err error) {
	query := selectMTLRegistration
	args := []any{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY requested_at, id`
	if err = p.db.SelectContext(ctx, &requests, p.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("failed to list MTL registrations: %w", err)
	}
	return requests, nil
}

// RejectMTLRegistration rejects the exact pending request version.
func (p *SQLProvider) RejectMTLRegistration(ctx context.Context, id int64, expectedVersion int, actorUsername string) (request model.MTLRegistrationRequest, err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return request, fmt.Errorf("failed to begin MTL registration rejection: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = tx.GetContext(ctx, &request, tx.Rebind(selectMTLRegistration+` WHERE id = ?`), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return request, ErrMTLRegistrationNotFound
		}
		return request, fmt.Errorf("failed to load MTL registration for rejection: %w", err)
	}
	if request.Version != expectedVersion {
		return request, ErrMTLVersionConflict
	}
	if request.Status != model.MTLRegistrationStatusPending {
		return request, ErrMTLRegistrationTerminal
	}

	actorID, err := loadOptionalMTLActor(ctx, tx, actorUsername)
	if err != nil {
		return request, err
	}
	result, err := tx.ExecContext(ctx, tx.Rebind(`UPDATE mtl_registration_requests SET status = 'rejected', version = version + 1, updated_at = CURRENT_TIMESTAMP, resolved_at = CURRENT_TIMESTAMP, resolved_by_user_id = ? WHERE id = ? AND version = ? AND status = 'pending'`), actorID, id, expectedVersion)
	if err != nil {
		return request, fmt.Errorf("failed to reject MTL registration: %w", err)
	}
	if err = requireOneMTLRegistrationRow(result); err != nil {
		return request, err
	}
	if _, err = tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_audit_events (actor_user_id, event_type, target_type, target_id) VALUES (?, 'registration.rejected', 'registration', ?)`), actorID, fmt.Sprint(id)); err != nil {
		return request, fmt.Errorf("failed to audit MTL registration rejection: %w", err)
	}
	if err = tx.GetContext(ctx, &request, tx.Rebind(selectMTLRegistration+` WHERE id = ?`), id); err != nil {
		return request, fmt.Errorf("failed to reload rejected MTL registration: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return request, fmt.Errorf("failed to commit MTL registration rejection: %w", err)
	}
	return request, nil
}

// ApproveMTLRegistration atomically creates a passwordless local user and links the Telegram identity.
func (p *SQLProvider) ApproveMTLRegistration(ctx context.Context, approval model.MTLRegistrationApproval) (username string, err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to begin MTL registration approval: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var request model.MTLRegistrationRequest
	if err = tx.GetContext(ctx, &request, tx.Rebind(selectMTLRegistration+` WHERE id = ?`), approval.RequestID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrMTLRegistrationNotFound
		}
		return "", fmt.Errorf("failed to load MTL registration for approval: %w", err)
	}
	if request.Version != approval.ExpectedVersion {
		return "", ErrMTLVersionConflict
	}
	if request.Status != model.MTLRegistrationStatusPending {
		return "", ErrMTLRegistrationTerminal
	}

	username = strings.TrimSpace(approval.Username)
	if username == "" && request.ProposedUsername.Valid {
		username = strings.TrimSpace(request.ProposedUsername.String)
	}
	email := strings.TrimSpace(approval.Email)
	if email == "" && request.ProposedEmail.Valid {
		email = strings.TrimSpace(request.ProposedEmail.String)
	}
	if username == "" || email == "" {
		return "", ErrMTLRegistrationIncomplete
	}
	displayName := strings.TrimSpace(approval.DisplayName)
	if displayName == "" && request.DisplayName.Valid {
		displayName = strings.TrimSpace(request.DisplayName.String)
	}
	if displayName == "" {
		displayName = username
	}

	actorID, err := loadOptionalMTLActor(ctx, tx, approval.ActorUsername)
	if err != nil {
		return "", err
	}
	result, err := tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_users (username, display_name, status, password_hash) VALUES (?, ?, 'active', NULL)`), username, displayName)
	if err != nil {
		return "", mapMTLConflict("failed to create approved MTL user", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		return "", fmt.Errorf("failed to obtain approved MTL user ID: %w", err)
	}
	if _, err = tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_user_emails (user_id, email, is_primary, verified) VALUES (?, ?, 1, 1)`), userID, email); err != nil {
		return "", mapMTLConflict("failed to create approved MTL user email", err)
	}
	if _, err = tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_user_identities (user_id, provider, provider_user_id, provider_username) VALUES (?, ?, ?, ?)`), userID, request.Provider, request.ProviderUserID, request.ProviderUsername); err != nil {
		return "", mapMTLConflict("failed to link approved MTL user identity", err)
	}
	groups := make(map[string]struct{}, len(approval.Groups))
	for _, rawGroup := range approval.Groups {
		group := strings.TrimSpace(rawGroup)
		if _, exists := groups[group]; exists {
			continue
		}
		groups[group] = struct{}{}
		result, execErr := tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_group_memberships (user_id, group_id) SELECT ?, id FROM mtl_groups WHERE name = ?`), userID, group)
		if execErr != nil {
			return "", mapMTLConflict("failed to add approved MTL user group", execErr)
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return "", fmt.Errorf("failed to check approved MTL user group: %w", rowsErr)
		}
		if rows != 1 {
			return "", ErrMTLGroupNotFound
		}
	}
	result, err = tx.ExecContext(ctx, tx.Rebind(`UPDATE mtl_registration_requests SET status = 'approved', version = version + 1, updated_at = CURRENT_TIMESTAMP, resolved_at = CURRENT_TIMESTAMP, resolved_by_user_id = ?, approved_user_id = ? WHERE id = ? AND version = ? AND status = 'pending'`), actorID, userID, request.ID, approval.ExpectedVersion)
	if err != nil {
		return "", fmt.Errorf("failed to resolve approved MTL registration: %w", err)
	}
	if err = requireOneMTLRegistrationRow(result); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_audit_events (actor_user_id, event_type, target_type, target_id) VALUES (?, 'user.created', 'user', ?)`), actorID, username); err != nil {
		return "", fmt.Errorf("failed to audit approved MTL user creation: %w", err)
	}
	if _, err = tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_audit_events (actor_user_id, event_type, target_type, target_id) VALUES (?, 'registration.approved', 'registration', ?)`), actorID, fmt.Sprint(request.ID)); err != nil {
		return "", fmt.Errorf("failed to audit MTL registration approval: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit MTL registration approval: %w", err)
	}
	return username, nil
}

type mtlRegistrationQuerier interface {
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	Rebind(query string) string
}

func loadOptionalMTLActor(ctx context.Context, q mtlRegistrationQuerier, username string) (any, error) {
	if strings.TrimSpace(username) == "" {
		return nil, nil
	}
	var id int64
	if err := q.GetContext(ctx, &id, q.Rebind(`SELECT id FROM mtl_users WHERE username = ?`), username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMTLUserNotFound
		}
		return nil, fmt.Errorf("failed to load MTL registration actor: %w", err)
	}
	return id, nil
}

func requireOneMTLRegistrationRow(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check MTL registration mutation: %w", err)
	}
	if rows != 1 {
		return ErrMTLVersionConflict
	}
	return nil
}

func nullableMTLString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
