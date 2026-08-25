package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/model"
)

func TestMTLRegistrationLifecycle(t *testing.T) {
	provider := newTestSQLiteProvider(t)
	t.Cleanup(func() { require.NoError(t, provider.Close()) })
	require.NoError(t, provider.MigrateMTL(context.Background()))
	ctx := context.Background()

	request, err := provider.UpsertMTLRegistration(ctx, model.MTLRegistrationCandidate{
		Provider: "telegram", ProviderUserID: "12345", ProviderUsername: "bublik",
		DisplayName: "Bublik", ProposedUsername: "bublik", ProposedEmail: "bublik@eurmtl.me",
	})
	require.NoError(t, err)
	assert.Equal(t, model.MTLRegistrationStatusPending, request.Status)
	assert.Equal(t, 1, request.Version)

	updated, err := provider.UpsertMTLRegistration(ctx, model.MTLRegistrationCandidate{
		Provider: "telegram", ProviderUserID: "12345", ProviderUsername: "new_bublik",
		DisplayName: "New Bublik", ProposedUsername: "new_bublik", ProposedEmail: "new_bublik@eurmtl.me",
	})
	require.NoError(t, err)
	assert.Equal(t, request.ID, updated.ID)
	assert.Equal(t, 2, updated.Version)
	assert.Equal(t, "new_bublik", updated.ProviderUsername.String)

	listed, err := provider.ListMTLRegistrations(ctx, model.MTLRegistrationStatusPending)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, updated.ID, listed[0].ID)

	rejected, err := provider.RejectMTLRegistration(ctx, updated.ID, updated.Version, "")
	require.NoError(t, err)
	assert.Equal(t, model.MTLRegistrationStatusRejected, rejected.Status)

	replayed, err := provider.UpsertMTLRegistration(ctx, model.MTLRegistrationCandidate{
		Provider: "telegram", ProviderUserID: "12345", ProviderUsername: "latest",
	})
	require.NoError(t, err)
	assert.Equal(t, model.MTLRegistrationStatusRejected, replayed.Status)
	assert.Equal(t, "latest", replayed.ProviderUsername.String)

	_, err = provider.RejectMTLRegistration(ctx, replayed.ID, replayed.Version-1, "")
	assert.ErrorIs(t, err, ErrMTLVersionConflict)
}

func TestMTLRegistrationAllowsMissingTelegramUsername(t *testing.T) {
	provider := newTestSQLiteProvider(t)
	t.Cleanup(func() { require.NoError(t, provider.Close()) })
	require.NoError(t, provider.MigrateMTL(context.Background()))

	request, err := provider.UpsertMTLRegistration(context.Background(), model.MTLRegistrationCandidate{Provider: "telegram", ProviderUserID: "no-username"})
	require.NoError(t, err)
	assert.False(t, request.ProviderUsername.Valid)
	assert.False(t, request.ProposedUsername.Valid)
	assert.False(t, request.ProposedEmail.Valid)
}

func TestMTLRegistrationApprovalIsAtomic(t *testing.T) {
	provider := newTestSQLiteProvider(t)
	t.Cleanup(func() { require.NoError(t, provider.Close()) })
	require.NoError(t, provider.MigrateMTL(context.Background()))
	ctx := context.Background()

	request, err := provider.UpsertMTLRegistration(ctx, model.MTLRegistrationCandidate{
		Provider: "telegram", ProviderUserID: "987654321", ProviderUsername: "bublik",
		DisplayName: "Bublik", ProposedUsername: "bublik", ProposedEmail: "bublik@eurmtl.me",
	})
	require.NoError(t, err)

	username, err := provider.ApproveMTLRegistration(ctx, model.MTLRegistrationApproval{RequestID: request.ID, ExpectedVersion: request.Version})
	require.NoError(t, err)
	assert.Equal(t, "bublik", username)

	details, found, err := provider.LoadMTLUser(ctx, "bublik")
	require.NoError(t, err)
	require.True(t, found)
	assert.False(t, details.User.PasswordHash.Valid)
	assert.Equal(t, "bublik@eurmtl.me", details.PrimaryEmail)
	assert.Empty(t, details.Groups)

	identity, found, err := provider.LoadMTLUserIdentity(ctx, "bublik", "telegram")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "987654321", identity.ProviderUserID)

	approved, found, err := provider.LoadMTLRegistration(ctx, request.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.MTLRegistrationStatusApproved, approved.Status)
	assert.True(t, approved.ApprovedUserID.Valid)

	var userAuditCount, registrationAuditCount int
	require.NoError(t, provider.db.GetContext(ctx, &userAuditCount, `SELECT COUNT(*) FROM mtl_audit_events WHERE event_type = 'user.created' AND target_type = 'user' AND target_id = 'bublik'`))
	assert.Equal(t, 1, userAuditCount)
	require.NoError(t, provider.db.GetContext(ctx, &registrationAuditCount, `SELECT COUNT(*) FROM mtl_audit_events WHERE event_type = 'registration.approved' AND target_type = 'registration' AND target_id = ?`, request.ID))
	assert.Equal(t, 1, registrationAuditCount)
}

func TestMTLRegistrationApprovalRejectsIncompleteAndConflictingData(t *testing.T) {
	provider := newTestSQLiteProvider(t)
	t.Cleanup(func() { require.NoError(t, provider.Close()) })
	require.NoError(t, provider.MigrateMTL(context.Background()))
	ctx := context.Background()

	request, err := provider.UpsertMTLRegistration(ctx, model.MTLRegistrationCandidate{Provider: "telegram", ProviderUserID: "no-username"})
	require.NoError(t, err)
	_, err = provider.ApproveMTLRegistration(ctx, model.MTLRegistrationApproval{RequestID: request.ID, ExpectedVersion: request.Version})
	assert.ErrorIs(t, err, ErrMTLRegistrationIncomplete)

	request, err = provider.UpsertMTLRegistration(ctx, model.MTLRegistrationCandidate{Provider: "telegram", ProviderUserID: "with-override"})
	require.NoError(t, err)
	username, err := provider.ApproveMTLRegistration(ctx, model.MTLRegistrationApproval{
		RequestID: request.ID, ExpectedVersion: request.Version, Username: "manual", Email: "manual@eurmtl.me",
	})
	require.NoError(t, err)
	assert.Equal(t, "manual", username)

	conflict, err := provider.UpsertMTLRegistration(ctx, model.MTLRegistrationCandidate{
		Provider: "telegram", ProviderUserID: "conflict", ProposedUsername: "manual", ProposedEmail: "other@eurmtl.me",
	})
	require.NoError(t, err)
	_, err = provider.ApproveMTLRegistration(ctx, model.MTLRegistrationApproval{RequestID: conflict.ID, ExpectedVersion: conflict.Version})
	assert.ErrorIs(t, err, ErrMTLConflict)
	emailConflict, err := provider.UpsertMTLRegistration(ctx, model.MTLRegistrationCandidate{
		Provider: "telegram", ProviderUserID: "email-conflict", ProposedUsername: "other", ProposedEmail: "manual@eurmtl.me",
	})
	require.NoError(t, err)
	_, err = provider.ApproveMTLRegistration(ctx, model.MTLRegistrationApproval{RequestID: emailConflict.ID, ExpectedVersion: emailConflict.Version})
	assert.ErrorIs(t, err, ErrMTLConflict)

	var users int
	require.NoError(t, provider.db.GetContext(ctx, &users, `SELECT COUNT(*) FROM mtl_users`))
	assert.Equal(t, 1, users)
}
