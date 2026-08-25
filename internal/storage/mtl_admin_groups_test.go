package storage

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/model"
)

func TestMTLAdminGroupLifecycle(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := context.Background()

	_, err := provider.CreateMTLAdminUser(ctx, model.MTLAdminUserCreate{Username: "admin", Email: "admin@example.com"}, "")
	require.NoError(t, err)
	_, err = provider.CreateMTLAdminUser(ctx, model.MTLAdminUserCreate{Username: "bublik", Email: "bublik@example.com"}, "admin")
	require.NoError(t, err)

	group, err := provider.CreateMTLAdminGroup(ctx, "team / weird:*", "admin")
	require.NoError(t, err)
	assert.Equal(t, "team / weird:*", group.Name)
	assert.Empty(t, group.Users)

	groups, err := provider.ListMTLAdminGroups(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, group.Name, groups[0].Name)

	group, err = provider.AddMTLAdminGroupUser(ctx, group.Name, "bublik", group.Version, "admin")
	require.NoError(t, err)
	assert.Equal(t, []string{"bublik"}, group.Users)
	user, found, err := provider.LoadMTLAdminUser(ctx, "bublik")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 1, user.SessionEpoch)

	group, affected, err := provider.RenameMTLAdminGroup(ctx, group.Name, "app:grafana", group.Version, "admin")
	require.NoError(t, err)
	assert.Equal(t, []string{"bublik"}, affected)
	assert.Equal(t, "app:grafana", group.Name)
	user, _, err = provider.LoadMTLAdminUser(ctx, "bublik")
	require.NoError(t, err)
	assert.Equal(t, 2, user.SessionEpoch)

	loaded, found, err := provider.LoadMTLAdminGroup(ctx, "app:grafana")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, group, loaded)

	group, err = provider.RemoveMTLAdminGroupUser(ctx, group.Name, "bublik", group.Version, "admin")
	require.NoError(t, err)
	assert.Empty(t, group.Users)
	user, _, err = provider.LoadMTLAdminUser(ctx, "bublik")
	require.NoError(t, err)
	assert.Equal(t, 3, user.SessionEpoch)

	group, err = provider.AddMTLAdminGroupUser(ctx, group.Name, "bublik", group.Version, "admin")
	require.NoError(t, err)
	user, _, err = provider.LoadMTLAdminUser(ctx, "bublik")
	require.NoError(t, err)
	assert.Equal(t, 4, user.SessionEpoch)
	affected, err = provider.DeleteMTLAdminGroup(ctx, group.Name, group.Version, "admin")
	require.NoError(t, err)
	assert.Equal(t, []string{"bublik"}, affected)
	user, _, err = provider.LoadMTLAdminUser(ctx, "bublik")
	require.NoError(t, err)
	assert.Equal(t, 5, user.SessionEpoch)
	_, found, err = provider.LoadMTLAdminGroup(ctx, group.Name)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestMTLAdminGroupConflictsAndAudit(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := context.Background()

	_, err := provider.CreateMTLAdminUser(ctx, model.MTLAdminUserCreate{Username: "admin", Email: "admin@example.com"}, "")
	require.NoError(t, err)
	group, err := provider.CreateMTLAdminGroup(ctx, "admins", "admin")
	require.NoError(t, err)
	_, err = provider.CreateMTLAdminGroup(ctx, "admins", "admin")
	assert.ErrorIs(t, err, ErrMTLConflict)

	_, err = provider.AddMTLAdminGroupUser(ctx, "admins", "missing", group.Version, "admin")
	assert.ErrorIs(t, err, ErrMTLUserNotFound)
	_, err = provider.RemoveMTLAdminGroupUser(ctx, "admins", "admin", group.Version+1, "admin")
	assert.ErrorIs(t, err, ErrMTLVersionConflict)

	var events int
	require.NoError(t, provider.db.Get(&events, `SELECT COUNT(*) FROM mtl_audit_events WHERE actor_user_id = (SELECT id FROM mtl_users WHERE username = 'admin') AND target_type = 'group'`))
	assert.Equal(t, 1, events)
}

func TestMTLAdminGroupMembershipFailuresRollBack(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := t.Context()
	_, err := provider.CreateMTLAdminUser(ctx, model.MTLAdminUserCreate{Username: "admin", Email: "admin@example.com"}, "")
	require.NoError(t, err)
	group, err := provider.CreateMTLAdminGroup(ctx, "app:grafana", "admin")
	require.NoError(t, err)
	group, err = provider.AddMTLAdminGroupUser(ctx, group.Name, "admin", group.Version, "admin")
	require.NoError(t, err)

	_, err = provider.AddMTLAdminGroupUser(ctx, group.Name, "admin", group.Version, "admin")
	assert.ErrorIs(t, err, ErrMTLConflict)
	_, err = provider.RemoveMTLAdminGroupUser(ctx, group.Name, "admin", group.Version-1, "admin")
	assert.ErrorIs(t, err, ErrMTLVersionConflict)

	current, found, err := provider.LoadMTLAdminGroup(ctx, group.Name)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, group.Version, current.Version)
	assert.Equal(t, []string{"admin"}, current.Users)
	user, found, err := provider.LoadMTLAdminUser(ctx, "admin")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 1, user.SessionEpoch)
}

func TestReconcileMTLGroupsCreatesOnlyMissingGroups(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := context.Background()

	_, err := provider.CreateMTLAdminUser(ctx, model.MTLAdminUserCreate{Username: "bublik", Email: "bublik@example.com"}, "")
	require.NoError(t, err)
	group, err := provider.CreateMTLAdminGroup(ctx, "existing", "bublik")
	require.NoError(t, err)
	_, err = provider.AddMTLAdminGroupUser(ctx, group.Name, "bublik", group.Version, "bublik")
	require.NoError(t, err)

	require.NoError(t, provider.ReconcileMTLGroups(ctx, []string{"existing", "new", "new"}))
	require.NoError(t, provider.ReconcileMTLGroups(ctx, []string{"existing", "new"}))

	groups, err := provider.ListMTLAdminGroups(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	assert.Equal(t, "existing", groups[0].Name)
	assert.Equal(t, 1, groups[0].UserCount)
	assert.Equal(t, "new", groups[1].Name)
	assert.Zero(t, groups[1].UserCount)
}

func TestReconcileMTLGroupsQueriesOnlyIgnoreDuplicateNames(t *testing.T) {
	assert.Equal(t, `INSERT INTO mtl_groups (name) VALUES (?) ON CONFLICT(name) DO NOTHING`, reconcileMTLGroupQuery(providerSQLite))
	assert.Equal(t, `INSERT INTO mtl_groups (name) VALUES (?) ON CONFLICT(name) DO NOTHING`, reconcileMTLGroupQuery(providerPostgres))
	assert.Equal(t, `INSERT INTO mtl_groups (name) VALUES (?) ON DUPLICATE KEY UPDATE name = name`, reconcileMTLGroupQuery(providerMySQL))
	assert.NotContains(t, reconcileMTLGroupQuery(providerMySQL), "IGNORE")
}

func TestReconcileMTLGroupsIsSafeAcrossConcurrentProviders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite3")
	config := &schema.Configuration{Storage: schema.Storage{Local: &schema.StorageLocal{Path: path}}}
	providers := []*SQLiteProvider{NewSQLiteProvider(config), NewSQLiteProvider(config)}
	for _, provider := range providers {
		require.NoError(t, provider.MigrateMTL(t.Context()))
		t.Cleanup(func() { require.NoError(t, provider.Close()) })
	}

	start := make(chan struct{})
	errs := make(chan error, len(providers))
	var wait sync.WaitGroup
	for _, provider := range providers {
		wait.Add(1)
		go func(provider *SQLiteProvider) {
			defer wait.Done()
			<-start
			errs <- provider.ReconcileMTLGroups(t.Context(), []string{"app:grafana"})
		}(provider)
	}
	close(start)
	wait.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	groups, err := providers[0].ListMTLAdminGroups(t.Context())
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, "app:grafana", groups[0].Name)
}
