package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/authentication"
	"github.com/authelia/authelia/v4/internal/mocks"
	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/session"
	"github.com/authelia/authelia/v4/internal/telegram"
)

func TestTelegramPasswordProofHandlersBindCurrentSessionAndHideGrant(t *testing.T) {
	mock := mocks.NewMockAutheliaCtxWithUserSession(t, session.UserSession{Username: "bublik", CookieDomain: "example.com"})
	defer mock.Close()
	client := &handlerTelegramClient{identity: telegram.Identity{ProviderUserID: "42"}}
	store := &handlerPasswordProofStore{identity: model.MTLUserIdentity{ProviderUserID: "42"}}
	states := telegram.NewStateStore(time.Minute, nil, nil, []byte("test secret"), newHandlerStateReplayStore())
	mock.Ctx.Providers.TelegramPasswordProof = telegram.NewPasswordProofService(client, states, store)

	TelegramPasswordProofGET(mock.Ctx)
	require.Equal(t, fasthttp.StatusFound, mock.Ctx.Response.StatusCode())
	require.Equal(t, "password_setup", client.flow.Purpose)
	require.Equal(t, "bublik", client.flow.Username)
	var stateCookie fasthttp.Cookie
	require.NoError(t, stateCookie.ParseBytes(mock.Ctx.Response.Header.PeekCookie(telegramStateCookieName(client.flow.State))))
	mock.Ctx.Request.Header.SetCookie(string(stateCookie.Key()), string(stateCookie.Value()))

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetRequestURI("https://auth.example.com/api/telegram/callback?state=" + client.flow.State + "&code=code")
	TelegramCallbackGET(mock.Ctx)
	require.Equal(t, fasthttp.StatusFound, mock.Ctx.Response.StatusCode())
	assert.Equal(t, "https://login.example.com:8080/settings/security?telegram_password_setup=verified", string(mock.Ctx.Response.Header.Peek("Location")))
	grantCookie := mock.Ctx.Response.Header.PeekCookie(telegramPasswordGrantCookie)
	assert.NotEmpty(t, grantCookie)
	assert.NotContains(t, string(mock.Ctx.Response.Header.Peek("Location")), "password_grant")
}

func TestSelfServicePasswordRemovalPreservesCurrentSession(t *testing.T) {
	mock := mocks.NewMockAutheliaCtxWithUserSession(t, session.UserSession{Username: "bublik", CookieDomain: "example.com"})
	defer mock.Close()
	epoch := 9
	mock.Ctx.Providers.UserProvider = &handlerSelfServicePasswordProvider{details: &authentication.UserDetails{Username: "bublik", SessionEpoch: &epoch}}
	current, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	require.Equal(t, "bublik", current.Username)
	body, err := json.Marshal(selfServicePasswordRemoveRequest{CurrentPassword: "current", ExpectedVersion: 4})
	require.NoError(t, err)
	mock.Ctx.Request.SetBody(body)

	SelfServicePasswordDELETE(mock.Ctx)
	require.Equal(t, fasthttp.StatusNoContent, mock.Ctx.Response.StatusCode())
	updated, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	assert.Equal(t, &epoch, updated.SessionEpoch)
}

type handlerSelfServicePasswordProvider struct {
	authentication.UserProvider
	details *authentication.UserDetails
}

func (p *handlerSelfServicePasswordProvider) SetPasswordFromProof(string, string) (*authentication.UserDetails, error) {
	return p.details, nil
}

func (p *handlerSelfServicePasswordProvider) RemovePassword(string, string, int) (*authentication.UserDetails, error) {
	return p.details, nil
}

type handlerPasswordProofStore struct {
	identity model.MTLUserIdentity
}

func (s *handlerPasswordProofStore) LoadMTLUserIdentity(context.Context, string, string) (model.MTLUserIdentity, bool, error) {
	return s.identity, true, nil
}
