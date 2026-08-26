package telegram

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"time"

	"github.com/google/uuid"

	"github.com/authelia/authelia/v4/internal/model"
)

var (
	// ErrInvalidState indicates the state is unknown or was already consumed.
	ErrInvalidState = errors.New("invalid Telegram authentication state")
	// ErrExpiredState indicates the state expired before it was consumed.
	ErrExpiredState = errors.New("expired Telegram authentication state")
)

// Flow binds an OIDC request to its state, nonce, PKCE verifier, and return URL.
type Flow struct {
	State          string `json:"-"`
	Nonce          string
	CodeVerifier   string
	ReturnURL      string
	Purpose        string
	Username       string
	SessionBinding string
	ReplayKey      string `json:"-"`
	ExpiresAt      time.Time
}

// StateReplayStore persists and atomically consumes state replay markers.
type StateReplayStore interface {
	SaveOneTimeCode(ctx context.Context, code model.OneTimeCode) (signature string, err error)
	LoadOneTimeCodeByPublicID(ctx context.Context, id uuid.UUID) (code *model.OneTimeCode, err error)
	ConsumeTelegramState(ctx context.Context, signature string, consumedAt time.Time) (consumed bool, err error)
	PurgeTelegramStates(ctx context.Context, before time.Time) error
}

// StateStore seals short-lived OIDC flows and uses shared storage for cluster-wide replay protection.
type StateStore struct {
	ttl    time.Duration
	now    func() time.Time
	random io.Reader
	replay StateReplayStore
}

// NewStateStore constructs a state store. Nil clock and random sources use production defaults.
func NewStateStore(ttl time.Duration, now func() time.Time, random io.Reader, _ []byte, replay StateReplayStore) *StateStore {
	if now == nil {
		now = time.Now
	}

	if random == nil {
		random = rand.Reader
	}

	return &StateStore{ttl: ttl, now: now, random: random, replay: replay}
}

// Create creates and stores a new Telegram OIDC flow.
func (s *StateStore) Create(ctx context.Context, returnURL string) (Flow, error) {
	return s.create(ctx, returnURL, "login", "")
}

// CreateLink creates a flow bound to account linking for one local username.
func (s *StateStore) CreateLink(ctx context.Context, username string) (Flow, error) {
	return s.create(ctx, "", "link", username)
}

// CreatePasswordSetup creates a fresh Telegram verification flow for password setup.
func (s *StateStore) CreatePasswordSetup(ctx context.Context, username, sessionBinding string) (Flow, error) {
	return s.createBound(ctx, "", "password_setup", username, sessionBinding)
}

// CreatePasswordGrant creates a single-use grant after Telegram verification succeeds.
func (s *StateStore) CreatePasswordGrant(ctx context.Context, username, sessionBinding string) (Flow, error) {
	return s.createBound(ctx, "", "password_grant", username, sessionBinding)
}

func (s *StateStore) create(ctx context.Context, returnURL, purpose, username string) (Flow, error) {
	return s.createBound(ctx, returnURL, purpose, username, "")
}

func (s *StateStore) createBound(ctx context.Context, returnURL, purpose, username, sessionBinding string) (Flow, error) {
	nonce, err := s.randomValue()
	if err != nil {
		return Flow{}, err
	}

	verifier, err := s.randomValue()
	if err != nil {
		return Flow{}, err
	}

	publicID, err := uuid.NewRandomFromReader(s.random)
	if err != nil {
		return Flow{}, err
	}
	expiresAt := s.now().Add(s.ttl)
	if s.replay == nil {
		return Flow{}, errors.New("Telegram state replay store is not configured")
	}
	if err = s.replay.PurgeTelegramStates(ctx, s.now()); err != nil {
		return Flow{}, err
	}
	flow := Flow{
		Nonce:          nonce,
		CodeVerifier:   verifier,
		ReturnURL:      returnURL,
		Purpose:        purpose,
		Username:       username,
		SessionBinding: sessionBinding,
		ExpiresAt:      expiresAt,
	}

	payload, err := json.Marshal(flow)
	if err != nil {
		return Flow{}, err
	}
	replayKey, err := s.replay.SaveOneTimeCode(ctx, model.OneTimeCode{
		PublicID: publicID, IssuedAt: s.now(), IssuedIP: model.NewIP(net.IPv4zero), ExpiresAt: expiresAt,
		Username: "telegram", Intent: "telegram_state", Code: payload,
	})
	if err != nil {
		return Flow{}, err
	}
	flow.State = publicID.String()
	flow.ReplayKey = replayKey

	return flow, nil
}

// Inspect loads and validates a flow without consuming it.
func (s *StateStore) Inspect(ctx context.Context, state string) (Flow, error) {
	publicID, err := uuid.Parse(state)
	if err != nil {
		return Flow{}, ErrInvalidState
	}
	code, err := s.replay.LoadOneTimeCodeByPublicID(ctx, publicID)
	if err != nil {
		return Flow{}, err
	}
	if code == nil || code.Username != "telegram" || code.Intent != "telegram_state" {
		return Flow{}, ErrInvalidState
	}
	if s.now().After(code.ExpiresAt) {
		return Flow{State: state, ReplayKey: code.Signature, ExpiresAt: code.ExpiresAt}, ErrExpiredState
	}
	flow := Flow{State: state, ReplayKey: code.Signature}
	if err = json.Unmarshal(code.Code, &flow); err != nil {
		return Flow{}, ErrInvalidState
	}
	if s.now().After(flow.ExpiresAt) {
		return flow, ErrExpiredState
	}

	return flow, nil
}

// Consume atomically consumes and returns a valid flow as single use across all instances sharing storage.
func (s *StateStore) Consume(ctx context.Context, state string) (Flow, error) {
	flow, err := s.Inspect(ctx, state)
	if err != nil {
		return Flow{}, err
	}
	consumed, err := s.replay.ConsumeTelegramState(ctx, flow.ReplayKey, s.now())
	if err != nil {
		return Flow{}, err
	}
	if !consumed {
		return Flow{}, ErrInvalidState
	}
	return flow, nil
}

func (s *StateStore) randomValue() (string, error) {
	value := make([]byte, 32)
	if _, err := io.ReadFull(s.random, value); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(value), nil
}

// PKCEChallenge returns the RFC 7636 S256 challenge for a verifier.
func PKCEChallenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(digest[:])
}
