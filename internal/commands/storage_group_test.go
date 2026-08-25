package commands

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/storage"
)

func TestStorageGroupCommandHelpListsRecoveryWorkflow(t *testing.T) {
	cmd := newStorageGroupCmd(NewCmdCtx())
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	require.NoError(t, cmd.Execute())
	for _, name := range []string{"list", "show", "create", "rename", "delete", "add-user", "remove-user"} {
		assert.Contains(t, buf.String(), name)
	}
}

func TestRunStorageGroupBootstrapAndRecoveryLifecycle(t *testing.T) {
	ctx := context.Background()
	config := &schema.Configuration{Storage: schema.Storage{Local: &schema.StorageLocal{Path: filepath.Join(t.TempDir(), "db.sqlite3")}}}
	store := storage.NewSQLiteProvider(config)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.MigrateMTL(ctx))
	_, err := store.CreateMTLAdminUser(ctx, model.MTLAdminUserCreate{Username: "admin", Email: "admin@example.com"}, "")
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	group, err := runStorageGroupCreate(ctx, buf, store, "admins", "admin")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Created group admins")

	buf.Reset()
	group, err = runStorageGroupAddUser(ctx, buf, store, "admins", "admin", group.Version, "admin")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Added user admin")

	buf.Reset()
	require.NoError(t, runStorageGroupList(ctx, buf, store))
	assert.Contains(t, buf.String(), "admins")
	assert.Contains(t, buf.String(), "1")

	buf.Reset()
	require.NoError(t, runStorageGroupShow(ctx, buf, store, "admins"))
	assert.Contains(t, buf.String(), "Users: admin")

	buf.Reset()
	group, affected, err := runStorageGroupRename(ctx, buf, store, "admins", "admins-recovery", group.Version, "admin")
	require.NoError(t, err)
	assert.Equal(t, []string{"admin"}, affected)
	assert.Contains(t, buf.String(), "External YAML ACL references are not updated")

	buf.Reset()
	group, err = runStorageGroupRemoveUser(ctx, buf, store, group.Name, "admin", group.Version, "admin")
	require.NoError(t, err)
	assert.Empty(t, group.Users)

	buf.Reset()
	affected, err = runStorageGroupDelete(ctx, buf, store, group.Name, group.Version, "admin")
	require.NoError(t, err)
	assert.Empty(t, affected)
}
