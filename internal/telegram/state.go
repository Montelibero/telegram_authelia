package telegram

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
	State        string `json:"-"`
	Nonce        string
	CodeVerifier string
	ReturnURL    string
	Purpose      string
	Username     string
	ExpiresAt    time.Time
}

// StateStore seals short-lived OIDC flows and tracks local replay attempts.
type StateStore struct {
	mu       sync.Mutex
	consumed map[[sha256.Size]byte]time.Time
	ttl      time.Duration
	now      func() time.Time
	random   io.Reader
	aead     cipher.AEAD
}

// NewStateStore constructs a state store. Nil clock and random sources use production defaults.
func NewStateStore(ttl time.Duration, now func() time.Time, random io.Reader, secret []byte) *StateStore {
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

	return &StateStore{consumed: map[[sha256.Size]byte]time.Time{}, ttl: ttl, now: now, random: random, aead: aead}
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
	nonce, err := s.randomValue()
	if err != nil {
		return Flow{}, err
	}

	verifier, err := s.randomValue()
	if err != nil {
		return Flow{}, err
	}

	flow := Flow{
		Nonce:        nonce,
		CodeVerifier: verifier,
		ReturnURL:    returnURL,
		Purpose:      purpose,
		Username:     username,
		ExpiresAt:    s.now().Add(s.ttl),
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
func (s *StateStore) Consume(state string) (Flow, error) {
	fingerprint := sha256.Sum256([]byte(state))
	s.mu.Lock()
	_, alreadyConsumed := s.consumed[fingerprint]
	s.mu.Unlock()
	if alreadyConsumed {
		return Flow{}, ErrInvalidState
	}

	flow, err := s.Inspect(state)
	if err != nil {
		if errors.Is(err, ErrExpiredState) {
			s.mu.Lock()
			s.consumed[fingerprint] = s.now().Add(s.ttl)
			s.mu.Unlock()
		}
		return Flow{}, err
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for item, expires := range s.consumed {
		if now.After(expires) {
			delete(s.consumed, item)
		}
	}
	if _, ok := s.consumed[fingerprint]; ok {
		return Flow{}, ErrInvalidState
	}
	s.consumed[fingerprint] = flow.ExpiresAt
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
