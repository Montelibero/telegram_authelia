package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/model"
)

func TestAdminGroupsAPIWorkflowAndWarnings(t *testing.T) {
	mock, store := newAdminAPITestContext(t)

	mock.Ctx.Request.SetBodyString(`{"name":"team / weird:*"}`)
	AdminGroupPOST(mock.Ctx)
	assert.Equal(t, fasthttp.StatusCreated, mock.Ctx.Response.StatusCode())
	group, found, err := store.LoadMTLAdminGroup(context.Background(), "team / weird:*")
	require.NoError(t, err)
	require.True(t, found)
	mock.Ctx.Response.Reset()
	AdminGroupsGET(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	mock.Ctx.Response.Reset()
	mock.Ctx.QueryArgs().Set("name", "team / weird:*")
	AdminGroupGET(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	mock.Ctx.QueryArgs().Del("name")

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"name":"team / weird:*","username":"admin","expected_version":` + jsonInt(group.Version) + `}`)
	AdminGroupUserPUT(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	group, _, err = store.LoadMTLAdminGroup(context.Background(), "team / weird:*")
	require.NoError(t, err)
	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"name":"team / weird:*","username":"admin","expected_version":` + jsonInt(group.Version) + `,"confirm_username":"admin"}`)
	AdminGroupUserDELETE(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	group, _, err = store.LoadMTLAdminGroup(context.Background(), "team / weird:*")
	require.NoError(t, err)
	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"name":"team / weird:*","username":"admin","expected_version":` + jsonInt(group.Version) + `}`)
	AdminGroupUserPUT(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	group, _, err = store.LoadMTLAdminGroup(context.Background(), "team / weird:*")
	require.NoError(t, err)

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"name":"team / weird:*","new_name":"app:grafana","expected_version":` + jsonInt(group.Version) + `,"confirm_username":"admin"}`)
	AdminGroupPATCH(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.Contains(t, string(mock.Ctx.Response.Body()), "external_acl_not_updated")
	assert.Contains(t, string(mock.Ctx.Response.Body()), "admin")

	group, _, err = store.LoadMTLAdminGroup(context.Background(), "app:grafana")
	require.NoError(t, err)
	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"name":"app:grafana","expected_version":` + jsonInt(group.Version) + `,"confirm_username":"admin"}`)
	AdminGroupDELETE(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.Contains(t, string(mock.Ctx.Response.Body()), "external_acl_not_updated")
}

func TestAdminGroupsProtectConfiguredApplicationGroupsFromStructuralChanges(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	mock.Ctx.Configuration.Applications = []schema.Application{{Slug: "grafana", Name: "Grafana", Domain: "grafana.example.com"}}
	require.NoError(t, store.ReconcileMTLGroups(t.Context(), []string{"app:grafana"}))
	group, found, err := store.LoadMTLAdminGroup(t.Context(), "app:grafana")
	require.NoError(t, err)
	require.True(t, found)

	AdminGroupsGET(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	var listResponse struct {
		Data []struct {
			Name    string `json:"name"`
			Managed bool   `json:"managed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(mock.Ctx.Response.Body(), &listResponse))
	require.Len(t, listResponse.Data, 1)
	assert.Equal(t, "app:grafana", listResponse.Data[0].Name)
	assert.True(t, listResponse.Data[0].Managed)

	for _, mutation := range []struct {
		body string
		run  func(*middlewares.AutheliaCtx)
	}{
		{body: `{"name":"app:grafana","new_name":"renamed","expected_version":` + jsonInt(group.Version) + `}`, run: AdminGroupPATCH},
		{body: `{"name":"app:grafana","expected_version":` + jsonInt(group.Version) + `}`, run: AdminGroupDELETE},
	} {
		mock.Ctx.Response.Reset()
		mock.Ctx.Request.SetBodyString(mutation.body)
		mutation.run(mock.Ctx)
		assert.Equal(t, fasthttp.StatusConflict, mock.Ctx.Response.StatusCode())
	}

	_, found, err = store.LoadMTLAdminGroup(t.Context(), "app:grafana")
	require.NoError(t, err)
	assert.True(t, found)
	_, found, err = store.LoadMTLAdminGroup(t.Context(), "renamed")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestAdminGroupUserMutationRefreshesCurrentAdministratorSession(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	group, err := store.CreateMTLAdminGroup(t.Context(), "app:grafana", "admin")
	require.NoError(t, err)

	userSession, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	epoch := 0
	userSession.SessionEpoch = &epoch
	userSession.FirstFactorAuthnTimestamp = 123
	require.NoError(t, mock.Ctx.SaveSession(userSession))

	mock.Ctx.Request.SetBodyString(`{"name":"app:grafana","username":"admin","expected_version":` + jsonInt(group.Version) + `}`)
	AdminGroupUserPUT(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())

	updated, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	require.NotNil(t, updated.SessionEpoch)
	assert.Equal(t, 1, *updated.SessionEpoch)
	assert.Equal(t, []string{"app:grafana", "admins"}, updated.Groups)
	assert.Equal(t, int64(123), updated.FirstFactorAuthnTimestamp)
	assert.True(t, updated.AuthenticationMethodRefs.External)
}

func TestAdminGroupManagerMutation(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	_, err := store.CreateMTLAdminUser(t.Context(), model.MTLAdminUserCreate{Username: "manager", Email: "manager@example.com"}, "admin")
	require.NoError(t, err)
	group, err := store.CreateMTLAdminGroup(t.Context(), "app:grafana", "admin")
	require.NoError(t, err)

	mock.Ctx.Request.SetBodyString(`{"name":"app:grafana","username":"manager","expected_version":` + jsonInt(group.Version) + `}`)
	AdminGroupManagerPUT(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.Contains(t, string(mock.Ctx.Response.Body()), `"managers":["manager"]`)

	group, _, err = store.LoadMTLAdminGroup(t.Context(), group.Name)
	require.NoError(t, err)
	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"name":"app:grafana","username":"manager","expected_version":` + jsonInt(group.Version) + `}`)
	AdminGroupManagerDELETE(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.Contains(t, string(mock.Ctx.Response.Body()), `"managers":[]`)
}
