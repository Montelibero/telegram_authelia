package telegram

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"sync"
	"time"
)

var (
	// ErrInvalidState indicates the state is unknown or was already consumed.
	ErrInvalidState = errors.New("invalid Telegram authentication state")
	// ErrExpiredState indicates the state expired before it was consumed.
	ErrExpiredState = errors.New("expired Telegram authentication state")
)

// Flow binds an OIDC request to its state, nonce, PKCE verifier, and return URL.
type Flow struct {
	State        string
	Nonce        string
	CodeVerifier string
	ReturnURL    string
	Purpose      string
	Username     string
	ExpiresAt    time.Time
}

// StateStore stores short-lived, single-use OIDC flows in memory.
type StateStore struct {
	mu     sync.Mutex
	flows  map[string]Flow
	ttl    time.Duration
	now    func() time.Time
	random io.Reader
}

// NewStateStore constructs a state store. Nil clock and random sources use production defaults.
func NewStateStore(ttl time.Duration, now func() time.Time, random io.Reader) *StateStore {
	if now == nil {
		now = time.Now
	}

	if random == nil {
		random = rand.Reader
	}

	return &StateStore{flows: map[string]Flow{}, ttl: ttl, now: now, random: random}
}

// Create creates and stores a new Telegram OIDC flow.
func (s *StateStore) Create(returnURL string) (Flow, error) {
	return s.create(returnURL, "login", "")
}

// CreateLink creates a flow bound to account linking for one local username.
func (s *StateStore) CreateLink(username string) (Flow, error) {
	return s.create("", "link", username)
}

func (s *StateStore) create(returnURL, purpose, username string) (Flow, error) {
	state, err := s.randomValue()
	if err != nil {
		return Flow{}, err
	}

	nonce, err := s.randomValue()
	if err != nil {
		return Flow{}, err
	}

	verifier, err := s.randomValue()
	if err != nil {
		return Flow{}, err
	}

	flow := Flow{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: verifier,
		ReturnURL:    returnURL,
		Purpose:      purpose,
		Username:     username,
		ExpiresAt:    s.now().Add(s.ttl),
	}

	s.mu.Lock()
	s.flows[state] = flow
	s.mu.Unlock()

	return flow, nil
}

// Consume atomically removes and returns a flow.
func (s *StateStore) Consume(state string) (Flow, error) {
	s.mu.Lock()
	flow, ok := s.flows[state]
	delete(s.flows, state)
	s.mu.Unlock()

	if !ok {
		return Flow{}, ErrInvalidState
	}

	if s.now().After(flow.ExpiresAt) {
		return Flow{}, ErrExpiredState
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
