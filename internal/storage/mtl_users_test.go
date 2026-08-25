package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/model"
)

func TestMTLUserStoreLoad(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := context.Background()
	hash := "$argon2id$v=19$m=65536,t=3,p=4$hash"

	require.NoError(t, provider.ImportMTLUsers(ctx, []model.MTLUserImport{
		{
			Username:     "bublik",
			DisplayName:  "Bublik",
			PasswordHash: &hash,
			Emails: []model.MTLUserImportEmail{
				{Email: "bublik@eurmtl.me", Primary: true, Verified: true},
				{Email: "elsewhere@example.com"},
			},
			Groups: []string{"admins", "app:grafana"},
		},
		{Username: "telegram", DisplayName: "Telegram Only", Emails: []model.MTLUserImportEmail{{Email: "telegram@eurmtl.me", Primary: true}}},
	}))

	details, found, err := provider.LoadMTLUser(ctx, "BUBLIK")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "bublik", details.User.Username)
	assert.Equal(t, model.MTLUserStatusActive, details.User.Status)
	assert.True(t, details.User.PasswordHash.Valid)
	assert.Equal(t, hash, details.User.PasswordHash.String)
	assert.Equal(t, "bublik@eurmtl.me", details.PrimaryEmail)
	assert.Equal(t, []string{"admins", "app:grafana"}, details.Groups)

	telegram, found, err := provider.LoadMTLUser(ctx, "telegram")
	require.NoError(t, err)
	assert.True(t, found)
	assert.False(t, telegram.User.PasswordHash.Valid)

	_, found, err = provider.LoadMTLUser(ctx, "missing")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestMTLUserStoreStatusAndPasswordVersion(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := context.Background()
	require.NoError(t, provider.ImportMTLUsers(ctx, []model.MTLUserImport{{
		Username: "disabled", DisplayName: "Disabled", Emails: []model.MTLUserImportEmail{{Email: "disabled@eurmtl.me", Primary: true}},
	}}))
	_, err := provider.db.Exec(`UPDATE mtl_users SET status = 'disabled' WHERE username = 'disabled'`)
	require.NoError(t, err)

	details, found, err := provider.LoadMTLUser(ctx, "disabled")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, model.MTLUserStatusDisabled, details.User.Status)

	hash := "new-hash"
	require.NoError(t, provider.UpdateMTLUserPassword(ctx, details.User.ID, &hash, details.User.Version))

	updated, found, err := provider.LoadMTLUser(ctx, "disabled")
	require.NoError(t, err)
	assert.True(t, found)
	assert.True(t, updated.User.PasswordHash.Valid)
	assert.Equal(t, hash, updated.User.PasswordHash.String)
	assert.Equal(t, details.User.Version+1, updated.User.Version)

	err = provider.UpdateMTLUserPassword(ctx, details.User.ID, nil, details.User.Version)
	assert.ErrorIs(t, err, ErrMTLVersionConflict)
}

func TestMTLUserStoreImportIsAtomicAndMapsConflicts(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := context.Background()
	users := []model.MTLUserImport{
		{Username: "first", DisplayName: "First", Emails: []model.MTLUserImportEmail{{Email: "same@example.com", Primary: true}}},
		{Username: "second", DisplayName: "Second", Emails: []model.MTLUserImportEmail{{Email: "same@example.com", Primary: true}}},
	}

	err := provider.ImportMTLUsers(ctx, users)
	assert.ErrorIs(t, err, ErrMTLConflict)

	var count int
	require.NoError(t, provider.db.Get(&count, `SELECT COUNT(*) FROM mtl_users`))
	assert.Zero(t, count)
}

func TestMTLUserStoreRejectsMissingPrimaryEmail(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	err := provider.ImportMTLUsers(context.Background(), []model.MTLUserImport{{
		Username: "no-primary", DisplayName: "No Primary", Emails: []model.MTLUserImportEmail{{Email: "mail@example.com"}},
	}})
	assert.True(t, errors.Is(err, ErrMTLPrimaryEmailRequired))
}

func TestMTLUserIdentityLifecycle(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := context.Background()
	require.NoError(t, provider.ImportMTLUsers(ctx, []model.MTLUserImport{
		{Username: "bublik", DisplayName: "Bublik", Emails: []model.MTLUserImportEmail{{Email: "bublik@eurmtl.me", Primary: true}}},
		{Username: "other", DisplayName: "Other", Emails: []model.MTLUserImportEmail{{Email: "other@eurmtl.me", Primary: true}}},
	}))

	require.NoError(t, provider.LinkMTLUserIdentity(ctx, "bublik", "telegram", "987654321", "bublik_tg"))
	identity, found, err := provider.LoadMTLUserIdentity(ctx, "bublik", "telegram")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "987654321", identity.ProviderUserID)
	assert.Equal(t, "bublik_tg", *identity.ProviderUsername)

	details, found, err := provider.LoadMTLUserByIdentity(ctx, "telegram", "987654321")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "bublik", details.User.Username)

	err = provider.LinkMTLUserIdentity(ctx, "other", "telegram", "987654321", "other_tg")
	assert.ErrorIs(t, err, ErrMTLConflict)
	err = provider.LinkMTLUserIdentity(ctx, "missing", "telegram", "1", "missing")
	assert.ErrorIs(t, err, ErrMTLUserNotFound)

	require.NoError(t, provider.UnlinkMTLUserIdentity(ctx, "bublik", "telegram"))
	_, found, err = provider.LoadMTLUserByIdentity(ctx, "telegram", "987654321")
	require.NoError(t, err)
	assert.False(t, found)
	assert.ErrorIs(t, provider.UnlinkMTLUserIdentity(ctx, "bublik", "telegram"), ErrMTLIdentityNotFound)

	var auditCount int
	require.NoError(t, provider.db.Get(&auditCount, `SELECT COUNT(*) FROM mtl_audit_events WHERE event_type IN ('identity.linked', 'identity.unlinked')`))
	assert.Equal(t, 2, auditCount)
}

func newTestMTLUserProvider(t *testing.T) *SQLiteProvider {
	t.Helper()
	provider := newTestSQLiteProvider(t)
	t.Cleanup(func() { require.NoError(t, provider.Close()) })
	require.NoError(t, provider.MigrateMTL(context.Background()))

	return provider
}
