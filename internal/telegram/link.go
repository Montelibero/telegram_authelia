package telegram

import (
	"context"
	"errors"
)

// ErrLinkUserMismatch indicates the callback session differs from the user that started linking.
var ErrLinkUserMismatch = errors.New("Telegram linking user mismatch")

// IdentityLinkStore persists and removes external user identities.
type IdentityLinkStore interface {
	LinkMTLUserIdentity(ctx context.Context, username, provider, providerUserID, providerUsername string) error
	UnlinkMTLUserIdentity(ctx context.Context, username, provider string) error
}

// LinkService coordinates user-bound Telegram linking flows.
type LinkService struct {
	client loginClient
	states *StateStore
	store  IdentityLinkStore
}

// NewLinkService constructs a Telegram account linking service.
func NewLinkService(client loginClient, states *StateStore, store IdentityLinkStore) *LinkService {
	return &LinkService{client: client, states: states, store: store}
}

// Begin creates a linking flow bound to the current local username.
func (s *LinkService) Begin(username string) (authorizationURL, state string, err error) {
	flow, err := s.states.CreateLink(username)
	if err != nil {
		return "", "", err
	}
	return s.client.AuthorizationURL(flow), flow.State, nil
}

// Complete verifies the callback and links it only to the initiating current user.
func (s *LinkService) Complete(ctx context.Context, currentUsername, state, code string) error {
	flow, err := s.states.Consume(state)
	if err != nil {
		return err
	}
	if flow.Purpose != "link" || flow.Username == "" || flow.Username != currentUsername {
		return ErrLinkUserMismatch
	}
	identity, err := s.client.Exchange(ctx, code, flow)
	if err != nil {
		return err
	}
	return s.store.LinkMTLUserIdentity(ctx, currentUsername, "telegram", identity.ProviderUserID, identity.Username)
}

// Unlink removes the Telegram identity from the exact current user.
func (s *LinkService) Unlink(ctx context.Context, currentUsername string) error {
	return s.store.UnlinkMTLUserIdentity(ctx, currentUsername, "telegram")
}
