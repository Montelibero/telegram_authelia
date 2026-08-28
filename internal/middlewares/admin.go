package middlewares

import (
	"context"
	"net/url"
	"slices"
	"strings"

	"github.com/authelia/authelia/v4/internal/session"
)

const adminGroup = "admins"

type managerDelegationStore interface {
	ListMTLManagedGroups(context.Context, string) ([]string, error)
}

// RequireAdmin requires an authenticated session with current membership in the admins group.
func RequireAdmin(next RequestHandler) RequestHandler {
	return func(ctx *AutheliaCtx) {
		userSession, ok := loadAdminSession(ctx)
		if !ok {
			return
		}
		if !slices.Contains(userSession.Groups, adminGroup) {
			ctx.ReplyForbidden()
			return
		}
		next(ctx)
	}
}

// RequireAdminAccess permits full administrators and users with at least one explicit group delegation.
func RequireAdminAccess(next RequestHandler) RequestHandler {
	return func(ctx *AutheliaCtx) {
		userSession, ok := loadAdminSession(ctx)
		if !ok {
			return
		}
		if slices.Contains(userSession.Groups, adminGroup) {
			next(ctx)
			return
		}
		groups, err := LoadMTLManagedGroups(ctx, userSession.Username)
		if err != nil {
			ctx.GetLogger().WithError(err).Error("Unable to load delegated administrator groups")
			ctx.ReplyForbidden()
			return
		}
		if len(groups) == 0 {
			ctx.ReplyForbidden()
			return
		}
		next(ctx)
	}
}

// LoadMTLManagedGroups returns the groups delegated to a user by a full administrator.
func LoadMTLManagedGroups(ctx *AutheliaCtx, username string) ([]string, error) {
	store, ok := ctx.Providers.StorageProvider.(managerDelegationStore)
	if !ok {
		return nil, nil
	}
	return store.ListMTLManagedGroups(ctx, username)
}

func loadAdminSession(ctx *AutheliaCtx) (session.UserSession, bool) {
	userSession, err := ctx.GetSession()
	if err != nil || userSession.IsAnonymous() {
		ctx.ReplyUnauthorized()
		return userSession, false
	}
	if !validateMTLSession(ctx, &userSession) {
		ctx.ReplyUnauthorized()
		return userSession, false
	}
	return userSession, true
}

// RequireAdminMutation additionally requires a same-origin request and an elevated session.
func RequireAdminMutation(next RequestHandler) RequestHandler {
	return RequireAdmin(requireAdminSameOrigin(RequireElevated(next)))
}

// RequireAdminAccessMutation permits scoped managers and full administrators with same-origin elevation.
func RequireAdminAccessMutation(next RequestHandler) RequestHandler {
	return RequireAdminAccess(requireAdminSameOrigin(RequireElevated(next)))
}

// RequireSameOriginMutation rejects state-changing requests not originating from this Authelia endpoint.
func RequireSameOriginMutation(next RequestHandler) RequestHandler {
	return requireAdminSameOrigin(next)
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
