package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

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

type handlerPasswordProofStore struct {
	identity model.MTLUserIdentity
}

func (s *handlerPasswordProofStore) LoadMTLUserIdentity(context.Context, string, string) (model.MTLUserIdentity, bool, error) {
	return s.identity, true, nil
}
