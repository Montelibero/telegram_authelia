package telegram

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
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
	State        string `json:"-"`
	Nonce        string
	CodeVerifier string
	ReturnURL    string
	Purpose      string
	Username     string
	ReplayKey    string
	ExpiresAt    time.Time
}

// StateReplayStore persists and atomically consumes state replay markers.
type StateReplayStore interface {
	SaveOneTimeCode(ctx context.Context, code model.OneTimeCode) (signature string, err error)
	ConsumeTelegramState(ctx context.Context, signature string, consumedAt time.Time) (consumed bool, err error)
}

// StateStore seals short-lived OIDC flows and tracks local replay attempts.
type StateStore struct {
	ttl    time.Duration
	now    func() time.Time
	random io.Reader
	aead   cipher.AEAD
	replay StateReplayStore
}

// NewStateStore constructs a state store. Nil clock and random sources use production defaults.
func NewStateStore(ttl time.Duration, now func() time.Time, random io.Reader, secret []byte, replay StateReplayStore) *StateStore {
	if now == nil {
		now = time.Now
	}

	if random == nil {
		random = rand.Reader
	}

	key := sha256.Sum256(secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		panic(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}

	return &StateStore{ttl: ttl, now: now, random: random, aead: aead, replay: replay}
}

// Create creates and stores a new Telegram OIDC flow.
func (s *StateStore) Create(ctx context.Context, returnURL string) (Flow, error) {
	return s.create(ctx, returnURL, "login", "")
}

// CreateLink creates a flow bound to account linking for one local username.
func (s *StateStore) CreateLink(ctx context.Context, username string) (Flow, error) {
	return s.create(ctx, "", "link", username)
}

func (s *StateStore) create(ctx context.Context, returnURL, purpose, username string) (Flow, error) {
	nonce, err := s.randomValue()
	if err != nil {
		return Flow{}, err
	}

	verifier, err := s.randomValue()
	if err != nil {
		return Flow{}, err
	}

	replayCode := make([]byte, 32)
	if _, err = io.ReadFull(s.random, replayCode); err != nil {
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
	replayKey, err := s.replay.SaveOneTimeCode(ctx, model.OneTimeCode{
		PublicID: publicID, IssuedAt: s.now(), IssuedIP: model.NewIP(net.IPv4zero), ExpiresAt: expiresAt,
		Username: "telegram", Intent: "telegram_state", Code: replayCode,
	})
	if err != nil {
		return Flow{}, err
	}

	flow := Flow{
		Nonce:        nonce,
		CodeVerifier: verifier,
		ReturnURL:    returnURL,
		Purpose:      purpose,
		Username:     username,
		ReplayKey:    replayKey,
		ExpiresAt:    expiresAt,
	}

	payload, err := json.Marshal(flow)
	if err != nil {
		return Flow{}, err
	}
	nonceBytes := make([]byte, s.aead.NonceSize())
	if _, err = io.ReadFull(s.random, nonceBytes); err != nil {
		return Flow{}, err
	}
	flow.State = base64.RawURLEncoding.EncodeToString(append(nonceBytes, s.aead.Seal(nil, nonceBytes, payload, nil)...))

	return flow, nil
}

// Inspect decrypts and validates a flow without consuming it.
func (s *StateStore) Inspect(state string) (Flow, error) {
	token, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil || len(token) <= s.aead.NonceSize() {
		return Flow{}, ErrInvalidState
	}
	payload, err := s.aead.Open(nil, token[:s.aead.NonceSize()], token[s.aead.NonceSize():], nil)
	if err != nil {
		return Flow{}, ErrInvalidState
	}
	flow := Flow{State: state}
	if err = json.Unmarshal(payload, &flow); err != nil {
		return Flow{}, ErrInvalidState
	}

	if s.now().After(flow.ExpiresAt) {
		return flow, ErrExpiredState
	}

	return flow, nil
}

// Consume atomically marks and returns a valid flow as single use in this process.
func (s *StateStore) Consume(ctx context.Context, state string) (Flow, error) {
	flow, err := s.Inspect(state)
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
