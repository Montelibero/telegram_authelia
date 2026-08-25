package telegram

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/model"
)

func TestRegistrationServiceCreatesPendingCandidate(t *testing.T) {
	store := &fakeRegistrationStore{}
	service := NewRegistrationService(store, "eurmtl.me")

	request, err := service.Register(context.Background(), Identity{
		ProviderUserID: "987654321", Username: "bublik", Name: "Bublik",
	})
	require.NoError(t, err)
	assert.Equal(t, model.MTLRegistrationStatusPending, request.Status)
	assert.Equal(t, "bublik", store.candidate.ProposedUsername)
	assert.Equal(t, "bublik@eurmtl.me", store.candidate.ProposedEmail)
}

func TestRegistrationServiceUsesVerifiedProviderEmail(t *testing.T) {
	store := &fakeRegistrationStore{}
	service := NewRegistrationService(store, "eurmtl.me")

	_, err := service.Register(context.Background(), Identity{
		ProviderUserID: "1", Username: "bublik", Email: "person@example.com", EmailVerified: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "person@example.com", store.candidate.ProposedEmail)
}

func TestRegistrationServiceKeepsMissingUsernameIncomplete(t *testing.T) {
	store := &fakeRegistrationStore{}
	service := NewRegistrationService(store, "eurmtl.me")

	_, err := service.Register(context.Background(), Identity{ProviderUserID: "1", Email: "unverified@example.com"})
	require.NoError(t, err)
	assert.Empty(t, store.candidate.ProposedUsername)
	assert.Empty(t, store.candidate.ProposedEmail)
}

type fakeRegistrationStore struct {
	candidate model.MTLRegistrationCandidate
	request   model.MTLRegistrationRequest
	err       error
}

func (s *fakeRegistrationStore) UpsertMTLRegistration(_ context.Context, candidate model.MTLRegistrationCandidate) (model.MTLRegistrationRequest, error) {
	s.candidate = candidate
	if s.request.Status == "" {
		s.request.Status = model.MTLRegistrationStatusPending
	}
	return s.request, s.err
}
