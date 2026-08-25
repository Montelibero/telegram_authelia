package handlers

import (
	"path"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/authentication"
	"github.com/authelia/authelia/v4/internal/middlewares"
)

const telegramPasswordGrantCookie = "authelia_telegram_password_grant"

type selfServicePasswordSetRequest struct {
	NewPassword string `json:"new_password" validate:"required"`
}

func TelegramPasswordProofGET(ctx *middlewares.AutheliaCtx) {
	userSession, err := ctx.GetSession()
	if err != nil || userSession.Username == "" || ctx.Providers.TelegramPasswordProof == nil {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	authorizationURL, state, err := ctx.Providers.TelegramPasswordProof.Begin(ctx, userSession.Username)
	if err != nil {
		ctx.Logger.WithError(err).Warn("Failed to start Telegram password proof")
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}
	setTelegramStateCookie(ctx, state)
	ctx.Redirect(authorizationURL, fasthttp.StatusFound)
}

func TelegramPasswordProofCallbackGET(ctx *middlewares.AutheliaCtx) {
	userSession, err := ctx.GetSession()
	if err != nil || userSession.Username == "" || ctx.Providers.TelegramPasswordProof == nil {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	grant, err := ctx.Providers.TelegramPasswordProof.Complete(ctx, userSession.Username, string(ctx.QueryArgs().Peek("state")), string(ctx.QueryArgs().Peek("code")))
	if err != nil {
		ctx.Logger.WithError(err).Warn("Telegram password proof failed")
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	setTelegramPasswordGrantCookie(ctx, grant)
	returnURL := ctx.TemplateRootURL()
	returnURL.Path = path.Join(returnURL.Path, "settings/security")
	query := returnURL.Query()
	query.Set("telegram_password_setup", "verified")
	returnURL.RawQuery = query.Encode()
	ctx.Redirect(returnURL.String(), fasthttp.StatusFound)
}

func SelfServicePasswordSetPOST(ctx *middlewares.AutheliaCtx) {
	userSession, err := ctx.GetSession()
	if err != nil || userSession.Username == "" || ctx.Providers.TelegramPasswordProof == nil {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	grant := string(ctx.Request.Header.Cookie(telegramPasswordGrantCookie))
	var body selfServicePasswordSetRequest
	if err = ctx.ParseBody(&body); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}
	clearTelegramPasswordGrantCookie(ctx)
	if err = ctx.Providers.TelegramPasswordProof.Consume(ctx, userSession.Username, grant); err != nil {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	if err = ctx.Providers.PasswordPolicy.Check(body.NewPassword); err != nil {
		ctx.SetJSONError(messagePasswordWeak)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}
	provider, ok := ctx.Providers.UserProvider.(authentication.SelfServicePasswordProvider)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusNotImplemented)
		return
	}
	details, err := provider.SetPasswordFromProof(userSession.Username, body.NewPassword)
	if err != nil {
		ctx.Logger.WithError(err).Warn("Failed to set self-service password")
		ctx.SetStatusCode(fasthttp.StatusConflict)
		return
	}
	userSession.SessionEpoch = details.SessionEpoch
	if err = ctx.SaveSession(userSession); err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func setTelegramPasswordGrantCookie(ctx *middlewares.AutheliaCtx, grant string) {
	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)
	cookie.SetKey(telegramPasswordGrantCookie)
	cookie.SetValue(grant)
	cookie.SetPath("/")
	cookie.SetHTTPOnly(true)
	cookie.SetSecure(true)
	cookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	cookie.SetMaxAge(int((5 * time.Minute).Seconds()))
	ctx.Response.Header.SetCookie(cookie)
}

func clearTelegramPasswordGrantCookie(ctx *middlewares.AutheliaCtx) {
	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)
	cookie.SetKey(telegramPasswordGrantCookie)
	cookie.SetPath("/")
	cookie.SetExpire(time.Unix(1, 0))
	ctx.Response.Header.SetCookie(cookie)
}
