package handlers

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/authentication"
	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/telegram"
)

// TelegramLoginGET starts a Telegram OIDC flow.
func TelegramLoginGET(ctx *middlewares.AutheliaCtx) {
	if ctx.Providers.Telegram == nil {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}

	authorizationURL, state, err := ctx.Providers.Telegram.Begin(ctx, string(ctx.QueryArgs().Peek("rd")))
	if err != nil {
		ctx.Logger.WithError(err).Warn("Failed to start Telegram login")
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}
	setTelegramStateCookie(ctx, state)

	ctx.Redirect(authorizationURL, fasthttp.StatusFound)
}

// TelegramCallbackGET completes Telegram OIDC and creates a normal one-factor session.
func TelegramCallbackGET(ctx *middlewares.AutheliaCtx) {
	if ctx.Providers.Telegram == nil && ctx.Providers.TelegramLink == nil {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}

	state := string(ctx.QueryArgs().Peek("state"))
	if !validTelegramStateCookie(ctx, state) {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}
	clearTelegramStateCookie(ctx, state)

	var purpose string
	var err error
	if ctx.Providers.Telegram != nil {
		purpose, err = ctx.Providers.Telegram.Purpose(state)
	} else {
		purpose, err = ctx.Providers.TelegramLink.Purpose(state)
	}
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}
	if purpose == "link" {
		middlewares.RequireFreshPasswordElevation(TelegramLinkCallbackGET)(ctx)
		return
	}
	if purpose != "login" || ctx.Providers.Telegram == nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	result, err := ctx.Providers.Telegram.Complete(ctx, state, string(ctx.QueryArgs().Peek("code")))
	if err != nil {
		ctx.Logger.WithError(err).Warn("Telegram login failed")
		switch {
		case errors.Is(err, telegram.ErrIdentityNotLinked), errors.Is(err, telegram.ErrUserDisabled):
			ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		case errors.Is(err, telegram.ErrInvalidState), errors.Is(err, telegram.ErrExpiredState):
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
		default:
			ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		}
		return
	}

	details := &authentication.UserDetails{
		Username:    result.Details.User.Username,
		DisplayName: result.Details.User.DisplayName,
		Emails:      []string{result.Details.PrimaryEmail},
		Groups:      result.Details.Groups,
	}
	provider, err := ctx.GetSessionProvider()
	if err != nil {
		ctx.Logger.WithError(err).Error("Failed to get session provider during Telegram login")
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	_ = provider.DestroySession(ctx.RequestCtx)
	userSession := provider.NewDefaultUserSession()
	if err = provider.SaveSession(ctx.RequestCtx, userSession); err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}
	if err = provider.RegenerateSession(ctx.RequestCtx); err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	userSession.SetOneFactorExternal(ctx.GetClock().Now(), details)
	if err = provider.SaveSession(ctx.RequestCtx, userSession); err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	returnURL := result.ReturnURL
	if returnURL == "" {
		returnURL = "/"
	}
	ctx.Redirect(returnURL, fasthttp.StatusFound)
}

func telegramStateCookieName(state string) string {
	digest := sha256.Sum256([]byte(state))
	return "authelia_telegram_state_" + hex.EncodeToString(digest[:8])
}

func setTelegramStateCookie(ctx *middlewares.AutheliaCtx, state string) {
	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)
	cookie.SetKey(telegramStateCookieName(state))
	cookie.SetValue(state)
	cookie.SetPath("/")
	cookie.SetHTTPOnly(true)
	cookie.SetSecure(true)
	cookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	cookie.SetMaxAge(int((5 * time.Minute).Seconds()))
	ctx.Response.Header.SetCookie(cookie)
}

func validTelegramStateCookie(ctx *middlewares.AutheliaCtx, state string) bool {
	value := ctx.Request.Header.Cookie(telegramStateCookieName(state))
	return len(value) == len(state) && subtle.ConstantTimeCompare(value, []byte(state)) == 1
}

func clearTelegramStateCookie(ctx *middlewares.AutheliaCtx, state string) {
	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)
	cookie.SetKey(telegramStateCookieName(state))
	cookie.SetPath("/")
	cookie.SetExpire(time.Unix(1, 0))
	ctx.Response.Header.SetCookie(cookie)
}
