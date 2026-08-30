package handlers

import (
	"context"
	"errors"

	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/storage"
)

type selfServiceProfileStore interface {
	LoadMTLAdminUser(ctx context.Context, username string) (model.MTLAdminUserDetails, bool, error)
	UpdateMTLAdminUser(ctx context.Context, username string, update model.MTLAdminUserUpdate, actor string) (model.MTLAdminUserDetails, error)
}

func SelfServiceProfileGET(ctx *middlewares.AutheliaCtx) {
	userSession, err := ctx.GetSession()
	store, ok := ctx.Providers.StorageProvider.(selfServiceProfileStore)
	if err != nil || userSession.Username == "" || !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	details, found, err := store.LoadMTLAdminUser(ctx, userSession.Username)
	if err != nil || !found {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}
	profile := model.MTLSelfServiceProfile{Username: details.Username, DisplayName: details.DisplayName, Version: details.Version, PasswordEnabled: details.PasswordEnabled}
	for _, identity := range details.Identities {
		if identity.Provider == "telegram" {
			profile.TelegramLinked = true
			break
		}
	}
	_ = ctx.ReplyJSON(middlewares.OKResponse{Status: "OK", Data: profile}, fasthttp.StatusOK)
}

func SelfServiceProfilePATCH(ctx *middlewares.AutheliaCtx) {
	userSession, err := ctx.GetSession()
	store, ok := ctx.Providers.StorageProvider.(selfServiceProfileStore)
	if err != nil || userSession.Username == "" || !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	var update model.MTLSelfServiceProfileUpdate
	if err = ctx.ParseBody(&update); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}
	current, found, err := store.LoadMTLAdminUser(ctx, userSession.Username)
	if err != nil || !found {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}
	details, err := store.UpdateMTLAdminUser(ctx, userSession.Username, model.MTLAdminUserUpdate{ExpectedVersion: update.ExpectedVersion, DisplayName: update.DisplayName, Status: current.Status}, userSession.Username)
	if err != nil {
		if errors.Is(err, storage.ErrMTLVersionConflict) {
			ctx.SetStatusCode(fasthttp.StatusConflict)
		} else {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		}
		return
	}
	userSession.DisplayName = details.DisplayName
	if err = ctx.SaveSession(userSession); err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}
	SelfServiceProfileGET(ctx)
}
