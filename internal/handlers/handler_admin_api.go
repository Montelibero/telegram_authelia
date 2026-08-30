package handlers

import (
	"errors"
	"slices"

	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/storage"
)

type adminAccessScope struct {
	Full          bool
	ManagedGroups []string
}

func loadAdminAccessScope(ctx *middlewares.AutheliaCtx) adminAccessScope {
	userSession, err := ctx.GetSession()
	if err != nil {
		return adminAccessScope{}
	}
	if slices.Contains(userSession.Groups, "admins") {
		return adminAccessScope{Full: true}
	}
	groups, err := middlewares.LoadMTLManagedGroups(ctx, userSession.Username)
	if err != nil {
		ctx.GetLogger().WithError(err).Error("Unable to load administrator delegation scope")
		return adminAccessScope{}
	}
	return adminAccessScope{ManagedGroups: groups}
}

func (s adminAccessScope) managesGroup(group string) bool {
	return s.Full || slices.Contains(s.ManagedGroups, group)
}

func (s adminAccessScope) managesAny(groups []string) bool {
	if s.Full {
		return true
	}
	for _, group := range groups {
		if slices.Contains(s.ManagedGroups, group) {
			return true
		}
	}
	return false
}

func (s adminAccessScope) managesAll(groups []string) bool {
	if s.Full {
		return true
	}
	if len(groups) == 0 || !s.managesAny(groups) {
		return false
	}
	for _, group := range groups {
		if !slices.Contains(s.ManagedGroups, group) {
			return false
		}
	}
	return true
}

func requireAdminFull(ctx *middlewares.AutheliaCtx) bool {
	if !loadAdminAccessScope(ctx).Full {
		ctx.ReplyForbidden()
		return false
	}
	return true
}

func requireAdminManagedGroup(ctx *middlewares.AutheliaCtx, group string) bool {
	if !loadAdminAccessScope(ctx).managesGroup(group) {
		ctx.ReplyForbidden()
		return false
	}
	return true
}

func adminAPIParse(ctx *middlewares.AutheliaCtx, destination any) bool {
	if err := ctx.ParseBody(destination); err != nil {
		ctx.ReplyBadRequest()
		return false
	}
	return true
}

func adminAPIActor(ctx *middlewares.AutheliaCtx) string {
	userSession, err := ctx.GetSession()
	if err != nil {
		return ""
	}
	return userSession.Username
}

func adminAPIRespond(ctx *middlewares.AutheliaCtx, data any, status int, err error) {
	if err != nil {
		adminAPIError(ctx, err)
		return
	}
	if err = ctx.ReplyJSON(middlewares.OKResponse{Status: "OK", Data: data}, status); err != nil {
		adminAPIError(ctx, err)
	}
}

func adminAPIError(ctx *middlewares.AutheliaCtx, err error) {
	status := fasthttp.StatusInternalServerError
	switch {
	case errors.Is(err, storage.ErrMTLUserNotFound), errors.Is(err, storage.ErrMTLGroupNotFound), errors.Is(err, storage.ErrMTLIdentityNotFound), errors.Is(err, storage.ErrMTLMembershipNotFound), errors.Is(err, storage.ErrMTLRegistrationNotFound):
		status = fasthttp.StatusNotFound
	case errors.Is(err, storage.ErrMTLVersionConflict), errors.Is(err, storage.ErrMTLConflict), errors.Is(err, storage.ErrMTLPrimaryEmailRequired), errors.Is(err, storage.ErrMTLRegistrationTerminal):
		status = fasthttp.StatusConflict
	case errors.Is(err, storage.ErrMTLRegistrationIncomplete):
		status = fasthttp.StatusBadRequest
	}
	_ = ctx.ReplyJSON(middlewares.ErrorResponse{Status: "KO", Message: fasthttp.StatusMessage(status)}, status)
}
