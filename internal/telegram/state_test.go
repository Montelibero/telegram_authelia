package telegram

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateStoreCreatesAndConsumesSingleUseFlow(t *testing.T) {
	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	store := NewStateStore(5*time.Minute, func() time.Time { return now }, bytes.NewReader(bytes.Repeat([]byte{0x42}, 128)))

	flow, err := store.Create("https://app.example.com/dashboard")
	require.NoError(t, err)
	assert.NotEmpty(t, flow.State)
	assert.NotEmpty(t, flow.Nonce)
	assert.NotEmpty(t, flow.CodeVerifier)
	assert.Equal(t, now.Add(5*time.Minute), flow.ExpiresAt)
	assert.Equal(t, "https://app.example.com/dashboard", flow.ReturnURL)

	consumed, err := store.Consume(flow.State)
	require.NoError(t, err)
	assert.Equal(t, flow, consumed)

	_, err = store.Consume(flow.State)
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestStateStoreRejectsExpiredFlow(t *testing.T) {
	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	store := NewStateStore(time.Minute, func() time.Time { return now }, bytes.NewReader(bytes.Repeat([]byte{0x24}, 128)))

	flow, err := store.Create("")
	require.NoError(t, err)
	now = now.Add(time.Minute + time.Nanosecond)

	_, err = store.Consume(flow.State)
	assert.ErrorIs(t, err, ErrExpiredState)

	_, err = store.Consume(flow.State)
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestPKCEChallengeUsesS256(t *testing.T) {
	assert.Equal(t, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", PKCEChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"))
}
