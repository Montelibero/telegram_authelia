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

func TestMTLSelfServicePasswordLifecycle(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := context.Background()
	oldHash := "old-hash"
	require.NoError(t, provider.ImportMTLUsers(ctx, []model.MTLUserImport{
		{Username: "bublik", DisplayName: "Bublik", PasswordHash: &oldHash, Emails: []model.MTLUserImportEmail{{Email: "bublik@eurmtl.me", Primary: true}}, Groups: []string{"admins"}},
		{Username: "backup", DisplayName: "Backup", PasswordHash: &oldHash, Emails: []model.MTLUserImportEmail{{Email: "backup@eurmtl.me", Primary: true}}, Groups: []string{"admins"}},
	}))
	require.NoError(t, provider.LinkMTLUserIdentity(ctx, "bublik", "telegram", "42", "bublik"))

	details, found, err := provider.LoadMTLUser(ctx, "bublik")
	require.NoError(t, err)
	require.True(t, found)

	newHash := "new-hash"
	changed, err := provider.SetMTLSelfServicePassword(ctx, "bublik", newHash, details.User.Version, "bublik")
	require.NoError(t, err)
	assert.True(t, changed.PasswordEnabled)
	assert.Equal(t, details.User.Version+1, changed.Version)
	assert.Equal(t, details.User.SessionEpoch+1, changed.SessionEpoch)

	removed, err := provider.RemoveMTLSelfServicePassword(ctx, "bublik", changed.Version, "bublik")
	require.NoError(t, err)
	assert.False(t, removed.PasswordEnabled)
	assert.Equal(t, changed.Version+1, removed.Version)
	assert.Equal(t, changed.SessionEpoch+1, removed.SessionEpoch)

	stored, found, err := provider.LoadMTLUser(ctx, "bublik")
	require.NoError(t, err)
	require.True(t, found)
	assert.False(t, stored.User.PasswordHash.Valid)

	var setAudit, removeAudit int
	require.NoError(t, provider.db.Get(&setAudit, `SELECT COUNT(*) FROM mtl_audit_events WHERE event_type = 'password.set' AND target_id = 'bublik'`))
	require.NoError(t, provider.db.Get(&removeAudit, `SELECT COUNT(*) FROM mtl_audit_events WHERE event_type = 'password.removed' AND target_id = 'bublik'`))
	assert.Equal(t, 1, setAudit)
	assert.Equal(t, 1, removeAudit)
}

func TestMTLSelfServicePasswordRemovalGuards(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := context.Background()
	hash := "hash"
	require.NoError(t, provider.ImportMTLUsers(ctx, []model.MTLUserImport{
		{Username: "last-admin", DisplayName: "Last Admin", PasswordHash: &hash, Emails: []model.MTLUserImportEmail{{Email: "admin@eurmtl.me", Primary: true}}, Groups: []string{"admins"}},
		{Username: "plain", DisplayName: "Plain", PasswordHash: &hash, Emails: []model.MTLUserImportEmail{{Email: "plain@eurmtl.me", Primary: true}}},
	}))
	require.NoError(t, provider.LinkMTLUserIdentity(ctx, "last-admin", "telegram", "1", "admin"))

	admin, found, err := provider.LoadMTLUser(ctx, "last-admin")
	require.NoError(t, err)
	require.True(t, found)
	_, err = provider.RemoveMTLSelfServicePassword(ctx, "last-admin", admin.User.Version, "last-admin")
	assert.ErrorIs(t, err, ErrMTLLastPasswordAdmin)

	plain, found, err := provider.LoadMTLUser(ctx, "plain")
	require.NoError(t, err)
	require.True(t, found)
	_, err = provider.RemoveMTLSelfServicePassword(ctx, "plain", plain.User.Version, "plain")
	assert.ErrorIs(t, err, ErrMTLTelegramIdentityRequired)

	_, err = provider.SetMTLSelfServicePassword(ctx, "plain", "new", plain.User.Version+1, "plain")
	assert.ErrorIs(t, err, ErrMTLVersionConflict)
}

func newTestMTLUserProvider(t *testing.T) *SQLiteProvider {
	t.Helper()
	provider := newTestSQLiteProvider(t)
	t.Cleanup(func() { require.NoError(t, provider.Close()) })
	require.NoError(t, provider.MigrateMTL(context.Background()))

	return provider
}
