package authentication

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/model"
)

func TestSQLUserProviderContract(t *testing.T) {
	password := "$plaintext$password"
	store := &testSQLUserStore{users: map[string]model.MTLUserDetails{
		"active": {
			User:         model.MTLUser{ID: 1, Username: "active", DisplayName: "Active User", Status: model.MTLUserStatusActive, PasswordHash: sql.NullString{String: password, Valid: true}, Version: 1},
			PrimaryEmail: "active@eurmtl.me", Groups: []string{"admins", "app:grafana"},
		},
		"disabled": {User: model.MTLUser{ID: 2, Username: "disabled", Status: model.MTLUserStatusDisabled, PasswordHash: sql.NullString{String: password, Valid: true}}},
		"telegram": {User: model.MTLUser{ID: 3, Username: "telegram", Status: model.MTLUserStatusActive}},
	}}
	provider := NewSQLUserProvider(&schema.AuthenticationBackendSQL{Password: schema.DefaultPasswordConfig}, store)
	require.NoError(t, provider.StartupCheck())

	valid, err := provider.CheckUserPassword("active", "password")
	require.NoError(t, err)
	assert.True(t, valid)
	valid, err = provider.CheckUserPassword("active", "wrong")
	require.NoError(t, err)
	assert.False(t, valid)

	details, err := provider.GetDetails("active")
	require.NoError(t, err)
	assert.Equal(t, "Active User", details.DisplayName)
	assert.Equal(t, []string{"active@eurmtl.me"}, details.Emails)
	assert.Equal(t, []string{"admins", "app:grafana"}, details.Groups)

	extended, err := provider.GetDetailsExtended("active")
	require.NoError(t, err)
	assert.Equal(t, details, extended.UserDetails)

	for _, username := range []string{"disabled", "telegram", "missing"} {
		_, err = provider.CheckUserPassword(username, "password")
		assert.ErrorIs(t, err, ErrUserNotFound)
	}

	for _, username := range []string{"disabled", "missing"} {
		_, err = provider.GetDetails(username)
		assert.ErrorIs(t, err, ErrUserNotFound)
	}
	_, err = provider.GetDetails("telegram")
	require.NoError(t, err)

	require.NoError(t, provider.Close())
}

func TestSQLUserProviderUpdateAndChangePassword(t *testing.T) {
	store := &testSQLUserStore{users: map[string]model.MTLUserDetails{
		"active": {User: model.MTLUser{ID: 1, Username: "active", Status: model.MTLUserStatusActive, PasswordHash: sql.NullString{String: "$plaintext$old-password", Valid: true}, Version: 4}, PrimaryEmail: "active@eurmtl.me"},
	}}
	provider := NewSQLUserProvider(&schema.AuthenticationBackendSQL{Password: schema.DefaultPasswordConfig}, store)
	require.NoError(t, provider.StartupCheck())

	assert.ErrorIs(t, provider.ChangePassword("active", "wrong", "new-password"), ErrIncorrectPassword)
	assert.ErrorIs(t, provider.ChangePassword("active", "old-password", "old-password"), ErrPasswordWeak)
	require.NoError(t, provider.ChangePassword("active", "old-password", "new-password"))

	updated := store.users["active"]
	assert.Equal(t, 5, updated.User.Version)
	digest, err := schema.DecodePasswordDigest(updated.User.PasswordHash.String)
	require.NoError(t, err)
	valid, err := digest.MatchAdvanced("new-password")
	require.NoError(t, err)
	assert.True(t, valid)

	require.NoError(t, provider.UpdatePassword("active", "another-password"))
	assert.Equal(t, 6, store.users["active"].User.Version)
}

type testSQLUserStore struct {
	users map[string]model.MTLUserDetails
}

func (s *testSQLUserStore) MigrateMTL(context.Context) error {
	return nil
}

func (s *testSQLUserStore) LoadMTLUser(_ context.Context, username string) (model.MTLUserDetails, bool, error) {
	details, found := s.users[username]
	return details, found, nil
}

func (s *testSQLUserStore) UpdateMTLUserPassword(_ context.Context, userID int64, passwordHash *string, expectedVersion int) error {
	for username, details := range s.users {
		if details.User.ID == userID && details.User.Version == expectedVersion {
			if passwordHash == nil {
				details.User.PasswordHash = sql.NullString{}
			} else {
				details.User.PasswordHash = sql.NullString{String: *passwordHash, Valid: true}
			}
			details.User.Version++
			s.users[username] = details
			return nil
		}
	}

	return assert.AnError
}
