package handlers

import (
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/middlewares"
)

type adminStatusResponse struct {
	Username          string   `json:"username"`
	MutationReady     bool     `json:"mutation_ready"`
	FullAdministrator bool     `json:"full_administrator"`
	ManagedGroups     []string `json:"managed_groups"`
}

// AdminGET returns the safe capabilities of the current administrative session.
func AdminGET(ctx *middlewares.AutheliaCtx) {
	userSession, err := ctx.GetSession()
	if err != nil {
		ctx.ReplyUnauthorized()
		return
	}
	recentFirstFactor := middlewares.IsRecentFirstFactorAuthentication(ctx, &userSession)
	mutationReady := recentFirstFactor && (ctx.Configuration.IdentityValidation.ElevatedSession.DisableOneTimeCode || userSession.AuthenticationMethodRefs.UsernameAndPassword)
	scope := loadAdminAccessScope(ctx)
	if err = ctx.ReplyJSON(middlewares.OKResponse{Status: "OK", Data: adminStatusResponse{
		Username:          userSession.Username,
		MutationReady:     mutationReady,
		FullAdministrator: scope.Full,
		ManagedGroups:     scope.ManagedGroups,
	}}, fasthttp.StatusOK); err != nil {
		ctx.Logger.WithError(err).Error("Error occurred encoding the admin session response.")
		ctx.ReplyForbidden()
	}
}
