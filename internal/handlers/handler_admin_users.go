package handlers

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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

type adminUserSetupLinkRequest struct {
	Username string `json:"username"`
}

type adminUserSetupLinkResponse struct {
	SetupURL  string    `json:"setup_url"`
	ExpiresAt time.Time `json:"expires_at"`
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

// AdminUserSetupLinkPOST creates a one-time link compatible with the reset-password completion flow.
func AdminUserSetupLinkPOST(ctx *middlewares.AutheliaCtx) {
	var request adminUserSetupLinkRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminUserStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin user storage is unavailable"))
		return
	}
	_, found, err := store.LoadMTLAdminUser(ctx, request.Username)
	if err != nil {
		adminAPIError(ctx, err)
		return
	}
	if !found {
		adminAPIError(ctx, storage.ErrMTLUserNotFound)
		return
	}
	issuer, err := ctx.IssuerURL()
	if err != nil {
		adminAPIError(ctx, err)
		return
	}
	jti, err := uuid.NewRandom()
	if err != nil {
		adminAPIError(ctx, err)
		return
	}
	now := ctx.GetClock().Now()
	expiresAt := now.Add(ctx.Configuration.IdentityValidation.ResetPassword.JWTExpiration)
	verification := model.IdentityVerification{
		JTI: jti, IssuedAt: now, ExpiresAt: expiresAt, Action: ActionResetPassword,
		Username: request.Username, IssuedIP: model.NewIP(ctx.RemoteIP()),
	}
	claims := verification.ToIdentityVerificationClaim(issuer)
	method := jwt.SigningMethodHS256
	switch ctx.Configuration.IdentityValidation.ResetPassword.JWTAlgorithm {
	case "HS384":
		method = jwt.SigningMethodHS384
	case "HS512":
		method = jwt.SigningMethodHS512
	}
	token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(ctx.Configuration.IdentityValidation.ResetPassword.JWTSecret))
	if err != nil {
		adminAPIError(ctx, err)
		return
	}
	if err = ctx.Providers.StorageProvider.SaveIdentityVerification(ctx, verification); err != nil {
		adminAPIError(ctx, err)
		return
	}
	setupURL := (&url.URL{Scheme: issuer.Scheme, Host: issuer.Host, Path: issuer.Path}).JoinPath("/reset-password/step2")
	query := setupURL.Query()
	query.Set("token", token)
	setupURL.RawQuery = query.Encode()
	ctx.Response.Header.Set("Cache-Control", "no-store")
	adminAPIRespond(ctx, adminUserSetupLinkResponse{SetupURL: setupURL.String(), ExpiresAt: expiresAt}, fasthttp.StatusOK, nil)
}
