package telegram

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/authelia/authelia/v4/internal/model"
)

var (
	// ErrUnsafeReturnURL indicates an external or malformed return URL.
	ErrUnsafeReturnURL = errors.New("unsafe Telegram login return URL")
	// ErrIdentityNotLinked indicates the verified identity has no local user.
	ErrIdentityNotLinked = errors.New("Telegram identity is not linked")
	// ErrUserDisabled indicates the linked local user is disabled.
	ErrUserDisabled = errors.New("Telegram identity belongs to a disabled user")
)

type loginClient interface {
	AuthorizationURL(flow Flow) string
	Exchange(ctx context.Context, code string, flow Flow) (Identity, error)
}

// IdentityUserStore resolves stable provider identities to local users.
type IdentityUserStore interface {
	LoadMTLUserByIdentity(ctx context.Context, provider, providerUserID string) (model.MTLUserDetails, bool, error)
}

// LoginResult is a verified active local user and its state-bound return URL.
type LoginResult struct {
	Details   model.MTLUserDetails
	Identity  Identity
	ReturnURL string
}

// LoginService coordinates state, OIDC verification, and local identity resolution.
type LoginService struct {
	client loginClient
	states *StateStore
	users  IdentityUserStore
}

// NewLoginService constructs a Telegram login service.
func NewLoginService(client loginClient, states *StateStore, users IdentityUserStore) *LoginService {
	return &LoginService{client: client, states: states, users: users}
}

// Begin creates a state-bound flow and returns its authorization URL and state.
func (s *LoginService) Begin(returnURL string) (authorizationURL, state string, err error) {
	if !safeReturnURL(returnURL) {
		return "", "", ErrUnsafeReturnURL
	}

	flow, err := s.states.Create(returnURL)
	if err != nil {
		return "", "", err
	}

	return s.client.AuthorizationURL(flow), flow.State, nil
}

// Complete consumes the flow, verifies OIDC, and resolves an active linked user.
func (s *LoginService) Complete(ctx context.Context, state, code string) (LoginResult, error) {
	flow, err := s.states.Consume(state)
	if err != nil {
		return LoginResult{}, err
	}
	if flow.Purpose != "login" {
		return LoginResult{}, ErrInvalidState
	}

	identity, err := s.client.Exchange(ctx, code, flow)
	if err != nil {
		return LoginResult{}, err
	}

	details, found, err := s.users.LoadMTLUserByIdentity(ctx, "telegram", identity.ProviderUserID)
	if err != nil {
		return LoginResult{}, err
	}
	if !found {
		return LoginResult{}, ErrIdentityNotLinked
	}
	if details.User.Status != model.MTLUserStatusActive {
		return LoginResult{}, ErrUserDisabled
	}

	return LoginResult{Details: details, Identity: identity, ReturnURL: flow.ReturnURL}, nil
}

// Purpose returns the validated purpose of a pending flow without consuming it.
func (s *LoginService) Purpose(state string) (string, error) {
	flow, err := s.states.Inspect(state)
	return flow.Purpose, err
}

func safeReturnURL(value string) bool {
	if value == "" {
		return true
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.Contains(value, "\\") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && !parsed.IsAbs() && parsed.Host == ""
}
