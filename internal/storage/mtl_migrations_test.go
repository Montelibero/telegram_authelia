package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMTLMigrationsCreateSchema(t *testing.T) {
	provider := newTestSQLiteProvider(t)
	t.Cleanup(func() { require.NoError(t, provider.Close()) })

	require.NoError(t, provider.MigrateMTL(context.Background()))

	var tables []string
	require.NoError(t, provider.db.Select(&tables, `SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE 'mtl_%' ORDER BY name`))
	assert.Equal(t, []string{
		"mtl_audit_events",
		"mtl_group_memberships",
		"mtl_groups",
		"mtl_schema_migrations",
		"mtl_user_emails",
		"mtl_user_identities",
		"mtl_users",
	}, tables)

	var version int
	require.NoError(t, provider.db.Get(&version, `SELECT MAX(version) FROM mtl_schema_migrations`))
	assert.Equal(t, 1, version)
}

func TestMTLMigrationsAreIdempotent(t *testing.T) {
	provider := newTestSQLiteProvider(t)
	t.Cleanup(func() { require.NoError(t, provider.Close()) })

	require.NoError(t, provider.MigrateMTL(context.Background()))
	require.NoError(t, provider.MigrateMTL(context.Background()))

	var count int
	require.NoError(t, provider.db.Get(&count, `SELECT COUNT(*) FROM mtl_schema_migrations`))
	assert.Equal(t, 1, count)
}

func TestMTLSchemaConstraints(t *testing.T) {
	provider := newTestSQLiteProvider(t)
	t.Cleanup(func() { require.NoError(t, provider.Close()) })
	require.NoError(t, provider.MigrateMTL(context.Background()))

	ctx := context.Background()
	_, err := provider.db.ExecContext(ctx, `INSERT INTO mtl_users (username, display_name, status) VALUES ('bublik', 'Bublik', 'pending')`)
	require.Error(t, err)

	result, err := provider.db.ExecContext(ctx, `INSERT INTO mtl_users (username, display_name, status) VALUES ('bublik', 'Bublik', 'active')`)
	require.NoError(t, err)
	userID, err := result.LastInsertId()
	require.NoError(t, err)

	_, err = provider.db.ExecContext(ctx, `INSERT INTO mtl_users (username, display_name, status) VALUES ('bublik', 'Other', 'active')`)
	require.Error(t, err)

	_, err = provider.db.ExecContext(ctx, `INSERT INTO mtl_user_emails (user_id, email, is_primary) VALUES (?, 'bublik@eurmtl.me', 1)`, userID)
	require.NoError(t, err)
	_, err = provider.db.ExecContext(ctx, `INSERT INTO mtl_user_emails (user_id, email, is_primary) VALUES (?, 'other@eurmtl.me', 1)`, userID)
	require.Error(t, err)

	_, err = provider.db.ExecContext(ctx, `INSERT INTO mtl_users (username, display_name, status) VALUES ('other', 'Other', 'active')`)
	require.NoError(t, err)
	_, err = provider.db.ExecContext(ctx, `INSERT INTO mtl_user_emails (user_id, email) VALUES (2, 'BUBLIK@EURMTL.ME')`)
	require.Error(t, err)

	_, err = provider.db.ExecContext(ctx, `INSERT INTO mtl_user_identities (user_id, provider, provider_user_id) VALUES (?, 'telegram', '12345')`, userID)
	require.NoError(t, err)
	_, err = provider.db.ExecContext(ctx, `INSERT INTO mtl_user_identities (user_id, provider, provider_user_id) VALUES (2, 'telegram', '12345')`)
	require.Error(t, err)
}

func TestMTLMigrationFailureRollsBack(t *testing.T) {
	provider := newTestSQLiteProvider(t)
	t.Cleanup(func() { require.NoError(t, provider.Close()) })

	_, err := provider.db.Exec(`CREATE TABLE mtl_users (broken INTEGER)`)
	require.NoError(t, err)
	require.Error(t, provider.MigrateMTL(context.Background()))

	var count int
	require.NoError(t, provider.db.Get(&count, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'mtl_schema_migrations'`))
	assert.Zero(t, count)
}
