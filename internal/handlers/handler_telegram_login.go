package handlers

import (
	"errors"

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

	authorizationURL, _, err := ctx.Providers.Telegram.Begin(string(ctx.QueryArgs().Peek("rd")))
	if err != nil {
		ctx.Logger.WithError(err).Warn("Failed to start Telegram login")
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	ctx.Redirect(authorizationURL, fasthttp.StatusFound)
}

// TelegramCallbackGET completes Telegram OIDC and creates a normal one-factor session.
func TelegramCallbackGET(ctx *middlewares.AutheliaCtx) {
	if ctx.Providers.Telegram == nil {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}

	result, err := ctx.Providers.Telegram.Complete(ctx, string(ctx.QueryArgs().Peek("state")), string(ctx.QueryArgs().Peek("code")))
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
