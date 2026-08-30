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

type preauthorizedIdentityUserStore interface {
	FinalizeMTLTelegramPreauthorization(ctx context.Context, providerUserID, providerUsername, displayName, generatedEmailDomain string) (model.MTLUserDetails, bool, error)
}

type telegramIdentityProfileStore interface {
	SyncMTLTelegramIdentityProfile(ctx context.Context, providerUserID, providerUsername, generatedEmailDomain string) error
}

// LoginResult is a verified active local user and its state-bound return URL.
type LoginResult struct {
	Details            model.MTLUserDetails
	Identity           Identity
	ReturnURL          string
	RegistrationStatus model.MTLRegistrationStatus
}

// LoginService coordinates state, OIDC verification, and local identity resolution.
type LoginService struct {
	client        loginClient
	states        *StateStore
	users         IdentityUserStore
	registrations *RegistrationService
}

// NewLoginService constructs a Telegram login service.
func NewLoginService(client loginClient, states *StateStore, users IdentityUserStore) *LoginService {
	return &LoginService{client: client, states: states, users: users}
}

// NewLoginServiceWithRegistration constructs a login service that captures unknown identities for approval.
func NewLoginServiceWithRegistration(client loginClient, states *StateStore, users IdentityUserStore, registrations *RegistrationService) *LoginService {
	return &LoginService{client: client, states: states, users: users, registrations: registrations}
}

// Begin creates a state-bound flow and returns its authorization URL and state.
func (s *LoginService) Begin(ctx context.Context, returnURL string) (authorizationURL, state string, err error) {
	if !safeReturnURL(returnURL) {
		return "", "", ErrUnsafeReturnURL
	}

	flow, err := s.states.Create(ctx, returnURL)
	if err != nil {
		return "", "", err
	}

	return s.client.AuthorizationURL(flow), flow.State, nil
}

// Complete consumes the flow, verifies OIDC, and resolves an active linked user.
func (s *LoginService) Complete(ctx context.Context, state, code string) (LoginResult, error) {
	flow, err := s.states.Consume(ctx, state)
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
	if finalizer, ok := s.users.(preauthorizedIdentityUserStore); ok {
		domain := ""
		if s.registrations != nil {
			domain = s.registrations.generatedEmailDomain
		}
		details, finalized, finalizeErr := finalizer.FinalizeMTLTelegramPreauthorization(
			ctx, identity.ProviderUserID, identity.Username, identity.Name, domain,
		)
		if finalizeErr != nil {
			return LoginResult{}, finalizeErr
		}
		if finalized {
			if details.User.Status != model.MTLUserStatusActive {
				return LoginResult{}, ErrUserDisabled
			}
			return LoginResult{Details: details, Identity: identity, ReturnURL: flow.ReturnURL}, nil
		}
	}

	details, found, err := s.users.LoadMTLUserByIdentity(ctx, "telegram", identity.ProviderUserID)
	if err != nil {
		return LoginResult{}, err
	}
	if !found {
		if s.registrations != nil {
			request, registerErr := s.registrations.Register(ctx, identity)
			if registerErr != nil {
				return LoginResult{}, registerErr
			}
			return LoginResult{Identity: identity, ReturnURL: flow.ReturnURL, RegistrationStatus: request.Status}, nil
		}
		return LoginResult{}, ErrIdentityNotLinked
	}
	if details.User.Status != model.MTLUserStatusActive {
		return LoginResult{}, ErrUserDisabled
	}
	if profileStore, ok := s.users.(telegramIdentityProfileStore); ok {
		domain := ""
		if s.registrations != nil {
			domain = s.registrations.generatedEmailDomain
		}
		if err = profileStore.SyncMTLTelegramIdentityProfile(ctx, identity.ProviderUserID, identity.Username, domain); err != nil {
			return LoginResult{}, err
		}
		if details, found, err = s.users.LoadMTLUserByIdentity(ctx, "telegram", identity.ProviderUserID); err != nil {
			return LoginResult{}, err
		}
		if !found {
			return LoginResult{}, ErrIdentityNotLinked
		}
	}

	return LoginResult{Details: details, Identity: identity, ReturnURL: flow.ReturnURL}, nil
}

// Purpose returns the validated purpose of a pending flow without consuming it.
func (s *LoginService) Purpose(ctx context.Context, state string) (string, error) {
	flow, err := s.states.Inspect(ctx, state)
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
