package telegram

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/model"
)

func TestPasswordProofServiceRequiresExactLinkedIdentityAndSingleUseGrant(t *testing.T) {
	states := NewStateStore(time.Minute, time.Now, bytes.NewReader(bytes.Repeat([]byte{0x61}, 2048)), []byte("test secret"), newFakeStateReplayStore())
	service := NewPasswordProofService(
		&fakeLoginClient{identity: Identity{ProviderUserID: "42", Username: "bublik_tg"}},
		states,
		&fakePasswordProofStore{identity: model.MTLUserIdentity{ProviderUserID: "42"}},
	)

	_, state, err := service.Begin(context.Background(), "bublik", "session-a")
	require.NoError(t, err)
	_, err = service.Complete(context.Background(), "bublik", "session-b", state, "code")
	assert.ErrorIs(t, err, ErrPasswordProofSessionMismatch)

	grant, err := service.Complete(context.Background(), "bublik", "session-a", state, "code")
	require.NoError(t, err)
	assert.ErrorIs(t, service.Validate("bublik", "session-b", grant), ErrPasswordProofSessionMismatch)
	require.NoError(t, service.Consume(context.Background(), "bublik", "session-a", grant))
	assert.ErrorIs(t, service.Consume(context.Background(), "bublik", "session-a", grant), ErrInvalidState)

	_, state, err = service.Begin(context.Background(), "bublik", "session-a")
	require.NoError(t, err)
	_, err = service.Complete(context.Background(), "other", "session-a", state, "code")
	assert.ErrorIs(t, err, ErrPasswordProofUserMismatch)
}

func TestPasswordProofServiceRejectsDifferentTelegramIdentity(t *testing.T) {
	states := NewStateStore(time.Minute, time.Now, bytes.NewReader(bytes.Repeat([]byte{0x62}, 1024)), []byte("test secret"), newFakeStateReplayStore())
	service := NewPasswordProofService(
		&fakeLoginClient{identity: Identity{ProviderUserID: "99"}},
		states,
		&fakePasswordProofStore{identity: model.MTLUserIdentity{ProviderUserID: "42"}},
	)
	_, state, err := service.Begin(context.Background(), "bublik", "session-a")
	require.NoError(t, err)
	_, err = service.Complete(context.Background(), "bublik", "session-a", state, "code")
	assert.ErrorIs(t, err, ErrPasswordProofIdentityMismatch)
}

type fakePasswordProofStore struct {
	identity model.MTLUserIdentity
}

func (s *fakePasswordProofStore) LoadMTLUserIdentity(context.Context, string, string) (model.MTLUserIdentity, bool, error) {
	return s.identity, true, nil
}
