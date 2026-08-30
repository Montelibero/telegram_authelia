package handlers

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/storage"
)

type adminApplicationStore interface {
	ListMTLAdminUsers(context.Context) ([]model.MTLAdminUserSummary, error)
	ListMTLAdminGroups(context.Context) ([]model.MTLAdminGroupSummary, error)
	AddMTLAdminGroupUser(context.Context, string, string, int, string) (model.MTLAdminGroupDetails, error)
	RemoveMTLAdminGroupUser(context.Context, string, string, int, string) (model.MTLAdminGroupDetails, error)
}

type adminApplicationUserRequest struct {
	Slug            string `json:"slug"`
	Username        string `json:"username"`
	ExpectedVersion int    `json:"expected_version"`
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

	response, err := loadAdminApplications(ctx, store)
	adminAPIRespond(ctx, response, fasthttp.StatusOK, err)
}

// AdminApplicationUserPUT grants a configured application permission to a user.
func AdminApplicationUserPUT(ctx *middlewares.AutheliaCtx) {
	adminApplicationUserMutation(ctx, true)
}

// AdminApplicationUserDELETE revokes a configured application permission from a user.
func AdminApplicationUserDELETE(ctx *middlewares.AutheliaCtx) {
	adminApplicationUserMutation(ctx, false)
}

func adminApplicationUserMutation(ctx *middlewares.AutheliaCtx, grant bool) {
	var request adminApplicationUserRequest
	if !adminAPIParse(ctx, &request) {
		return
	}
	application, found := configuredApplication(configuredPermissionApplications(ctx.Configuration.AccessControl.Rules, ctx.Configuration.Applications), request.Slug)
	if !found {
		adminAPIError(ctx, storage.ErrMTLGroupNotFound)
		return
	}
	store, ok := ctx.Providers.StorageProvider.(adminApplicationStore)
	if !ok {
		adminAPIError(ctx, errors.New("admin application storage is unavailable"))
		return
	}

	group := applicationGroup(application)
	if !requireAdminManagedGroup(ctx, group) {
		return
	}
	var err error
	if grant {
		_, err = store.AddMTLAdminGroupUser(ctx, group, request.Username, request.ExpectedVersion, adminAPIActor(ctx))
	} else {
		_, err = store.RemoveMTLAdminGroupUser(ctx, group, request.Username, request.ExpectedVersion, adminAPIActor(ctx))
	}
	if err != nil {
		adminAPIError(ctx, err)
		return
	}

	response, err := loadAdminApplications(ctx, store)
	adminAPIRespond(ctx, response, fasthttp.StatusOK, err)
}

func loadAdminApplications(ctx *middlewares.AutheliaCtx, store adminApplicationStore) ([]adminApplicationResponse, error) {
	users, err := store.ListMTLAdminUsers(ctx)
	if err != nil {
		return nil, err
	}
	groups, err := store.ListMTLAdminGroups(ctx)
	if err != nil {
		return nil, err
	}

	versions := make(map[string]int, len(groups))
	for _, group := range groups {
		versions[group.Name] = group.Version
	}

	applications := configuredPermissionApplications(ctx.Configuration.AccessControl.Rules, ctx.Configuration.Applications)
	scope := loadAdminAccessScope(ctx)
	response := make([]adminApplicationResponse, 0, len(applications))
	for _, application := range applications {
		if !application.IsEnabled() {
			continue
		}
		group := applicationGroup(application)
		if !scope.managesGroup(group) {
			continue
		}
		version, found := versions[group]
		if !found {
			return nil, storage.ErrMTLGroupNotFound
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

	return response, nil
}

func configuredPermissionApplications(rules []schema.AccessControlRule, legacy []schema.Application) []schema.Application {
	if len(legacy) != 0 {
		return legacy
	}
	seen := map[string]struct{}{}
	groups := make([]string, 0)
	for _, rule := range rules {
		for _, subjects := range rule.Subjects {
			for _, subject := range subjects {
				subject = strings.TrimSpace(subject)
				if !strings.HasPrefix(subject, "group:") {
					continue
				}
				group := strings.TrimSpace(strings.TrimPrefix(subject, "group:"))
				if group == "" || strings.EqualFold(group, "admins") {
					continue
				}
				if _, found := seen[group]; found {
					continue
				}
				seen[group] = struct{}{}
				groups = append(groups, group)
			}
		}
	}
	sort.Strings(groups)
	applications := make([]schema.Application, 0, len(groups))
	for _, group := range groups {
		applications = append(applications, schema.Application{Slug: group, Name: group, Group: group})
	}
	return applications
}

func configuredApplication(applications []schema.Application, slug string) (schema.Application, bool) {
	for _, application := range applications {
		if application.Slug == slug && application.IsEnabled() && !strings.EqualFold(applicationGroup(application), "admins") {
			return application, true
		}
	}
	return schema.Application{}, false
}

func applicationGroup(application schema.Application) string {
	if application.Group != "" {
		return application.Group
	}
	return "app:" + application.Slug
}
