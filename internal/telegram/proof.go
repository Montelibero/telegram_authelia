package telegram

import (
	"context"
	"errors"

	"github.com/authelia/authelia/v4/internal/model"
)

var (
	ErrPasswordProofUserMismatch     = errors.New("Telegram password proof user mismatch")
	ErrPasswordProofIdentityMismatch = errors.New("Telegram password proof identity mismatch")
	ErrPasswordProofSessionMismatch  = errors.New("Telegram password proof session mismatch")
)

type PasswordProofStore interface {
	LoadMTLUserIdentity(ctx context.Context, username, provider string) (identity model.MTLUserIdentity, found bool, err error)
}

// PasswordProofService verifies a user's already-linked Telegram identity before first password setup.
type PasswordProofService struct {
	client loginClient
	states *StateStore
	store  PasswordProofStore
}

func NewPasswordProofService(client loginClient, states *StateStore, store PasswordProofStore) *PasswordProofService {
	return &PasswordProofService{client: client, states: states, store: store}
}

func (s *PasswordProofService) Begin(ctx context.Context, username, sessionBinding string) (authorizationURL, state string, err error) {
	flow, err := s.states.CreatePasswordSetup(ctx, username, sessionBinding)
	if err != nil {
		return "", "", err
	}
	return s.client.AuthorizationURL(flow), flow.State, nil
}

func (s *PasswordProofService) Purpose(state string) (string, error) {
	flow, err := s.states.Inspect(state)
	return flow.Purpose, err
}

func (s *PasswordProofService) Complete(ctx context.Context, currentUsername, sessionBinding, state, code string) (string, error) {
	flow, err := s.states.Inspect(state)
	if err != nil {
		return "", err
	}
	if flow.Purpose != "password_setup" || flow.Username == "" || flow.Username != currentUsername {
		return "", ErrPasswordProofUserMismatch
	}
	if flow.SessionBinding == "" || flow.SessionBinding != sessionBinding {
		return "", ErrPasswordProofSessionMismatch
	}
	if _, err = s.states.Consume(ctx, state); err != nil {
		return "", err
	}
	identity, err := s.client.Exchange(ctx, code, flow)
	if err != nil {
		return "", err
	}
	linked, found, err := s.store.LoadMTLUserIdentity(ctx, currentUsername, "telegram")
	if err != nil {
		return "", err
	}
	if !found || linked.ProviderUserID != identity.ProviderUserID {
		return "", ErrPasswordProofIdentityMismatch
	}
	grant, err := s.states.CreatePasswordGrant(ctx, currentUsername, sessionBinding)
	if err != nil {
		return "", err
	}
	return grant.State, nil
}

func (s *PasswordProofService) Validate(currentUsername, sessionBinding, grant string) error {
	flow, err := s.states.Inspect(grant)
	if err != nil {
		return err
	}
	if flow.Purpose != "password_grant" || flow.Username == "" || flow.Username != currentUsername {
		return ErrPasswordProofUserMismatch
	}
	if flow.SessionBinding == "" || flow.SessionBinding != sessionBinding {
		return ErrPasswordProofSessionMismatch
	}
	return nil
}

func (s *PasswordProofService) Consume(ctx context.Context, currentUsername, sessionBinding, grant string) error {
	if err := s.Validate(currentUsername, sessionBinding, grant); err != nil {
		return err
	}
	if _, err := s.states.Consume(ctx, grant); err != nil {
		return err
	}
	return nil
}
