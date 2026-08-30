package handlers

import (
	"slices"

	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/session"
)

// StateGET is the handler serving the user state.
func StateGET(ctx *middlewares.AutheliaCtx) {
	var (
		userSession session.UserSession
		err         error
	)
	if userSession, err = ctx.GetSession(); err != nil {
		ctx.Logger.WithError(err).Error("Error occurred retrieving user session")

		ctx.ReplyForbidden()

		return
	}

	administrator := slices.Contains(userSession.Groups, "admins")
	managedGroups, _ := middlewares.LoadMTLManagedGroups(ctx, userSession.Username)
	stateResponse := StateResponse{
		Username:            userSession.Username,
		AuthenticationLevel: userSession.AuthenticationLevel(ctx.Configuration.WebAuthn.EnablePasskey2FA),
		FactorKnowledge:     userSession.AuthenticationMethodRefs.FactorKnowledge(),
		Administrator:       administrator,
		Manager:             !administrator && len(managedGroups) != 0,
	}

	if uri := ctx.GetDefaultRedirectionURL(); uri != nil {
		stateResponse.DefaultRedirectionURL = uri.String()
	}

	if err = ctx.SetJSONBody(stateResponse); err != nil {
		ctx.Logger.Errorf("Unable to set state response in body: %s", err)
	}
}
