package handlers

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/storage"
)

type adminGroupStore interface {
	ListMTLAdminGroups(context.Context) ([]model.MTLAdminGroupSummary, error)
	LoadMTLAdminGroup(context.Context, string) (model.MTLAdminGroupDetails, bool, error)
	LoadMTLAdminUser(context.Context, string) (model.MTLAdminUserDetails, bool, error)
	CreateMTLAdminGroup(context.Context, string, string) (model.MTLAdminGroupDetails, error)
	RenameMTLAdminGroup(context.Context, string, string, int, string) (model.MTLAdminGroupDetails, []string, error)
	DeleteMTLAdminGroup(context.Context, string, int, string) ([]string, error)
	AddMTLAdminGroupUser(context.Context, string, string, int, string) (model.MTLAdminGroupDetails, error)
	RemoveMTLAdminGroupUser(context.Context, string, string, int, string) (model.MTLAdminGroupDetails, error)
	AddMTLAdminGroupManager(context.Context, string, string, int, string) (model.MTLAdminGroupDetails, error)
	RemoveMTLAdminGroupManager(context.Context, string, string, int, string) (model.MTLAdminGroupDetails, error)
}

type adminGroupRequest struct {
	Name            string `json:"name"`
	NewName         string `json:"new_name"`
	Username        string `json:"username"`
	ExpectedVersion int    `json:"expected_version"`
	ConfirmUsername string `json:"confirm_username"`
}

type adminGroupWarningResponse struct {
	Group                 model.MTLAdminGroupDetails `json:"group,omitempty"`
	AffectedUsers         []string                   `json:"affected_users"`
	ExternalACLNotUpdated bool                       `json:"external_acl_not_updated"`
}

func AdminGroupsGET(ctx *middlewares.AutheliaCtx) {
	store, ok := ctx.Providers.StorageProvider.(adminGroupStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin group storage is unavailable"))
		return
	}
	groups, err := store.ListMTLAdminGroups(ctx)
	scope := loadAdminAccessScope(ctx)
	if !scope.Full {
		filtered := groups[:0]
		for _, group := range groups {
			if scope.managesGroup(group.Name) {
				filtered = append(filtered, group)
			}
		}
		groups = filtered
	}
	for i := range groups {
		groups[i].Managed = adminApplicationGroupManaged(ctx, groups[i].Name)
	}
	adminAPIRespond(ctx, groups, fasthttp.StatusOK, err)
}

func AdminGroupGET(ctx *middlewares.AutheliaCtx) {
	name := string(ctx.QueryArgs().Peek("name"))
	if !requireAdminManagedGroup(ctx, name) {
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminGroupStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin group storage is unavailable"))
		return
	}
	group, found, err := store.LoadMTLAdminGroup(ctx, name)
	if err == nil && !found {
		err = storage.ErrMTLGroupNotFound
	}
	group.Managed = adminApplicationGroupManaged(ctx, group.Name)
	adminAPIRespond(ctx, group, fasthttp.StatusOK, err)
}

func AdminGroupPOST(ctx *middlewares.AutheliaCtx) {
	if !requireAdminFull(ctx) {
		return
	}
	var request adminGroupRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminGroupStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin group storage is unavailable"))
		return
	}
	group, err := store.CreateMTLAdminGroup(ctx, request.Name, adminAPIActor(ctx))
	group.Managed = adminApplicationGroupManaged(ctx, group.Name)
	adminAPIRespond(ctx, group, fasthttp.StatusCreated, err)
}

func AdminGroupPATCH(ctx *middlewares.AutheliaCtx) {
	if !requireAdminFull(ctx) {
		return
	}
	var request adminGroupRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	if adminApplicationGroupManaged(ctx, request.Name) {
		adminAPIError(ctx, storage.ErrMTLConflict)
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminGroupStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin group storage is unavailable"))
		return
	}
	actor := adminAPIActor(ctx)
	current, found, err := store.LoadMTLAdminGroup(ctx, request.Name)
	if err != nil || !found {
		if err == nil {
			err = storage.ErrMTLGroupNotFound
		}
		adminAPIRespond(ctx, nil, fasthttp.StatusOK, err)
		return
	}
	if slices.Contains(current.Users, actor) && request.ConfirmUsername != actor {
		ctx.ReplyBadRequest()
		return
	}
	group, affected, err := store.RenameMTLAdminGroup(ctx, request.Name, request.NewName, request.ExpectedVersion, actor)
	group.Managed = adminApplicationGroupManaged(ctx, group.Name)
	adminAPIRespond(ctx, adminGroupWarningResponse{Group: group, AffectedUsers: affected, ExternalACLNotUpdated: true}, fasthttp.StatusOK, err)
}

func AdminGroupDELETE(ctx *middlewares.AutheliaCtx) {
	if !requireAdminFull(ctx) {
		return
	}
	var request adminGroupRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	if adminApplicationGroupManaged(ctx, request.Name) {
		adminAPIError(ctx, storage.ErrMTLConflict)
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminGroupStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin group storage is unavailable"))
		return
	}
	actor := adminAPIActor(ctx)
	current, found, err := store.LoadMTLAdminGroup(ctx, request.Name)
	if err != nil || !found {
		if err == nil {
			err = storage.ErrMTLGroupNotFound
		}
		adminAPIRespond(ctx, nil, fasthttp.StatusOK, err)
		return
	}
	if slices.Contains(current.Users, actor) && request.ConfirmUsername != actor {
		ctx.ReplyBadRequest()
		return
	}
	affected, err := store.DeleteMTLAdminGroup(ctx, request.Name, request.ExpectedVersion, actor)
	adminAPIRespond(ctx, adminGroupWarningResponse{AffectedUsers: affected, ExternalACLNotUpdated: true}, fasthttp.StatusOK, err)
}

func AdminGroupUserPUT(ctx *middlewares.AutheliaCtx) {
	adminGroupUserMutation(ctx, true)
}

func AdminGroupUserDELETE(ctx *middlewares.AutheliaCtx) {
	adminGroupUserMutation(ctx, false)
}

func AdminGroupManagerPUT(ctx *middlewares.AutheliaCtx) {
	adminGroupManagerMutation(ctx, true)
}

func AdminGroupManagerDELETE(ctx *middlewares.AutheliaCtx) {
	adminGroupManagerMutation(ctx, false)
}

func adminGroupManagerMutation(ctx *middlewares.AutheliaCtx, add bool) {
	if !requireAdminFull(ctx) {
		return
	}
	var request adminGroupRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	if strings.EqualFold(request.Name, "admins") {
		ctx.ReplyForbidden()
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminGroupStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin group storage is unavailable"))
		return
	}
	var group model.MTLAdminGroupDetails
	var err error
	if add {
		group, err = store.AddMTLAdminGroupManager(ctx, request.Name, request.Username, request.ExpectedVersion, adminAPIActor(ctx))
	} else {
		group, err = store.RemoveMTLAdminGroupManager(ctx, request.Name, request.Username, request.ExpectedVersion, adminAPIActor(ctx))
	}
	group.Managed = adminApplicationGroupManaged(ctx, group.Name)
	adminAPIRespond(ctx, group, fasthttp.StatusOK, err)
}

func adminGroupUserMutation(ctx *middlewares.AutheliaCtx, add bool) {
	var request adminGroupRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	if !requireAdminManagedGroup(ctx, request.Name) {
		return
	}
	actor := adminAPIActor(ctx)
	if !add && request.Username == actor && request.ConfirmUsername != actor {
		ctx.ReplyBadRequest()
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminGroupStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin group storage is unavailable"))
		return
	}
	var group model.MTLAdminGroupDetails
	var err error
	if add {
		group, err = store.AddMTLAdminGroupUser(ctx, request.Name, request.Username, request.ExpectedVersion, actor)
	} else {
		group, err = store.RemoveMTLAdminGroupUser(ctx, request.Name, request.Username, request.ExpectedVersion, actor)
	}
	if err == nil && request.Username == actor {
		refreshAdminSessionAfterGroupMutation(ctx, store, actor)
	}
	group.Managed = adminApplicationGroupManaged(ctx, group.Name)
	adminAPIRespond(ctx, group, fasthttp.StatusOK, err)
}

func refreshAdminSessionAfterGroupMutation(ctx *middlewares.AutheliaCtx, store adminGroupStore, username string) {
	details, found, err := store.LoadMTLAdminUser(ctx, username)
	if err != nil || !found {
		ctx.GetLogger().WithError(err).Error("Unable to refresh administrator session after group membership change")
		return
	}
	userSession, err := ctx.GetSession()
	if err != nil {
		ctx.GetLogger().WithError(err).Error("Unable to load administrator session after group membership change")
		return
	}
	emails := make([]string, len(details.Emails))
	for i := range details.Emails {
		emails[i] = details.Emails[i].Email
	}
	userSession.DisplayName = details.DisplayName
	userSession.Groups = details.Groups
	userSession.Emails = emails
	userSession.SessionEpoch = &details.SessionEpoch
	if err = ctx.SaveSession(userSession); err != nil {
		ctx.GetLogger().WithError(err).Error("Unable to save administrator session after group membership change")
	}
}

func adminApplicationGroupManaged(ctx *middlewares.AutheliaCtx, name string) bool {
	for _, application := range ctx.Configuration.Applications {
		if application.IsEnabled() && applicationGroup(application) == name {
			return true
		}
	}

	return false
}
