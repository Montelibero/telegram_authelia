package handlers

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/storage"
)

type adminRegistrationStore interface {
	ListMTLRegistrations(context.Context, model.MTLRegistrationStatus) ([]model.MTLRegistrationRequest, error)
	LoadMTLRegistration(context.Context, int64) (model.MTLRegistrationRequest, bool, error)
	ApproveMTLRegistration(context.Context, model.MTLRegistrationApproval) (string, error)
	RejectMTLRegistration(context.Context, int64, int, string) (model.MTLRegistrationRequest, error)
}

type adminRegistrationMutationRequest struct {
	ID              int64    `json:"id"`
	ExpectedVersion int      `json:"expected_version"`
	Username        string   `json:"username"`
	DisplayName     string   `json:"display_name"`
	Email           string   `json:"email"`
	Groups          []string `json:"groups"`
}

type adminRegistrationResponse struct {
	ID               int64                       `json:"id"`
	Provider         string                      `json:"provider"`
	ProviderUserID   string                      `json:"provider_user_id"`
	ProviderUsername *string                     `json:"provider_username"`
	DisplayName      *string                     `json:"display_name"`
	ProposedUsername *string                     `json:"proposed_username"`
	ProposedEmail    *string                     `json:"proposed_email"`
	Status           model.MTLRegistrationStatus `json:"status"`
	Version          int                         `json:"version"`
	RequestedAt      time.Time                   `json:"requested_at"`
	UpdatedAt        time.Time                   `json:"updated_at"`
	ResolvedAt       *time.Time                  `json:"resolved_at"`
}

func AdminRegistrationsGET(ctx *middlewares.AutheliaCtx) {
	status := model.MTLRegistrationStatus(ctx.QueryArgs().Peek("status"))
	if status != "" && !status.Valid() {
		ctx.ReplyBadRequest()
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminRegistrationStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin registration storage is unavailable"))
		return
	}
	requests, err := store.ListMTLRegistrations(ctx, status)
	responses := make([]adminRegistrationResponse, len(requests))
	for i := range requests {
		responses[i] = newAdminRegistrationResponse(requests[i])
	}
	adminAPIRespond(ctx, responses, fasthttp.StatusOK, err)
}

func AdminRegistrationGET(ctx *middlewares.AutheliaCtx) {
	id, err := strconv.ParseInt(string(ctx.QueryArgs().Peek("id")), 10, 64)
	if err != nil || id < 1 {
		ctx.ReplyBadRequest()
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminRegistrationStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin registration storage is unavailable"))
		return
	}
	request, found, err := store.LoadMTLRegistration(ctx, id)
	if err == nil && !found {
		err = storage.ErrMTLRegistrationNotFound
	}
	adminAPIRespond(ctx, newAdminRegistrationResponse(request), fasthttp.StatusOK, err)
}

func AdminRegistrationApprovePOST(ctx *middlewares.AutheliaCtx) {
	var request adminRegistrationMutationRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminRegistrationStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin registration storage is unavailable"))
		return
	}
	username, err := store.ApproveMTLRegistration(ctx, model.MTLRegistrationApproval{
		RequestID: request.ID, ExpectedVersion: request.ExpectedVersion, Username: request.Username,
		DisplayName: request.DisplayName, Email: request.Email, Groups: request.Groups, ActorUsername: adminAPIActor(ctx),
	})
	adminAPIRespond(ctx, map[string]string{"username": username}, fasthttp.StatusOK, err)
}

func AdminRegistrationRejectPOST(ctx *middlewares.AutheliaCtx) {
	var request adminRegistrationMutationRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminRegistrationStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin registration storage is unavailable"))
		return
	}
	rejected, err := store.RejectMTLRegistration(ctx, request.ID, request.ExpectedVersion, adminAPIActor(ctx))
	adminAPIRespond(ctx, newAdminRegistrationResponse(rejected), fasthttp.StatusOK, err)
}

func newAdminRegistrationResponse(request model.MTLRegistrationRequest) adminRegistrationResponse {
	return adminRegistrationResponse{
		ID: request.ID, Provider: request.Provider, ProviderUserID: request.ProviderUserID,
		ProviderUsername: nullableAdminString(request.ProviderUsername), DisplayName: nullableAdminString(request.DisplayName),
		ProposedUsername: nullableAdminString(request.ProposedUsername), ProposedEmail: nullableAdminString(request.ProposedEmail),
		Status: request.Status, Version: request.Version, RequestedAt: request.RequestedAt, UpdatedAt: request.UpdatedAt,
		ResolvedAt: nullableAdminTime(request.ResolvedAt),
	}
}

func nullableAdminString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullableAdminTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
