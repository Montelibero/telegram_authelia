package handlers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/model"
)

func TestAdminApplicationsGETReturnsEnabledApplicationsAndSafePermissionState(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	disabled := false
	mock.Ctx.Configuration.Applications = []schema.Application{
		{Slug: "grafana", Name: "Grafana", Domain: "grafana.example.com"},
		{Slug: "shared", Name: "Shared", Domain: "shared.example.com", Group: "team / weird:*"},
		{Slug: "disabled", Name: "Disabled", Domain: "disabled.example.com", Enabled: &disabled},
	}
	require.NoError(t, store.ReconcileMTLGroups(t.Context(), []string{"app:grafana", "team / weird:*"}))
	_, err := store.CreateMTLAdminUser(t.Context(), model.MTLAdminUserCreate{Username: "bublik", DisplayName: "Bublik", Email: "bublik@example.com"}, "admin")
	require.NoError(t, err)
	group, found, err := store.LoadMTLAdminGroup(t.Context(), "app:grafana")
	require.NoError(t, err)
	require.True(t, found)
	_, err = store.AddMTLAdminGroupUser(t.Context(), group.Name, "bublik", group.Version, "admin")
	require.NoError(t, err)

	AdminApplicationsGET(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())

	var response struct {
		Data []adminApplicationResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(mock.Ctx.Response.Body(), &response))
	require.Len(t, response.Data, 2)
	assert.Equal(t, "grafana", response.Data[0].Slug)
	assert.Equal(t, "app:grafana", response.Data[0].Group)
	assert.Equal(t, "Grafana", response.Data[0].Name)
	assert.Equal(t, "grafana.example.com", response.Data[0].Domain)
	assert.GreaterOrEqual(t, response.Data[0].GroupVersion, 0)

	users := response.Data[0].Users
	require.Len(t, users, 2)
	assert.Equal(t, "admin", users[0].Username)
	assert.False(t, users[0].Granted)
	assert.Equal(t, "bublik", users[1].Username)
	assert.Equal(t, "Bublik", users[1].DisplayName)
	assert.Equal(t, "bublik@example.com", users[1].PrimaryEmail)
	assert.True(t, users[1].Granted)
	assert.NotContains(t, string(mock.Ctx.Response.Body()), "password_hash")
	assert.NotContains(t, string(mock.Ctx.Response.Body()), "identities")
	assert.NotContains(t, string(mock.Ctx.Response.Body()), "disabled.example.com")
}

func TestAdminApplicationsGETUsesGroupsFromAccessControlWithoutApplications(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	mock.Ctx.Configuration.Applications = nil
	mock.Ctx.Configuration.AccessControl.Rules = []schema.AccessControlRule{
		{Domains: []string{"grist.example.com"}, Policy: "one_factor", Subjects: [][]string{{"group:app:grist"}}},
		{Domains: []string{"shared.example.com"}, Policy: "one_factor", Subjects: [][]string{{"group:team / weird:*"}}},
		{Domains: []string{"admin.example.com"}, Policy: "one_factor", Subjects: [][]string{{"group:admins"}}},
		{Domains: []string{"duplicate.example.com"}, Policy: "one_factor", Subjects: [][]string{{"group:app:grist"}}},
	}
	require.NoError(t, store.ReconcileMTLGroups(t.Context(), []string{"app:grist", "team / weird:*", "admins"}))
	_, err := store.CreateMTLAdminUser(t.Context(), model.MTLAdminUserCreate{Username: "bublik", Email: "bublik@example.com"}, "admin")
	require.NoError(t, err)

	AdminApplicationsGET(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())

	var response struct {
		Data []adminApplicationResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(mock.Ctx.Response.Body(), &response))
	require.Len(t, response.Data, 2)
	assert.Equal(t, "app:grist", response.Data[0].Slug)
	assert.Equal(t, "app:grist", response.Data[0].Name)
	assert.Equal(t, "app:grist", response.Data[0].Group)
	assert.Empty(t, response.Data[0].Domain)
	assert.Equal(t, "team / weird:*", response.Data[1].Slug)
	assert.NotContains(t, string(mock.Ctx.Response.Body()), "admins")
}

func TestAdminApplicationsGETReportsUnavailableBackingGroup(t *testing.T) {
	mock, _ := newAdminAPITestContext(t)
	mock.Ctx.Configuration.Applications = []schema.Application{{Slug: "grafana", Name: "Grafana", Domain: "grafana.example.com"}}

	AdminApplicationsGET(mock.Ctx)
	assert.Equal(t, fasthttp.StatusNotFound, mock.Ctx.Response.StatusCode())
	assert.NotContains(t, string(mock.Ctx.Response.Body()), "grafana.example.com")
}

func TestAdminApplicationUserMutationGrantRevokeDuplicatesAndStaleVersion(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	mock.Ctx.Configuration.Applications = []schema.Application{
		{Slug: "grafana", Name: "Grafana", Domain: "grafana.example.com"},
	}
	require.NoError(t, store.ReconcileMTLGroups(t.Context(), []string{"app:grafana"}))
	_, err := store.CreateMTLAdminUser(t.Context(), model.MTLAdminUserCreate{Username: "bublik", Email: "bublik@example.com"}, "admin")
	require.NoError(t, err)
	group, found, err := store.LoadMTLAdminGroup(t.Context(), "app:grafana")
	require.NoError(t, err)
	require.True(t, found)

	mock.Ctx.Request.SetBodyString(`{"slug":"grafana","username":"bublik","expected_version":` + jsonInt(group.Version) + `}`)
	AdminApplicationUserPUT(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.Equal(t, 1, permissionResponseGrantedCount(t, mock.Ctx.Response.Body(), "bublik"))

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"slug":"grafana","username":"bublik","expected_version":` + jsonInt(group.Version+1) + `}`)
	AdminApplicationUserPUT(mock.Ctx)
	assert.Equal(t, fasthttp.StatusConflict, mock.Ctx.Response.StatusCode())

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"slug":"grafana","username":"bublik","expected_version":` + jsonInt(group.Version) + `}`)
	AdminApplicationUserDELETE(mock.Ctx)
	assert.Equal(t, fasthttp.StatusConflict, mock.Ctx.Response.StatusCode())

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"slug":"grafana","username":"bublik","expected_version":` + jsonInt(group.Version+1) + `}`)
	AdminApplicationUserDELETE(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.Zero(t, permissionResponseGrantedCount(t, mock.Ctx.Response.Body(), "bublik"))

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"slug":"grafana","username":"bublik","expected_version":` + jsonInt(group.Version+2) + `}`)
	AdminApplicationUserDELETE(mock.Ctx)
	assert.Equal(t, fasthttp.StatusNotFound, mock.Ctx.Response.StatusCode())
}

func TestAdminApplicationUserMutationRejectsUnconfiguredOrDisabledSlug(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	disabled := false
	mock.Ctx.Configuration.Applications = []schema.Application{{Slug: "disabled", Name: "Disabled", Domain: "disabled.example.com", Enabled: &disabled}}
	require.NoError(t, store.ReconcileMTLGroups(t.Context(), []string{"app:unconfigured"}))

	for _, slug := range []string{"unconfigured", "disabled"} {
		mock.Ctx.Response.Reset()
		mock.Ctx.Request.SetBodyString(`{"slug":"` + slug + `","username":"admin","expected_version":0}`)
		AdminApplicationUserPUT(mock.Ctx)
		assert.Equal(t, fasthttp.StatusNotFound, mock.Ctx.Response.StatusCode())
	}

	group, found, err := store.LoadMTLAdminGroup(t.Context(), "app:unconfigured")
	require.NoError(t, err)
	require.True(t, found)
	assert.Empty(t, group.Users)
}

func TestAdminApplicationUserMutationCannotChangeAdministrators(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	mock.Ctx.Configuration.Applications = []schema.Application{{Slug: "control", Name: "Control", Domain: "control.example.com", Group: "Admins"}}
	require.NoError(t, store.ReconcileMTLGroups(t.Context(), []string{"admins"}))
	_, err := store.CreateMTLAdminUser(t.Context(), model.MTLAdminUserCreate{Username: "bublik", Email: "bublik@example.com"}, "admin")
	require.NoError(t, err)
	group, found, err := store.LoadMTLAdminGroup(t.Context(), "admins")
	require.NoError(t, err)
	require.True(t, found)

	mock.Ctx.Request.SetBodyString(`{"slug":"control","username":"bublik","expected_version":` + jsonInt(group.Version) + `}`)
	AdminApplicationUserPUT(mock.Ctx)
	assert.Equal(t, fasthttp.StatusNotFound, mock.Ctx.Response.StatusCode())
	group, _, err = store.LoadMTLAdminGroup(t.Context(), "admins")
	require.NoError(t, err)
	assert.Empty(t, group.Users)

	group, err = store.AddMTLAdminGroupUser(t.Context(), "admins", "bublik", group.Version, "admin")
	require.NoError(t, err)
	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"slug":"control","username":"bublik","expected_version":` + jsonInt(group.Version) + `}`)
	AdminApplicationUserDELETE(mock.Ctx)
	assert.Equal(t, fasthttp.StatusNotFound, mock.Ctx.Response.StatusCode())
	group, _, err = store.LoadMTLAdminGroup(t.Context(), "admins")
	require.NoError(t, err)
	assert.Equal(t, []string{"bublik"}, group.Users)
}

func permissionResponseGrantedCount(t *testing.T, body []byte, username string) int {
	t.Helper()
	var response struct {
		Data []adminApplicationResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &response))
	count := 0
	for _, application := range response.Data {
		for _, user := range application.Users {
			if user.Username == username && user.Granted {
				count++
			}
		}
	}
	return count
}
