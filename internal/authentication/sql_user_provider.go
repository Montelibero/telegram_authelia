package authentication

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-crypt/crypt/algorithm"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/model"
)

// SQLUserProvider authenticates overlay users through the existing storage provider.
type SQLUserProvider struct {
	config *schema.AuthenticationBackendSQL
	store  SQLUserStore
	hash   algorithm.Hash
}

// NewSQLUserProvider constructs a SQL user provider without taking ownership of its store.
func NewSQLUserProvider(config *schema.AuthenticationBackendSQL, store SQLUserStore) *SQLUserProvider {
	return &SQLUserProvider{config: config, store: store}
}

// StartupCheck initializes password hashing for updates.
func (p *SQLUserProvider) StartupCheck() (err error) {
	if p.store == nil {
		return fmt.Errorf("SQL user store is not configured")
	}

	if err = p.store.MigrateMTL(context.Background()); err != nil {
		return fmt.Errorf("failed to migrate SQL user store: %w", err)
	}

	p.hash, err = NewFileCryptoHashFromConfig(p.config.Password)
	return err
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

	digest, err := p.hash.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrOperationFailed, err)
	}

	encoded := digest.Encode()
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

	valid, err := p.CheckUserPassword(username, oldPassword)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return ErrUserNotFound
		}

		return ErrAuthenticationFailed
	}

	if !valid {
		return ErrIncorrectPassword
	}

	return p.UpdatePassword(username, newPassword)
}

// Close is a no-op because the storage provider owns the database connection.
func (p *SQLUserProvider) Close() error {
	return nil
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
		Username:    d.User.Username,
		DisplayName: d.User.DisplayName,
		Emails:      []string{d.PrimaryEmail},
		Groups:      append([]string(nil), d.Groups...),
	}
}
