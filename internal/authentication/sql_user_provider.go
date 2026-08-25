package authentication

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-crypt/crypt/algorithm"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/model"
)

// SQLUserProvider authenticates overlay users through the existing storage provider.
type SQLUserProvider struct {
	config       *schema.AuthenticationBackendSQL
	applications []schema.Application
	store        SQLUserStore
	hash         algorithm.Hash
}

// NewSQLUserProvider constructs a SQL user provider without taking ownership of its store.
func NewSQLUserProvider(config *schema.AuthenticationBackendSQL, store SQLUserStore, applications []schema.Application) *SQLUserProvider {
	return &SQLUserProvider{config: config, applications: append([]schema.Application(nil), applications...), store: store}
}

// StartupCheck initializes password hashing for updates.
func (p *SQLUserProvider) StartupCheck() (err error) {
	if p.store == nil {
		return fmt.Errorf("SQL user store is not configured")
	}

	if err = p.store.MigrateMTL(context.Background()); err != nil {
		return fmt.Errorf("failed to migrate SQL user store: %w", err)
	}

	if err = p.store.ReconcileMTLGroups(context.Background(), applicationGroups(p.applications)); err != nil {
		return fmt.Errorf("failed to reconcile SQL user groups: %w", err)
	}

	p.hash, err = NewFileCryptoHashFromConfig(p.config.Password)
	return err
}

func applicationGroups(applications []schema.Application) []string {
	groups := make([]string, 0, len(applications))
	seen := make(map[string]struct{}, len(applications))

	for _, application := range applications {
		if !application.IsEnabled() {
			continue
		}

		group := application.Group
		if group == "" {
			group = "app:" + application.Slug
		}

		if _, ok := seen[group]; ok {
			continue
		}

		seen[group] = struct{}{}
		groups = append(groups, group)
	}

	return groups
}

// CheckUserPassword checks a stored Authelia password digest.
func (p *SQLUserProvider) CheckUserPassword(username, password string) (valid bool, err error) {
	details, err := p.loadActiveUser(username)
	if err != nil || !details.User.PasswordHash.Valid {
		if err != nil {
			return false, err
		}

		return false, ErrUserNotFound
	}

	digest, err := schema.DecodePasswordDigest(details.User.PasswordHash.String)
	if err != nil {
		return false, fmt.Errorf("failed to decode SQL user password digest: %w", err)
	}

	return digest.MatchAdvanced(password)
}

// GetDetails returns the standard Authelia user attributes.
func (p *SQLUserProvider) GetDetails(username string) (details *UserDetails, err error) {
	stored, err := p.loadActiveUser(username)
	if err != nil {
		return nil, err
	}

	return sqlUserDetails(stored), nil
}

// GetDetailsExtended returns the standard attributes with an empty extended profile.
func (p *SQLUserProvider) GetDetailsExtended(username string) (details *UserDetailsExtended, err error) {
	standard, err := p.GetDetails(username)
	if err != nil {
		return nil, err
	}

	return &UserDetailsExtended{UserDetails: standard}, nil
}

// UpdatePassword hashes and stores a new password using optimistic locking.
func (p *SQLUserProvider) UpdatePassword(username, newPassword string) (err error) {
	stored, err := p.loadActiveUser(username)
	if err != nil {
		return err
	}

	if strings.TrimSpace(newPassword) == "" {
		return ErrPasswordWeak
	}

	encoded, err := p.hashPassword(newPassword)
	if err != nil {
		return err
	}
	if err = p.store.UpdateMTLUserPassword(context.Background(), stored.User.ID, &encoded, stored.User.Version); err != nil {
		return fmt.Errorf("%w: %v", ErrOperationFailed, err)
	}

	return nil
}

// ChangePassword verifies the old password before storing a new one.
func (p *SQLUserProvider) ChangePassword(username, oldPassword, newPassword string) (err error) {
	if strings.TrimSpace(newPassword) == "" || oldPassword == newPassword {
		return ErrPasswordWeak
	}

	stored, err := p.loadActiveUser(username)
	if err != nil {
		return err
	}
	if !stored.User.PasswordHash.Valid {
		return ErrUserNotFound
	}
	digest, err := schema.DecodePasswordDigest(stored.User.PasswordHash.String)
	if err != nil {
		return ErrAuthenticationFailed
	}
	valid, err := digest.MatchAdvanced(oldPassword)
	if err != nil {
		return ErrAuthenticationFailed
	}

	if !valid {
		return ErrIncorrectPassword
	}

	encoded, err := p.hashPassword(newPassword)
	if err != nil {
		return err
	}
	if _, err = p.store.SetMTLSelfServicePassword(context.Background(), username, encoded, stored.User.Version, username); err != nil {
		return fmt.Errorf("%w: %v", ErrOperationFailed, err)
	}

	return nil
}

func (p *SQLUserProvider) hashPassword(password string) (string, error) {
	digest, err := p.hash.Hash(password)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrOperationFailed, err)
	}
	return digest.Encode(), nil
}

// SetPasswordFromProof sets the first password after an external identity proof.
func (p *SQLUserProvider) SetPasswordFromProof(username, newPassword, grantSignature string, consumedAt time.Time) (*UserDetails, error) {
	stored, err := p.loadActiveUser(username)
	if err != nil {
		return nil, err
	}
	if stored.User.PasswordHash.Valid || strings.TrimSpace(newPassword) == "" {
		return nil, ErrOperationFailed
	}
	encoded, err := p.hashPassword(newPassword)
	if err != nil {
		return nil, err
	}
	updated, err := p.store.SetMTLSelfServicePasswordWithTelegramGrant(context.Background(), username, encoded, stored.User.Version, username, grantSignature, consumedAt)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOperationFailed, err)
	}
	stored.User.Version = updated.Version
	stored.User.SessionEpoch = updated.SessionEpoch
	stored.User.PasswordHash.Valid = true
	return sqlUserDetails(stored), nil
}

// RemovePassword verifies the current password and disables password login.
func (p *SQLUserProvider) RemovePassword(username, currentPassword string, expectedVersion int) (*UserDetails, error) {
	valid, err := p.CheckUserPassword(username, currentPassword)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, ErrIncorrectPassword
	}
	stored, err := p.loadActiveUser(username)
	if err != nil {
		return nil, err
	}
	if stored.User.Version != expectedVersion {
		return nil, fmt.Errorf("%w: version conflict", ErrOperationFailed)
	}
	updated, err := p.store.RemoveMTLSelfServicePassword(context.Background(), username, expectedVersion, username)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOperationFailed, err)
	}
	stored.User.Version = updated.Version
	stored.User.SessionEpoch = updated.SessionEpoch
	stored.User.PasswordHash.Valid = false
	return sqlUserDetails(stored), nil
}

// Close is a no-op because the storage provider owns the database connection.
func (p *SQLUserProvider) Close() error {
	return nil
}

// IsMTLProvider identifies the SQL user backend for session epoch validation.
func (p *SQLUserProvider) IsMTLProvider() bool {
	return true
}

func (p *SQLUserProvider) loadActiveUser(username string) (details model.MTLUserDetails, err error) {
	details, found, err := p.store.LoadMTLUser(context.Background(), username)
	if err != nil {
		return details, err
	}

	if !found || details.User.Status != model.MTLUserStatusActive {
		return details, ErrUserNotFound
	}

	return details, nil
}

func sqlUserDetails(d model.MTLUserDetails) *UserDetails {
	return &UserDetails{
		Username:     d.User.Username,
		DisplayName:  d.User.DisplayName,
		Emails:       []string{d.PrimaryEmail},
		Groups:       append([]string(nil), d.Groups...),
		SessionEpoch: &d.User.SessionEpoch,
	}
}
