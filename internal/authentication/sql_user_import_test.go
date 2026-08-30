package authentication

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/storage"
)

func TestImportFileUsersDryRunAndIdempotency(t *testing.T) {
	ctx := context.Background()
	config := &schema.Configuration{Storage: schema.Storage{Local: &schema.StorageLocal{Path: filepath.Join(t.TempDir(), "db.sqlite3")}}}
	store := storage.NewSQLiteProvider(config)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	fixture := filepath.Join("testdata", "users_database_mtl.yml")

	report, err := ImportFileUsers(ctx, fixture, "eurmtl.me", store, true)
	require.NoError(t, err)
	assert.Equal(t, []string{"bublik", "disabled", "noemail"}, report.Created)

	require.NoError(t, store.SQLProvider.MigrateMTL(ctx))
	tables, err := store.SchemaTables(ctx)
	require.NoError(t, err)
	assert.Contains(t, tables, "mtl_users")
	_, found, err := store.LoadMTLUser(ctx, "bublik")
	require.NoError(t, err)
	assert.False(t, found)

	report, err = ImportFileUsers(ctx, fixture, "eurmtl.me", store, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"bublik", "disabled", "noemail"}, report.Created)

	bublik, found, err := store.LoadMTLUser(ctx, "bublik")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "bublik@example.com", bublik.PrimaryEmail)
	assert.Equal(t, []string{"admins", "app:grafana"}, bublik.Groups)
	assert.Equal(t, "$plaintext$password", bublik.User.PasswordHash.String)

	noemail, found, err := store.LoadMTLUser(ctx, "noemail")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "noemail@eurmtl.me", noemail.PrimaryEmail)

	disabled, found, err := store.LoadMTLUser(ctx, "disabled")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "disabled", disabled.User.Status)

	report, err = ImportFileUsers(ctx, fixture, "eurmtl.me", store, false)
	require.NoError(t, err)
	assert.Empty(t, report.Created)
	assert.Equal(t, []string{"bublik", "disabled", "noemail"}, report.Unchanged)
}

func TestImportFileUsersReportsConflictsBeforeMutation(t *testing.T) {
	ctx := context.Background()
	config := &schema.Configuration{Storage: schema.Storage{Local: &schema.StorageLocal{Path: filepath.Join(t.TempDir(), "db.sqlite3")}}}
	store := storage.NewSQLiteProvider(config)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	fixture := filepath.Join("testdata", "users_database_mtl.yml")

	require.NoError(t, store.MigrateMTL(ctx))
	// Import once, then change one stored record to force a deterministic conflict.
	_, err := ImportFileUsers(ctx, fixture, "eurmtl.me", store, false)
	require.NoError(t, err)
	err = store.SQLProvider.UpdateMTLUserPassword(ctx, 1, nil, 1)
	require.NoError(t, err)

	report, err := ImportFileUsers(ctx, fixture, "eurmtl.me", store, false)
	require.Error(t, err)
	assert.Equal(t, []string{"bublik"}, report.Conflicts)
}

func TestImportFileUsersCollectsInputConflictsBeforeMutation(t *testing.T) {
	ctx := context.Background()
	config := &schema.Configuration{Storage: schema.Storage{Local: &schema.StorageLocal{Path: filepath.Join(t.TempDir(), "db.sqlite3")}}}
	store := storage.NewSQLiteProvider(config)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	fixture := filepath.Join(t.TempDir(), "users.yml")
	require.NoError(t, os.WriteFile(fixture, []byte(`users:
  Bublik:
    password: "$plaintext$password"
    displayname: "One"
    email: "same@example.com"
  bublik:
    password: "$plaintext$password"
    displayname: "Two"
    email: "other@example.com"
  third:
    password: "$plaintext$password"
    displayname: "Three"
    email: "same@example.com"
`), 0600))

	report, err := ImportFileUsers(ctx, fixture, "eurmtl.me", store, false)
	require.Error(t, err)
	assert.Equal(t, []string{"Bublik", "bublik", "third"}, report.Conflicts)

	_, found, err := store.LoadMTLUser(ctx, "Bublik")
	require.NoError(t, err)
	assert.False(t, found)
}
