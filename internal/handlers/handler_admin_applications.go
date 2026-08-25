package handlers

import (
	"context"
	"errors"
	"slices"

	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/storage"
)

type adminApplicationStore interface {
	ListMTLAdminUsers(context.Context) ([]model.MTLAdminUserSummary, error)
	ListMTLAdminGroups(context.Context) ([]model.MTLAdminGroupSummary, error)
}

type adminApplicationUserResponse struct {
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Status       string `json:"status"`
	Version      int    `json:"version"`
	PrimaryEmail string `json:"primary_email"`
	Granted      bool   `json:"granted"`
}

type adminApplicationResponse struct {
	Slug         string                         `json:"slug"`
	Name         string                         `json:"name"`
	Domain       string                         `json:"domain"`
	Group        string                         `json:"group"`
	GroupVersion int                            `json:"group_version"`
	Users        []adminApplicationUserResponse `json:"users"`
}

// AdminApplicationsGET returns the enabled application permission matrix for administrators.
func AdminApplicationsGET(ctx *middlewares.AutheliaCtx) {
	store, ok := ctx.Providers.StorageProvider.(adminApplicationStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin application storage is unavailable"))
		return
	}

	users, err := store.ListMTLAdminUsers(ctx)
	if err != nil {
		adminAPIError(ctx, err)
		return
	}
	groups, err := store.ListMTLAdminGroups(ctx)
	if err != nil {
		adminAPIError(ctx, err)
		return
	}

	versions := make(map[string]int, len(groups))
	for _, group := range groups {
		versions[group.Name] = group.Version
	}

	response := make([]adminApplicationResponse, 0, len(ctx.Configuration.Applications))
	for _, application := range ctx.Configuration.Applications {
		if !application.IsEnabled() {
			continue
		}
		group := applicationGroup(application)
		version, found := versions[group]
		if !found {
			adminAPIError(ctx, storage.ErrMTLGroupNotFound)
			return
		}

		item := adminApplicationResponse{
			Slug: application.Slug, Name: application.Name, Domain: application.Domain,
			Group: group, GroupVersion: version,
			Users: make([]adminApplicationUserResponse, 0, len(users)),
		}
		for _, user := range users {
			item.Users = append(item.Users, adminApplicationUserResponse{
				Username: user.Username, DisplayName: user.DisplayName, Status: user.Status,
				Version: user.Version, PrimaryEmail: user.PrimaryEmail, Granted: slices.Contains(user.Groups, group),
			})
		}
		response = append(response, item)
	}

	adminAPIRespond(ctx, response, fasthttp.StatusOK, nil)
}

func applicationGroup(application schema.Application) string {
	if application.Group != "" {
		return application.Group
	}
	return "app:" + application.Slug
}
