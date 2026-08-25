package handlers

import (
	"context"
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

func TestTelegramLoginHandlersCreateFederatedOneFactorSession(t *testing.T) {
	mock := mocks.NewMockAutheliaCtxWithUserSession(t, session.UserSession{})
	defer mock.Close()
	client := &handlerTelegramClient{identity: telegram.Identity{ProviderUserID: "987654321"}}
	store := &handlerTelegramStore{details: model.MTLUserDetails{User: model.MTLUser{Username: "bublik", DisplayName: "Bublik", Status: model.MTLUserStatusActive}, PrimaryEmail: "bublik@eurmtl.me", Groups: []string{"app:grafana"}}}
	mock.Ctx.Providers.Telegram = telegram.NewLoginService(client, telegram.NewStateStore(time.Minute, nil, nil, []byte("test secret")), store)
	mock.Ctx.Request.SetRequestURI("https://auth.example.com/api/telegram/login?rd=/portal")

	TelegramLoginGET(mock.Ctx)
	require.Equal(t, fasthttp.StatusFound, mock.Ctx.Response.StatusCode())
	require.NotEmpty(t, client.flow.State)
	var stateCookie fasthttp.Cookie
	require.NoError(t, stateCookie.ParseBytes(mock.Ctx.Response.Header.PeekCookie(telegramStateCookieName(client.flow.State))))
	assert.True(t, stateCookie.HTTPOnly())
	assert.True(t, stateCookie.Secure())
	mock.Ctx.Request.Header.SetCookie(string(stateCookie.Key()), string(stateCookie.Value()))

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetRequestURI("https://auth.example.com/api/telegram/callback?state=" + client.flow.State + "&code=code")
	TelegramCallbackGET(mock.Ctx)
	require.Equal(t, fasthttp.StatusFound, mock.Ctx.Response.StatusCode())
	assert.Equal(t, "https://auth.example.com/portal", string(mock.Ctx.Response.Header.Peek("Location")))

	userSession, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	assert.Equal(t, "bublik", userSession.Username)
	assert.Equal(t, authentication.OneFactor, userSession.AuthenticationLevel(false))
	assert.True(t, userSession.AuthenticationMethodRefs.External)
	assert.False(t, userSession.AuthenticationMethodRefs.UsernameAndPassword)
}

func TestTelegramCallbackRejectsFlowFromAnotherBrowser(t *testing.T) {
	mock := mocks.NewMockAutheliaCtxWithUserSession(t, session.UserSession{})
	defer mock.Close()
	client := &handlerTelegramClient{identity: telegram.Identity{ProviderUserID: "987654321"}}
	store := &handlerTelegramStore{details: model.MTLUserDetails{User: model.MTLUser{Username: "bublik", Status: model.MTLUserStatusActive}, PrimaryEmail: "bublik@eurmtl.me"}}
	mock.Ctx.Providers.Telegram = telegram.NewLoginService(client, telegram.NewStateStore(time.Minute, nil, nil, []byte("test secret")), store)
	mock.Ctx.Request.SetRequestURI("https://auth.example.com/api/telegram/login")
	TelegramLoginGET(mock.Ctx)

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetRequestURI("https://auth.example.com/api/telegram/callback?state=" + client.flow.State + "&code=code")
	TelegramCallbackGET(mock.Ctx)

	assert.Equal(t, fasthttp.StatusBadRequest, mock.Ctx.Response.StatusCode())
}

type handlerTelegramClient struct {
	flow     telegram.Flow
	identity telegram.Identity
}

func (c *handlerTelegramClient) AuthorizationURL(flow telegram.Flow) string {
	c.flow = flow
	return "https://oauth.telegram.org/auth"
}

func (c *handlerTelegramClient) Exchange(context.Context, string, telegram.Flow) (telegram.Identity, error) {
	return c.identity, nil
}

type handlerTelegramStore struct{ details model.MTLUserDetails }

func (s *handlerTelegramStore) LoadMTLUserByIdentity(context.Context, string, string) (model.MTLUserDetails, bool, error) {
	return s.details, true, nil
}
