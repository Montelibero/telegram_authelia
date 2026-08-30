package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/mocks"
	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/session"
	"github.com/authelia/authelia/v4/internal/storage"
)

func TestSelfServiceProfileReadAndUpdate(t *testing.T) {
	mock := mocks.NewMockAutheliaCtxWithUserSession(t, session.UserSession{Username: "bublik", DisplayName: "Old", CookieDomain: "example.com"})
	defer mock.Close()
	store := &handlerSelfServiceProfileStore{details: model.MTLAdminUserDetails{
		MTLAdminUserSummary: model.MTLAdminUserSummary{Username: "bublik", DisplayName: "Old", Status: model.MTLUserStatusActive, Version: 3, PasswordEnabled: true},
		Identities:          []model.MTLUserIdentity{{Provider: "telegram", ProviderUserID: "42"}},
	}}
	mock.Ctx.Providers.StorageProvider = store
	current, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	require.Equal(t, "bublik", current.Username)

	SelfServiceProfileGET(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.Contains(t, string(mock.Ctx.Response.Body()), `"telegram_linked":true`)

	mock.Ctx.Response.Reset()
	body, err := json.Marshal(model.MTLSelfServiceProfileUpdate{ExpectedVersion: 3, DisplayName: "New Name"})
	require.NoError(t, err)
	mock.Ctx.Request.SetBody(body)
	SelfServiceProfilePATCH(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.Equal(t, "New Name", store.details.DisplayName)
	updated, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.DisplayName)
}

type handlerSelfServiceProfileStore struct {
	storage.Provider
	details model.MTLAdminUserDetails
}

func (s *handlerSelfServiceProfileStore) LoadMTLAdminUser(context.Context, string) (model.MTLAdminUserDetails, bool, error) {
	return s.details, true, nil
}

func (s *handlerSelfServiceProfileStore) UpdateMTLAdminUser(_ context.Context, _ string, update model.MTLAdminUserUpdate, _ string) (model.MTLAdminUserDetails, error) {
	s.details.DisplayName = update.DisplayName
	s.details.Version++
	return s.details, nil
}
