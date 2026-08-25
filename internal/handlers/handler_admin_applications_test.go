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

func TestAdminApplicationsGETReportsUnavailableBackingGroup(t *testing.T) {
	mock, _ := newAdminAPITestContext(t)
	mock.Ctx.Configuration.Applications = []schema.Application{{Slug: "grafana", Name: "Grafana", Domain: "grafana.example.com"}}

	AdminApplicationsGET(mock.Ctx)
	assert.Equal(t, fasthttp.StatusNotFound, mock.Ctx.Response.StatusCode())
	assert.NotContains(t, string(mock.Ctx.Response.Body()), "grafana.example.com")
}
