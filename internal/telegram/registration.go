package telegram

import (
	"context"
	"strings"

	"github.com/authelia/authelia/v4/internal/model"
)

// RegistrationStore records verified external identities awaiting approval.
type RegistrationStore interface {
	UpsertMTLRegistration(ctx context.Context, candidate model.MTLRegistrationCandidate) (model.MTLRegistrationRequest, error)
}

// RegistrationService converts a verified Telegram identity into a registration candidate.
type RegistrationService struct {
	store                RegistrationStore
	generatedEmailDomain string
}

// NewRegistrationService constructs a pending-registration service.
func NewRegistrationService(store RegistrationStore, generatedEmailDomain string) *RegistrationService {
	return &RegistrationService{store: store, generatedEmailDomain: generatedEmailDomain}
}

// Register creates or refreshes the request for a stable Telegram identity.
func (s *RegistrationService) Register(ctx context.Context, identity Identity) (model.MTLRegistrationRequest, error) {
	username := strings.TrimPrefix(strings.TrimSpace(identity.Username), "@")
	email := ""
	if identity.EmailVerified && strings.TrimSpace(identity.Email) != "" {
		email = strings.TrimSpace(identity.Email)
	} else if username != "" && strings.TrimSpace(s.generatedEmailDomain) != "" {
		email = username + "@" + strings.TrimSpace(s.generatedEmailDomain)
	}

	return s.store.UpsertMTLRegistration(ctx, model.MTLRegistrationCandidate{
		Provider:         "telegram",
		ProviderUserID:   identity.ProviderUserID,
		ProviderUsername: username,
		DisplayName:      identity.Name,
		ProposedUsername: username,
		ProposedEmail:    email,
	})
}
