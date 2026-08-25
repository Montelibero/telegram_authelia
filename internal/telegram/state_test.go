package telegram

import (
	"bytes"
	"context"
	"encoding/base64"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/model"
)

type fakeStateReplayStore struct {
	mu       sync.Mutex
	consumed map[string]bool
	next     int
}

func newFakeStateReplayStore() *fakeStateReplayStore {
	return &fakeStateReplayStore{consumed: map[string]bool{}}
}

func (s *fakeStateReplayStore) SaveOneTimeCode(_ context.Context, code model.OneTimeCode) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	return base64.RawURLEncoding.EncodeToString(code.Code) + strconv.Itoa(s.next), nil
}

func (s *fakeStateReplayStore) ConsumeTelegramState(_ context.Context, signature string, _ time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.consumed[signature] {
		return false, nil
	}
	s.consumed[signature] = true
	return true, nil
}

func TestStateStoreCreatesAndConsumesSingleUseFlow(t *testing.T) {
	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	store := NewStateStore(5*time.Minute, func() time.Time { return now }, bytes.NewReader(bytes.Repeat([]byte{0x42}, 512)), []byte("test secret"), newFakeStateReplayStore())

	flow, err := store.Create(context.Background(), "https://app.example.com/dashboard")
	require.NoError(t, err)
	assert.NotEmpty(t, flow.State)
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

func TestStateStoreRejectsTokenEncryptedWithAnotherKey(t *testing.T) {
	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	replay := newFakeStateReplayStore()
	creator := NewStateStore(5*time.Minute, func() time.Time { return now }, bytes.NewReader(bytes.Repeat([]byte{0x42}, 512)), []byte("first secret"), replay)
	consumer := NewStateStore(5*time.Minute, func() time.Time { return now }, bytes.NewReader(bytes.Repeat([]byte{0x24}, 512)), []byte("second secret"), replay)

	flow, err := creator.Create(context.Background(), "")
	require.NoError(t, err)

	_, err = consumer.Consume(context.Background(), flow.State)
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestPKCEChallengeUsesS256(t *testing.T) {
	assert.Equal(t, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", PKCEChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"))
}
