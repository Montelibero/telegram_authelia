package handlers

import (
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/middlewares"
)

// TelegramLinkGET starts a Telegram linking flow for the current elevated user.
func TelegramLinkGET(ctx *middlewares.AutheliaCtx) {
	userSession, err := ctx.GetSession()
	if err != nil || ctx.Providers.TelegramLink == nil {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	authorizationURL, _, err := ctx.Providers.TelegramLink.Begin(userSession.Username)
	if err != nil {
		ctx.Logger.WithError(err).Warn("Failed to start Telegram account linking")
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}
	ctx.Redirect(authorizationURL, fasthttp.StatusFound)
}

// TelegramLinkCallbackGET verifies and links Telegram to the current elevated user.
func TelegramLinkCallbackGET(ctx *middlewares.AutheliaCtx) {
	userSession, err := ctx.GetSession()
	if err != nil || ctx.Providers.TelegramLink == nil {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	if err = ctx.Providers.TelegramLink.Complete(ctx, userSession.Username, string(ctx.QueryArgs().Peek("state")), string(ctx.QueryArgs().Peek("code"))); err != nil {
		ctx.Logger.WithError(err).Warn("Telegram account linking failed")
		ctx.SetStatusCode(fasthttp.StatusConflict)
		return
	}
	ctx.Redirect("/", fasthttp.StatusFound)
}

// TelegramUnlinkDELETE removes Telegram from the exact current elevated user.
func TelegramUnlinkDELETE(ctx *middlewares.AutheliaCtx) {
	userSession, err := ctx.GetSession()
	if err != nil || ctx.Providers.TelegramLink == nil {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	if err = ctx.Providers.TelegramLink.Unlink(ctx, userSession.Username); err != nil {
		ctx.Logger.WithError(err).Warn("Telegram account unlinking failed")
		ctx.SetStatusCode(fasthttp.StatusConflict)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
