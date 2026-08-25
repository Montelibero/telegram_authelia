package telegram

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/model"
)

func TestLoginServiceBeginAndComplete(t *testing.T) {
	states := NewStateStore(time.Minute, time.Now, bytes.NewReader(bytes.Repeat([]byte{0x31}, 512)), []byte("test secret"), newFakeStateReplayStore())
	client := &fakeLoginClient{identity: Identity{ProviderUserID: "987654321", Username: "bublik_tg"}}
	users := &fakeIdentityUserStore{details: model.MTLUserDetails{User: model.MTLUser{Username: "bublik", DisplayName: "Bublik", Status: model.MTLUserStatusActive}, PrimaryEmail: "bublik@eurmtl.me", Groups: []string{"app:grafana"}}, found: true}
	service := NewLoginService(client, states, users)

	authorizationURL, state, err := service.Begin(context.Background(), "/portal")
	require.NoError(t, err)
	assert.Equal(t, "https://issuer.example/auth?state="+state, authorizationURL)

	result, err := service.Complete(context.Background(), state, "code")
	require.NoError(t, err)
	assert.Equal(t, "bublik", result.Details.User.Username)
	assert.Equal(t, "/portal", result.ReturnURL)
	assert.Equal(t, "987654321", users.providerUserID)

	_, err = service.Complete(context.Background(), state, "code")
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestLoginServiceRejectsUnsafeReturnURL(t *testing.T) {
	service := NewLoginService(&fakeLoginClient{}, NewStateStore(time.Minute, nil, nil, []byte("test secret"), newFakeStateReplayStore()), &fakeIdentityUserStore{})

	_, _, err := service.Begin(context.Background(), "https://attacker.example/")
	assert.ErrorIs(t, err, ErrUnsafeReturnURL)
	_, _, err = service.Begin(context.Background(), "//attacker.example/")
	assert.ErrorIs(t, err, ErrUnsafeReturnURL)
}

func TestLoginServiceRejectsUnknownAndDisabledUsers(t *testing.T) {
	testCases := []struct {
		name  string
		store *fakeIdentityUserStore
		err   error
	}{
		{name: "Unknown", store: &fakeIdentityUserStore{}, err: ErrIdentityNotLinked},
		{name: "Disabled", store: &fakeIdentityUserStore{found: true, details: model.MTLUserDetails{User: model.MTLUser{Status: model.MTLUserStatusDisabled}}}, err: ErrUserDisabled},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			states := NewStateStore(time.Minute, time.Now, bytes.NewReader(bytes.Repeat([]byte{0x42}, 512)), []byte("test secret"), newFakeStateReplayStore())
			service := NewLoginService(&fakeLoginClient{identity: Identity{ProviderUserID: "1"}}, states, tc.store)
			_, state, err := service.Begin(context.Background(), "")
			require.NoError(t, err)

			_, err = service.Complete(context.Background(), state, "code")
			assert.ErrorIs(t, err, tc.err)
		})
	}
}

func TestLoginServiceReturnsPendingRegistrationWithoutUser(t *testing.T) {
	states := NewStateStore(time.Minute, time.Now, bytes.NewReader(bytes.Repeat([]byte{0x43}, 512)), []byte("test secret"), newFakeStateReplayStore())
	registrations := &fakeRegistrationStore{}
	service := NewLoginServiceWithRegistration(
		&fakeLoginClient{identity: Identity{ProviderUserID: "1", Username: "bublik"}}, states,
		&fakeIdentityUserStore{}, NewRegistrationService(registrations, "eurmtl.me"),
	)
	_, state, err := service.Begin(context.Background(), "")
	require.NoError(t, err)

	result, err := service.Complete(context.Background(), state, "code")
	require.NoError(t, err)
	assert.Equal(t, model.MTLRegistrationStatusPending, result.RegistrationStatus)
	assert.Empty(t, result.Details.User.Username)
}

type fakeLoginClient struct {
	identity Identity
	err      error
}

func (c *fakeLoginClient) AuthorizationURL(flow Flow) string {
	return "https://issuer.example/auth?state=" + flow.State
}

func (c *fakeLoginClient) Exchange(context.Context, string, Flow) (Identity, error) {
	return c.identity, c.err
}

type fakeIdentityUserStore struct {
	details        model.MTLUserDetails
	found          bool
	err            error
	providerUserID string
}

func (s *fakeIdentityUserStore) LoadMTLUserByIdentity(_ context.Context, provider, providerUserID string) (model.MTLUserDetails, bool, error) {
	if provider != "telegram" {
		return model.MTLUserDetails{}, false, errors.New("unexpected provider")
	}
	s.providerUserID = providerUserID
	return s.details, s.found, s.err
}
