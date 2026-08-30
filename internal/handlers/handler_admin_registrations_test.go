package handlers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/model"
)

func TestAdminRegistrationsAPIWorkflow(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	_, err := store.CreateMTLAdminGroup(context.Background(), "readers", "admin")
	require.NoError(t, err)
	approval, err := store.UpsertMTLRegistration(context.Background(), model.MTLRegistrationCandidate{
		Provider: "telegram", ProviderUserID: "42", ProviderUsername: "bublik",
		DisplayName: "Original", ProposedUsername: "bublik", ProposedEmail: "bublik@example.com",
	})
	require.NoError(t, err)
	rejection, err := store.UpsertMTLRegistration(context.Background(), model.MTLRegistrationCandidate{
		Provider: "telegram", ProviderUserID: "43", ProviderUsername: "reject_me",
		ProposedUsername: "reject_me", ProposedEmail: "reject@example.com",
	})
	require.NoError(t, err)

	mock.Ctx.QueryArgs().Set("status", "pending")
	AdminRegistrationsGET(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.Contains(t, string(mock.Ctx.Response.Body()), `"provider_user_id":"42"`)
	assert.NotContains(t, string(mock.Ctx.Response.Body()), "resolved_by_user_id")
	assert.NotContains(t, string(mock.Ctx.Response.Body()), "approved_user_id")
	mock.Ctx.Response.Reset()
	mock.Ctx.QueryArgs().Del("status")
	mock.Ctx.QueryArgs().Set("id", jsonInt64(approval.ID))
	AdminRegistrationGET(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	mock.Ctx.QueryArgs().Del("id")

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"id":` + jsonInt64(approval.ID) + `,"expected_version":` + jsonInt(approval.Version) + `,"username":"edited","display_name":"Edited Name","email":"edited@example.com","groups":["readers"]}`)
	AdminRegistrationApprovePOST(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	details, found, err := store.LoadMTLAdminUser(context.Background(), "edited")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "Edited Name", details.DisplayName)
	assert.Equal(t, []string{"readers"}, details.Groups)

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"id":` + jsonInt64(rejection.ID) + `,"expected_version":` + jsonInt(rejection.Version) + `}`)
	AdminRegistrationRejectPOST(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBody(nil)
	mock.Ctx.QueryArgs().Set("status", "pending")
	AdminRegistrationsGET(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.NotContains(t, string(mock.Ctx.Response.Body()), `"provider_user_id":"43"`)
	mock.Ctx.QueryArgs().Del("status")

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"id":` + jsonInt64(rejection.ID) + `,"expected_version":` + jsonInt(rejection.Version) + `}`)
	AdminRegistrationRejectPOST(mock.Ctx)
	assert.Equal(t, fasthttp.StatusConflict, mock.Ctx.Response.StatusCode())
}

func TestAdminRegistrationsRejectsInvalidStatus(t *testing.T) {
	mock, _ := newAdminAPITestContext(t)
	mock.Ctx.QueryArgs().Set("status", "unknown")
	AdminRegistrationsGET(mock.Ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, mock.Ctx.Response.StatusCode())
}
