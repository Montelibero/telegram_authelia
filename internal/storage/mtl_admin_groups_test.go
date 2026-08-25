package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
