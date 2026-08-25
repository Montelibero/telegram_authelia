package handlers

import (
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/middlewares"
)

type adminStatusResponse struct {
	Username      string `json:"username"`
	PasswordFresh bool   `json:"password_fresh"`
}

// AdminGET returns the safe capabilities of the current administrative session.
func AdminGET(ctx *middlewares.AutheliaCtx) {
	userSession, err := ctx.GetSession()
	if err != nil {
		ctx.ReplyUnauthorized()
		return
	}
	now := ctx.GetClock().Now()
	passwordProof := userSession.GetFirstFactorAuthn()
	lifespan := ctx.Configuration.IdentityValidation.ElevatedSession.ElevationLifespan
	passwordFresh := userSession.AuthenticationMethodRefs.UsernameAndPassword && lifespan > 0 && !passwordProof.IsZero() && !passwordProof.After(now) && now.Sub(passwordProof) <= lifespan
	if err = ctx.ReplyJSON(middlewares.OKResponse{Status: "OK", Data: adminStatusResponse{
		Username:      userSession.Username,
		PasswordFresh: passwordFresh,
	}}, fasthttp.StatusOK); err != nil {
		ctx.Logger.WithError(err).Error("Error occurred encoding the admin session response.")
		ctx.ReplyForbidden()
	}
}
