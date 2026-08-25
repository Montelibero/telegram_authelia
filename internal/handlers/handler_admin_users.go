package handlers

import (
	"context"
	"errors"

	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/storage"
)

type adminUserStore interface {
	ListMTLAdminUsers(context.Context) ([]model.MTLAdminUserSummary, error)
	LoadMTLAdminUser(context.Context, string) (model.MTLAdminUserDetails, bool, error)
	CreateMTLAdminUser(context.Context, model.MTLAdminUserCreate, string) (model.MTLAdminUserDetails, error)
	UpdateMTLAdminUser(context.Context, string, model.MTLAdminUserUpdate, string) (model.MTLAdminUserDetails, error)
	AddMTLAdminUserEmail(context.Context, string, model.MTLAdminEmailCreate, string) (model.MTLAdminUserDetails, error)
	SetMTLAdminPrimaryEmail(context.Context, string, string, int, string) (model.MTLAdminUserDetails, error)
	DeleteMTLAdminUserEmail(context.Context, string, string, int, string) (model.MTLAdminUserDetails, error)
	UnlinkMTLAdminUserIdentity(context.Context, string, string, int, string) (model.MTLAdminUserDetails, error)
}

type adminUserUpdateRequest struct {
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	Status          string `json:"status"`
	ExpectedVersion int    `json:"expected_version"`
	ConfirmUsername string `json:"confirm_username"`
}

type adminUserEmailRequest struct {
	Username        string `json:"username"`
	Email           string `json:"email"`
	ExpectedVersion int    `json:"expected_version"`
	Primary         bool   `json:"primary"`
}

type adminUserIdentityRequest struct {
	Username        string `json:"username"`
	Provider        string `json:"provider"`
	ExpectedVersion int    `json:"expected_version"`
	ConfirmUsername string `json:"confirm_username"`
}

func AdminUsersGET(ctx *middlewares.AutheliaCtx) {
	store, ok := ctx.Providers.StorageProvider.(adminUserStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin user storage is unavailable"))
		return
	}
	users, err := store.ListMTLAdminUsers(ctx)
	adminAPIRespond(ctx, users, fasthttp.StatusOK, err)
}

func AdminUserGET(ctx *middlewares.AutheliaCtx) {
	store, ok := ctx.Providers.StorageProvider.(adminUserStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin user storage is unavailable"))
		return
	}
	username := string(ctx.QueryArgs().Peek("username"))
	details, found, err := store.LoadMTLAdminUser(ctx, username)
	if err == nil && !found {
		err = storage.ErrMTLUserNotFound
	}
	adminAPIRespond(ctx, details, fasthttp.StatusOK, err)
}

func AdminUserPOST(ctx *middlewares.AutheliaCtx) {
	var request model.MTLAdminUserCreate
	if !adminAPIParse(ctx, &request) {
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminUserStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin user storage is unavailable"))
		return
	}
	details, err := store.CreateMTLAdminUser(ctx, request, adminAPIActor(ctx))
	adminAPIRespond(ctx, details, fasthttp.StatusCreated, err)
}

func AdminUserPATCH(ctx *middlewares.AutheliaCtx) {
	var request adminUserUpdateRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	actor := adminAPIActor(ctx)
	if request.Username == actor && request.Status == model.MTLUserStatusDisabled && request.ConfirmUsername != actor {
		ctx.ReplyBadRequest()
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminUserStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin user storage is unavailable"))
		return
	}
	details, err := store.UpdateMTLAdminUser(ctx, request.Username, model.MTLAdminUserUpdate{ExpectedVersion: request.ExpectedVersion, DisplayName: request.DisplayName, Status: request.Status}, actor)
	adminAPIRespond(ctx, details, fasthttp.StatusOK, err)
}

func AdminUserEmailPOST(ctx *middlewares.AutheliaCtx) {
	var request adminUserEmailRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminUserStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin user storage is unavailable"))
		return
	}
	details, err := store.AddMTLAdminUserEmail(ctx, request.Username, model.MTLAdminEmailCreate{ExpectedVersion: request.ExpectedVersion, Email: request.Email, Primary: request.Primary}, adminAPIActor(ctx))
	adminAPIRespond(ctx, details, fasthttp.StatusOK, err)
}

func AdminUserEmailPrimaryPUT(ctx *middlewares.AutheliaCtx) {
	var request adminUserEmailRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminUserStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin user storage is unavailable"))
		return
	}
	details, err := store.SetMTLAdminPrimaryEmail(ctx, request.Username, request.Email, request.ExpectedVersion, adminAPIActor(ctx))
	adminAPIRespond(ctx, details, fasthttp.StatusOK, err)
}

func AdminUserEmailDELETE(ctx *middlewares.AutheliaCtx) {
	var request adminUserEmailRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminUserStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin user storage is unavailable"))
		return
	}
	details, err := store.DeleteMTLAdminUserEmail(ctx, request.Username, request.Email, request.ExpectedVersion, adminAPIActor(ctx))
	adminAPIRespond(ctx, details, fasthttp.StatusOK, err)
}

func AdminUserIdentityDELETE(ctx *middlewares.AutheliaCtx) {
	var request adminUserIdentityRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	actor := adminAPIActor(ctx)
	store, ok := ctx.Providers.StorageProvider.(adminUserStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin user storage is unavailable"))
		return
	}
	if request.Username == actor {
		details, found, err := store.LoadMTLAdminUser(ctx, request.Username)
		if err != nil {
			adminAPIError(ctx, err)
			return
		}
		if !found {
			adminAPIError(ctx, storage.ErrMTLUserNotFound)
			return
		}
		providerFound := false
		for _, identity := range details.Identities {
			if identity.Provider == request.Provider {
				providerFound = true
				break
			}
		}
		if providerFound && !details.PasswordEnabled && len(details.Identities) == 1 && request.ConfirmUsername != actor {
			ctx.ReplyBadRequest()
			return
		}
	}
	details, err := store.UnlinkMTLAdminUserIdentity(ctx, request.Username, request.Provider, request.ExpectedVersion, actor)
	adminAPIRespond(ctx, details, fasthttp.StatusOK, err)
}
