package telegram

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinkServiceBindsCallbackToLocalUser(t *testing.T) {
	states := NewStateStore(time.Minute, time.Now, bytes.NewReader(bytes.Repeat([]byte{0x51}, 256)))
	client := &fakeLoginClient{identity: Identity{ProviderUserID: "987654321", Username: "bublik_tg"}}
	store := &fakeLinkStore{}
	service := NewLinkService(client, states, store)

	_, state, err := service.Begin("bublik")
	require.NoError(t, err)
	require.NoError(t, service.Complete(context.Background(), "bublik", state, "code"))
	assert.Equal(t, "bublik", store.username)
	assert.Equal(t, "987654321", store.providerUserID)

	_, state, err = service.Begin("bublik")
	require.NoError(t, err)
	err = service.Complete(context.Background(), "other", state, "code")
	assert.ErrorIs(t, err, ErrLinkUserMismatch)
}

func TestLinkServiceUnlinksExactCurrentUser(t *testing.T) {
	store := &fakeLinkStore{}
	service := NewLinkService(&fakeLoginClient{}, NewStateStore(time.Minute, nil, nil), store)

	require.NoError(t, service.Unlink(context.Background(), "bublik"))
	assert.Equal(t, "bublik", store.unlinkedUsername)
}

type fakeLinkStore struct {
	username         string
	providerUserID   string
	unlinkedUsername string
}

func (s *fakeLinkStore) LinkMTLUserIdentity(_ context.Context, username, provider, providerUserID, providerUsername string) error {
	s.username, s.providerUserID = username, providerUserID
	return nil
}

func (s *fakeLinkStore) UnlinkMTLUserIdentity(_ context.Context, username, provider string) error {
	s.unlinkedUsername = username
	return nil
}
