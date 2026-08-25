package middlewares

import (
	"net/url"
	"slices"
	"strings"
)

const adminGroup = "admins"

// RequireAdmin requires an authenticated session with current membership in the admins group.
func RequireAdmin(next RequestHandler) RequestHandler {
	return func(ctx *AutheliaCtx) {
		userSession, err := ctx.GetSession()
		if err != nil || userSession.IsAnonymous() {
			ctx.ReplyUnauthorized()
			return
		}
		if !validateMTLSession(ctx, &userSession) {
			ctx.ReplyUnauthorized()
			return
		}
		if !slices.Contains(userSession.Groups, adminGroup) {
			ctx.ReplyForbidden()
			return
		}
		next(ctx)
	}
}

// RequireAdminMutation additionally requires a same-origin request and fresh password proof in the same session.
func RequireAdminMutation(next RequestHandler) RequestHandler {
	return RequireAdmin(requireAdminSameOrigin(requireAdminFreshPassword(next)))
}

func requireAdminFreshPassword(next RequestHandler) RequestHandler {
	return func(ctx *AutheliaCtx) {
		userSession, err := ctx.GetSession()
		if err != nil || !userSession.AuthenticationMethodRefs.UsernameAndPassword {
			ctx.ReplyForbidden()
			return
		}
		now := ctx.GetClock().Now()
		lifespan := ctx.Configuration.IdentityValidation.ElevatedSession.ElevationLifespan
		passwordProof := userSession.GetFirstFactorAuthn()
		if lifespan <= 0 || passwordProof.IsZero() || passwordProof.After(now) || now.Sub(passwordProof) > lifespan {
			ctx.ReplyForbidden()
			return
		}
		next(ctx)
	}
}

func requireAdminSameOrigin(next RequestHandler) RequestHandler {
	return func(ctx *AutheliaCtx) {
		origin, err := url.Parse(string(ctx.Request.Header.Peek("Origin")))
		expectedScheme := string(ctx.XForwardedProto())
		if expectedScheme == "" {
			expectedScheme = "http"
		}
		if err != nil || origin.Scheme == "" || origin.Host == "" ||
			!strings.EqualFold(origin.Host, string(ctx.GetXForwardedHost())) ||
			!strings.EqualFold(origin.Scheme, expectedScheme) {
			ctx.ReplyForbidden()
			return
		}
		next(ctx)
	}
}
