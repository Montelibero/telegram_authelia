package telegram

import (
	"bytes"
	"context"
	"encoding/base64"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/model"
)

type fakeStateReplayStore struct {
	mu       sync.Mutex
	consumed map[string]bool
	codes    map[uuid.UUID]model.OneTimeCode
	next     int
}

func newFakeStateReplayStore() *fakeStateReplayStore {
	return &fakeStateReplayStore{consumed: map[string]bool{}, codes: map[uuid.UUID]model.OneTimeCode{}}
}

func (s *fakeStateReplayStore) SaveOneTimeCode(_ context.Context, code model.OneTimeCode) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	code.Signature = base64.RawURLEncoding.EncodeToString(code.Code) + strconv.Itoa(s.next)
	s.codes[code.PublicID] = code
	return code.Signature, nil
}

func (s *fakeStateReplayStore) LoadOneTimeCodeByPublicID(_ context.Context, id uuid.UUID) (*model.OneTimeCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	code, ok := s.codes[id]
	if !ok {
		return nil, nil
	}
	return &code, nil
}

func (s *fakeStateReplayStore) ConsumeTelegramState(_ context.Context, signature string, _ time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.consumed[signature] {
		return false, nil
	}
	s.consumed[signature] = true
	for id, code := range s.codes {
		if code.Signature == signature {
			delete(s.codes, id)
		}
	}
	return true, nil
}

func (s *fakeStateReplayStore) PurgeTelegramStates(context.Context, time.Time) error { return nil }

func TestStateStoreCreatesAndConsumesSingleUseFlow(t *testing.T) {
	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	store := NewStateStore(5*time.Minute, func() time.Time { return now }, bytes.NewReader(bytes.Repeat([]byte{0x42}, 512)), []byte("test secret"), newFakeStateReplayStore())

	flow, err := store.Create(context.Background(), "https://app.example.com/dashboard")
	require.NoError(t, err)
	assert.NotEmpty(t, flow.State)
	assert.Len(t, flow.State, 36)
	assert.NotEmpty(t, flow.Nonce)
	assert.NotEmpty(t, flow.CodeVerifier)
	assert.Equal(t, now.Add(5*time.Minute), flow.ExpiresAt)
	assert.Equal(t, "https://app.example.com/dashboard", flow.ReturnURL)

	consumed, err := store.Consume(context.Background(), flow.State)
	require.NoError(t, err)
	assert.Equal(t, flow, consumed)

	_, err = store.Consume(context.Background(), flow.State)
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestStateStoreRejectsExpiredFlow(t *testing.T) {
	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	store := NewStateStore(time.Minute, func() time.Time { return now }, bytes.NewReader(bytes.Repeat([]byte{0x24}, 512)), []byte("test secret"), newFakeStateReplayStore())

	flow, err := store.Create(context.Background(), "")
	require.NoError(t, err)
	now = now.Add(time.Minute + time.Nanosecond)

	_, err = store.Consume(context.Background(), flow.State)
	assert.ErrorIs(t, err, ErrExpiredState)

	_, err = store.Consume(context.Background(), flow.State)
	assert.ErrorIs(t, err, ErrExpiredState)
}

func TestStateStoreCanConsumeAcrossInstances(t *testing.T) {
	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	key := []byte("shared telegram client secret")
	replay := newFakeStateReplayStore()
	creator := NewStateStore(5*time.Minute, func() time.Time { return now }, bytes.NewReader(bytes.Repeat([]byte{0x42}, 512)), key, replay)
	consumer := NewStateStore(5*time.Minute, func() time.Time { return now }, bytes.NewReader(bytes.Repeat([]byte{0x24}, 512)), key, replay)

	flow, err := creator.Create(context.Background(), "/dashboard")
	require.NoError(t, err)

	consumed, err := consumer.Consume(context.Background(), flow.State)
	require.NoError(t, err)
	assert.Equal(t, "/dashboard", consumed.ReturnURL)
	assert.Equal(t, "login", consumed.Purpose)
	_, err = creator.Consume(context.Background(), flow.State)
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestStateStoreCreatesPasswordSetupAndGrantFlows(t *testing.T) {
	store := NewStateStore(time.Minute, time.Now, bytes.NewReader(bytes.Repeat([]byte{0x31}, 1024)), []byte("test secret"), newFakeStateReplayStore())

	setup, err := store.CreatePasswordSetup(context.Background(), "bublik", "session-a")
	require.NoError(t, err)
	assert.Equal(t, "password_setup", setup.Purpose)
	assert.Equal(t, "bublik", setup.Username)
	assert.Equal(t, "session-a", setup.SessionBinding)

	grant, err := store.CreatePasswordGrant(context.Background(), "bublik", "session-a")
	require.NoError(t, err)
	assert.Equal(t, "password_grant", grant.Purpose)
	assert.Equal(t, "bublik", grant.Username)
	assert.Equal(t, "session-a", grant.SessionBinding)
}

func TestPKCEChallengeUsesS256(t *testing.T) {
	assert.Equal(t, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", PKCEChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"))
}
