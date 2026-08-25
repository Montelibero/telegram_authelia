package handlers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
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
