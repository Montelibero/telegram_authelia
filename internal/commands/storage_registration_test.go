package commands

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/storage"
)

func TestStorageRegistrationCommandHelpListsWorkflow(t *testing.T) {
	cmd := newStorageRegistrationCmd(NewCmdCtx())
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	require.NoError(t, cmd.Execute())
	for _, name := range []string{"list", "show", "approve", "reject"} {
		assert.Contains(t, buf.String(), name)
	}
}

func TestRunStorageRegistrationLifecycle(t *testing.T) {
	ctx := context.Background()
	config := &schema.Configuration{Storage: schema.Storage{Local: &schema.StorageLocal{Path: filepath.Join(t.TempDir(), "db.sqlite3")}}}
	store := storage.NewSQLiteProvider(config)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.MigrateMTL(ctx))
	request, err := store.UpsertMTLRegistration(ctx, model.MTLRegistrationCandidate{Provider: "telegram", ProviderUserID: "42", ProviderUsername: "bublik", ProposedUsername: "bublik", ProposedEmail: "bublik@eurmtl.me"})
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	require.NoError(t, runStorageRegistrationList(ctx, buf, store, "pending"))
	assert.Contains(t, buf.String(), "bublik@eurmtl.me")
	assert.Contains(t, buf.String(), "VERSION")

	buf.Reset()
	require.NoError(t, runStorageRegistrationShow(ctx, buf, store, request.ID))
	assert.Contains(t, buf.String(), "Provider user ID: 42")

	buf.Reset()
	require.NoError(t, runStorageRegistrationApprove(ctx, buf, store, model.MTLRegistrationApproval{RequestID: request.ID, ExpectedVersion: request.Version}))
	assert.Equal(t, "Approved registration 1 and created user bublik.\n", buf.String())
}

func TestRunStorageRegistrationRejectAndValidation(t *testing.T) {
	ctx := context.Background()
	config := &schema.Configuration{Storage: schema.Storage{Local: &schema.StorageLocal{Path: filepath.Join(t.TempDir(), "db.sqlite3")}}}
	store := storage.NewSQLiteProvider(config)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.MigrateMTL(ctx))
	request, err := store.UpsertMTLRegistration(ctx, model.MTLRegistrationCandidate{Provider: "telegram", ProviderUserID: "43"})
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	require.NoError(t, runStorageRegistrationReject(ctx, buf, store, request.ID, request.Version, ""))
	assert.Equal(t, "Rejected registration 1.\n", buf.String())
	assert.ErrorContains(t, runStorageRegistrationList(ctx, buf, store, "unknown"), "invalid registration status")
	_, err = parseRegistrationID("nope")
	assert.Error(t, err)

	help := newStorageRegistrationApproveCmd(NewCmdCtx()).Example
	assert.True(t, strings.Contains(help, "--version") && strings.Contains(help, "--email"))
}
