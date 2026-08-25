package storage

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/model"
)

func TestMTLAdminUserLifecycle(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := context.Background()

	details, err := provider.CreateMTLAdminUser(ctx, model.MTLAdminUserCreate{
		Username: "bublik", DisplayName: "Bublik", Email: "bublik@eurmtl.me",
	}, "")
	require.NoError(t, err)
	assert.Equal(t, "bublik", details.Username)
	assert.Equal(t, model.MTLUserStatusActive, details.Status)
	assert.False(t, details.PasswordEnabled)
	assert.Equal(t, "bublik@eurmtl.me", details.PrimaryEmail)
	require.Len(t, details.Emails, 1)
	assert.True(t, details.Emails[0].Verified)

	users, err := provider.ListMTLAdminUsers(ctx)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, "bublik", users[0].Username)

	details, err = provider.UpdateMTLAdminUser(ctx, "bublik", model.MTLAdminUserUpdate{
		ExpectedVersion: details.Version, DisplayName: "New Bublik", Status: model.MTLUserStatusDisabled,
	}, "")
	require.NoError(t, err)
	assert.Equal(t, "New Bublik", details.DisplayName)
	assert.Equal(t, model.MTLUserStatusDisabled, details.Status)
	assert.Equal(t, 1, details.SessionEpoch)

	details, err = provider.AddMTLAdminUserEmail(ctx, "bublik", model.MTLAdminEmailCreate{
		ExpectedVersion: details.Version, Email: "other@example.com",
	}, "")
	require.NoError(t, err)
	require.Len(t, details.Emails, 2)

	details, err = provider.SetMTLAdminPrimaryEmail(ctx, "bublik", "other@example.com", details.Version, "")
	require.NoError(t, err)
	assert.Equal(t, "other@example.com", details.PrimaryEmail)

	details, err = provider.DeleteMTLAdminUserEmail(ctx, "bublik", "bublik@eurmtl.me", details.Version, "")
	require.NoError(t, err)
	require.Len(t, details.Emails, 1)
	assert.Equal(t, "other@example.com", details.Emails[0].Email)
	_, err = provider.DeleteMTLAdminUserEmail(ctx, "bublik", "other@example.com", details.Version, "")
	assert.ErrorIs(t, err, ErrMTLPrimaryEmailRequired)
}

func TestMTLAdminUserConcurrentUpdateHasSingleWinner(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := context.Background()
	details, err := provider.CreateMTLAdminUser(ctx, model.MTLAdminUserCreate{Username: "race", Email: "race@example.com"}, "")
	require.NoError(t, err)

	errorsFound := make(chan error, 2)
	var wait sync.WaitGroup
	for _, displayName := range []string{"First", "Second"} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, updateErr := provider.UpdateMTLAdminUser(ctx, "race", model.MTLAdminUserUpdate{ExpectedVersion: details.Version, DisplayName: displayName, Status: model.MTLUserStatusActive}, "")
			errorsFound <- updateErr
		}()
	}
	wait.Wait()
	close(errorsFound)

	successes, conflicts := 0, 0
	for updateErr := range errorsFound {
		switch {
		case updateErr == nil:
			successes++
		case errors.Is(updateErr, ErrMTLVersionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent update error: %v", updateErr)
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, conflicts)
}

func TestMTLAdminUserConflictsRollbackAndIdentityUnlink(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := context.Background()

	first, err := provider.CreateMTLAdminUser(ctx, model.MTLAdminUserCreate{Username: "first", DisplayName: "First", Email: "first@example.com"}, "")
	require.NoError(t, err)
	_, err = provider.CreateMTLAdminUser(ctx, model.MTLAdminUserCreate{Username: "second", DisplayName: "Second", Email: "first@example.com"}, "")
	assert.ErrorIs(t, err, ErrMTLConflict)

	_, err = provider.UpdateMTLAdminUser(ctx, "first", model.MTLAdminUserUpdate{ExpectedVersion: first.Version + 1, DisplayName: "Stale", Status: model.MTLUserStatusActive}, "")
	assert.ErrorIs(t, err, ErrMTLVersionConflict)
	loaded, found, err := provider.LoadMTLAdminUser(ctx, "first")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "First", loaded.DisplayName)

	require.NoError(t, provider.LinkMTLUserIdentity(ctx, "first", "telegram", "42", "first_tg"))
	loaded, found, err = provider.LoadMTLAdminUser(ctx, "first")
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, loaded.Identities, 1)

	loaded, err = provider.UnlinkMTLAdminUserIdentity(ctx, "first", "telegram", loaded.Version, "")
	require.NoError(t, err)
	assert.Empty(t, loaded.Identities)
	_, err = provider.UnlinkMTLAdminUserIdentity(ctx, "first", "telegram", loaded.Version, "")
	assert.ErrorIs(t, err, ErrMTLIdentityNotFound)

	var users int
	require.NoError(t, provider.db.Get(&users, `SELECT COUNT(*) FROM mtl_users`))
	assert.Equal(t, 1, users)
}

func TestMTLAdminUserSessionEpochAndAuditActor(t *testing.T) {
	provider := newTestMTLUserProvider(t)
	ctx := context.Background()

	_, err := provider.CreateMTLAdminUser(ctx, model.MTLAdminUserCreate{Username: "admin", Email: "admin@example.com"}, "")
	require.NoError(t, err)
	target, err := provider.CreateMTLAdminUser(ctx, model.MTLAdminUserCreate{Username: "target", Email: "target@example.com"}, "admin")
	require.NoError(t, err)

	target, err = provider.UpdateMTLAdminUser(ctx, "target", model.MTLAdminUserUpdate{ExpectedVersion: target.Version, Status: model.MTLUserStatusDisabled}, "admin")
	require.NoError(t, err)
	assert.Equal(t, 1, target.SessionEpoch)

	target, err = provider.UpdateMTLAdminUser(ctx, "target", model.MTLAdminUserUpdate{ExpectedVersion: target.Version, Status: model.MTLUserStatusDisabled}, "admin")
	require.NoError(t, err)
	assert.Equal(t, 1, target.SessionEpoch)

	target, err = provider.UpdateMTLAdminUser(ctx, "target", model.MTLAdminUserUpdate{ExpectedVersion: target.Version, Status: model.MTLUserStatusActive}, "admin")
	require.NoError(t, err)
	assert.Equal(t, 1, target.SessionEpoch)
	assert.Equal(t, "target", target.DisplayName)

	_, err = provider.UpdateMTLAdminUser(ctx, "target", model.MTLAdminUserUpdate{ExpectedVersion: target.Version, Status: "unknown"}, "admin")
	assert.ErrorIs(t, err, ErrMTLConflict)

	var actorID int64
	require.NoError(t, provider.db.Get(&actorID, `SELECT actor_user_id FROM mtl_audit_events WHERE event_type = 'user.created' AND target_id = 'target'`))
	var expectedActorID int64
	require.NoError(t, provider.db.Get(&expectedActorID, `SELECT id FROM mtl_users WHERE username = 'admin'`))
	assert.Equal(t, expectedActorID, actorID)
}
