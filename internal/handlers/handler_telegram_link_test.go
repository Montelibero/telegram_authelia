package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/authorization"
	"github.com/authelia/authelia/v4/internal/mocks"
	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/session"
	"github.com/authelia/authelia/v4/internal/telegram"
)

func TestTelegramLinkHandlersBindCurrentUser(t *testing.T) {
	mock := mocks.NewMockAutheliaCtxWithUserSession(t, session.UserSession{Username: "bublik", CookieDomain: "example.com", AuthenticationMethodRefs: authorization.AuthenticationMethodsReferences{External: true}})
	defer mock.Close()
	mock.Ctx.Providers.Clock = &mock.Clock
	mock.Ctx.Configuration.IdentityValidation.ElevatedSession.DisableOneTimeCode = true
	mock.Ctx.Configuration.IdentityValidation.ElevatedSession.ElevationLifespan = time.Minute
	mock.Ctx.Request.Header.Set(fasthttp.HeaderXForwardedFor, "127.0.0.1")
	var sessionCookie fasthttp.Cookie
	require.NoError(t, sessionCookie.ParseBytes(mock.Ctx.Response.Header.PeekCookie("authelia_session")))
	mock.Ctx.Request.Header.SetCookie(string(sessionCookie.Key()), string(sessionCookie.Value()))
	mock.Ctx.Response.Reset()
	client := &handlerTelegramClient{identity: telegram.Identity{ProviderUserID: "987654321", Username: "bublik_tg"}}
	store := &handlerIdentityLinkStore{}
	states := telegram.NewStateStore(time.Minute, mock.Clock.Now, nil, []byte("test secret"), newHandlerStateReplayStore())
	mock.Ctx.Providers.Telegram = telegram.NewLoginService(client, states, &handlerTelegramStore{})
	mock.Ctx.Providers.TelegramLink = telegram.NewLinkService(client, states, store)
	mock.Ctx.Request.SetRequestURI("https://login.example.com:8080/api/telegram/link")
	require.NotEmpty(t, mock.Ctx.Request.Header.Cookie("authelia_session"))
	initialSession, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	require.Equal(t, "bublik", initialSession.Username)
	initialSession.FirstFactorAuthnTimestamp = mock.Clock.Now().Unix()
	require.NoError(t, mock.Ctx.SaveSession(initialSession))

	TelegramLinkGET(mock.Ctx)
	require.Equal(t, fasthttp.StatusFound, mock.Ctx.Response.StatusCode())
	require.Equal(t, "bublik", client.flow.Username)
	var stateCookie fasthttp.Cookie
	require.NoError(t, stateCookie.ParseBytes(mock.Ctx.Response.Header.PeekCookie(telegramStateCookieName(client.flow.State))))
	mock.Ctx.Request.Header.SetCookie(string(stateCookie.Key()), string(stateCookie.Value()))

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetRequestURI("https://login.example.com:8080/api/telegram/callback?state=" + client.flow.State + "&code=code")
	callbackSession, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	require.Equal(t, "bublik", callbackSession.Username)
	TelegramCallbackGET(mock.Ctx)
	require.Equal(t, fasthttp.StatusFound, mock.Ctx.Response.StatusCode(), mock.LogEntryN(0))
	assert.Equal(t, "bublik", store.linkedUsername)

	mock.Ctx.Response.Reset()
	TelegramUnlinkDELETE(mock.Ctx)
	assert.Equal(t, fasthttp.StatusNoContent, mock.Ctx.Response.StatusCode())
	assert.Equal(t, "bublik", store.unlinkedUsername)
}

func TestTelegramLinkStatusReturnsCurrentUsersIdentity(t *testing.T) {
	mock := mocks.NewMockAutheliaCtxWithUserSession(t, session.UserSession{Username: "bublik", CookieDomain: "example.com"})
	defer mock.Close()
	providerUsername := "bublik_tg"
	store := &handlerIdentityLinkStore{identity: model.MTLUserIdentity{ProviderUserID: "987654321", ProviderUsername: &providerUsername}, found: true}
	mock.Ctx.Providers.TelegramLink = telegram.NewLinkService(&handlerTelegramClient{}, telegram.NewStateStore(time.Minute, nil, nil, []byte("test secret"), newHandlerStateReplayStore()), store)

	TelegramLinkStatusGET(mock.Ctx)

	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.JSONEq(t, `{"status":"OK","data":{"linked":true,"provider_user_id":"987654321","provider_username":"bublik_tg"}}`, string(mock.Ctx.Response.Body()))
}

type handlerIdentityLinkStore struct {
	linkedUsername   string
	unlinkedUsername string
	identity         model.MTLUserIdentity
	found            bool
}

func (s *handlerIdentityLinkStore) LoadMTLUserIdentity(_ context.Context, username, provider string) (model.MTLUserIdentity, bool, error) {
	return s.identity, s.found, nil
}

func (s *handlerIdentityLinkStore) LinkMTLUserIdentity(_ context.Context, username, provider, providerUserID, providerUsername string) error {
	s.linkedUsername = username
	return nil
}

func (s *handlerIdentityLinkStore) UnlinkMTLUserIdentity(_ context.Context, username, provider string) error {
	s.unlinkedUsername = username
	return nil
}
