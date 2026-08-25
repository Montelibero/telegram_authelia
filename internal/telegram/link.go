package telegram

import (
	"context"
	"errors"

	"github.com/authelia/authelia/v4/internal/model"
)

// ErrLinkUserMismatch indicates the callback session differs from the user that started linking.
var ErrLinkUserMismatch = errors.New("Telegram linking user mismatch")

// IdentityLinkStore persists and removes external user identities.
type IdentityLinkStore interface {
	LinkMTLUserIdentity(ctx context.Context, username, provider, providerUserID, providerUsername string) error
	UnlinkMTLUserIdentity(ctx context.Context, username, provider string) error
	LoadMTLUserIdentity(ctx context.Context, username, provider string) (identity model.MTLUserIdentity, found bool, err error)
}

// LinkStatus is the public account-linking state for one local user.
type LinkStatus struct {
	Linked           bool   `json:"linked"`
	ProviderUserID   string `json:"provider_user_id,omitempty"`
	ProviderUsername string `json:"provider_username,omitempty"`
}

// Status returns the Telegram identity linked to the current local user.
func (s *LinkService) Status(ctx context.Context, currentUsername string) (LinkStatus, error) {
	identity, found, err := s.store.LoadMTLUserIdentity(ctx, currentUsername, "telegram")
	if err != nil || !found {
		return LinkStatus{Linked: false}, err
	}
	status := LinkStatus{Linked: true, ProviderUserID: identity.ProviderUserID}
	if identity.ProviderUsername != nil {
		status.ProviderUsername = *identity.ProviderUsername
	}
	return status, nil
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
func (s *LinkService) Begin(ctx context.Context, username string) (authorizationURL, state string, err error) {
	flow, err := s.states.CreateLink(ctx, username)
	if err != nil {
		return "", "", err
	}
	return s.client.AuthorizationURL(flow), flow.State, nil
}

// Complete verifies the callback and links it only to the initiating current user.
func (s *LinkService) Complete(ctx context.Context, currentUsername, state, code string) error {
	flow, err := s.states.Consume(ctx, state)
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

// Purpose returns the validated purpose of a pending flow without consuming it.
func (s *LinkService) Purpose(state string) (string, error) {
	flow, err := s.states.Inspect(state)
	return flow.Purpose, err
}

// Unlink removes the Telegram identity from the exact current user.
func (s *LinkService) Unlink(ctx context.Context, currentUsername string) error {
	return s.store.UnlinkMTLUserIdentity(ctx, currentUsername, "telegram")
}
