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
	provider := NewSQLUserProvider(&schema.AuthenticationBackendSQL{Password: schema.DefaultPasswordConfig}, store, nil)
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

func TestSQLUserProviderStartupReconcilesEnabledApplicationGroups(t *testing.T) {
	disabled := false
	store := &testSQLUserStore{users: map[string]model.MTLUserDetails{}}
	provider := NewSQLUserProvider(
		&schema.AuthenticationBackendSQL{Password: schema.DefaultPasswordConfig},
		store,
		[]schema.Application{
			{Slug: "grafana"},
			{Slug: "shared-one", Group: "shared"},
			{Slug: "shared-two", Group: "shared"},
			{Slug: "disabled", Enabled: &disabled},
		},
	)

	require.NoError(t, provider.StartupCheck())
	assert.Equal(t, []string{"app:grafana", "shared"}, store.reconciledGroups)
}

func TestSQLUserProviderUpdateAndChangePassword(t *testing.T) {
	store := &testSQLUserStore{users: map[string]model.MTLUserDetails{
		"active":   {User: model.MTLUser{ID: 1, Username: "active", Status: model.MTLUserStatusActive, PasswordHash: sql.NullString{String: "$plaintext$old-password", Valid: true}, Version: 4, SessionEpoch: 2}, PrimaryEmail: "active@eurmtl.me"},
		"telegram": {User: model.MTLUser{ID: 2, Username: "telegram", Status: model.MTLUserStatusActive, Version: 1, SessionEpoch: 4}, PrimaryEmail: "telegram@eurmtl.me"},
	}}
	provider := NewSQLUserProvider(&schema.AuthenticationBackendSQL{Password: schema.DefaultPasswordConfig}, store, nil)
	require.NoError(t, provider.StartupCheck())

	assert.ErrorIs(t, provider.ChangePassword("active", "wrong", "new-password"), ErrIncorrectPassword)
	assert.ErrorIs(t, provider.ChangePassword("active", "old-password", "old-password"), ErrPasswordWeak)
	require.NoError(t, provider.ChangePassword("active", "old-password", "new-password"))

	updated := store.users["active"]
	assert.Equal(t, 5, updated.User.Version)
	assert.Equal(t, 3, updated.User.SessionEpoch)
	digest, err := schema.DecodePasswordDigest(updated.User.PasswordHash.String)
	require.NoError(t, err)
	valid, err := digest.MatchAdvanced("new-password")
	require.NoError(t, err)
	assert.True(t, valid)

	require.NoError(t, provider.UpdatePassword("active", "another-password"))
	assert.Equal(t, 6, store.users["active"].User.Version)
	removed, err := provider.RemovePassword("active", "another-password", 6)
	require.NoError(t, err)
	assert.Equal(t, 4, *removed.SessionEpoch)
	assert.False(t, store.users["active"].User.PasswordHash.Valid)

	proofDetails, err := provider.SetPasswordFromProof("telegram", "first-password")
	require.NoError(t, err)
	assert.Equal(t, 5, *proofDetails.SessionEpoch)
	assert.Error(t, func() error { _, err := provider.SetPasswordFromProof("telegram", "second-password"); return err }())
}

type testSQLUserStore struct {
	users            map[string]model.MTLUserDetails
	reconciledGroups []string
}

func (s *testSQLUserStore) RemoveMTLSelfServicePassword(_ context.Context, username string, expectedVersion int, actor string) (model.MTLAdminUserDetails, error) {
	details, ok := s.users[username]
	if !ok || details.User.Version != expectedVersion || actor != username {
		return model.MTLAdminUserDetails{}, assert.AnError
	}
	details.User.PasswordHash = sql.NullString{}
	details.User.Version++
	details.User.SessionEpoch++
	s.users[username] = details
	return model.MTLAdminUserDetails{MTLAdminUserSummary: model.MTLAdminUserSummary{Username: username, Version: details.User.Version}, SessionEpoch: details.User.SessionEpoch}, nil
}

func (s *testSQLUserStore) ReconcileMTLGroups(_ context.Context, groups []string) error {
	s.reconciledGroups = append([]string(nil), groups...)
	return nil
}

func (s *testSQLUserStore) MigrateMTL(context.Context) error {
	return nil
}

func (s *testSQLUserStore) LoadMTLUser(_ context.Context, username string) (model.MTLUserDetails, bool, error) {
	details, found := s.users[username]
	return details, found, nil
}

func (s *testSQLUserStore) FindMTLUserByEmail(_ context.Context, email string) (string, bool, error) {
	for username, details := range s.users {
		if details.PrimaryEmail == email {
			return username, true, nil
		}
	}

	return "", false, nil
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

func (s *testSQLUserStore) SetMTLSelfServicePassword(_ context.Context, username, passwordHash string, expectedVersion int, actor string) (model.MTLAdminUserDetails, error) {
	details, ok := s.users[username]
	if !ok || details.User.Version != expectedVersion || actor != username {
		return model.MTLAdminUserDetails{}, assert.AnError
	}
	details.User.PasswordHash = sql.NullString{String: passwordHash, Valid: true}
	details.User.Version++
	details.User.SessionEpoch++
	s.users[username] = details
	return model.MTLAdminUserDetails{
		MTLAdminUserSummary: model.MTLAdminUserSummary{Username: username, Version: details.User.Version, PasswordEnabled: true},
		SessionEpoch:        details.User.SessionEpoch,
	}, nil
}
