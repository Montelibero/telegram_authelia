package handlers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/mocks"
	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/storage"
)

func TestAdminManagerCapabilitiesAndScopedGroups(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	managed := makeAdminManager(t, mock, store, "manager", "app:grafana")
	other, err := store.CreateMTLAdminGroup(t.Context(), "app:portainer", "admin")
	require.NoError(t, err)

	AdminGET(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.Contains(t, string(mock.Ctx.Response.Body()), `"full_administrator":false`)
	assert.Contains(t, string(mock.Ctx.Response.Body()), `"managed_groups":["app:grafana"]`)

	mock.Ctx.Response.Reset()
	AdminGroupsGET(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.Contains(t, string(mock.Ctx.Response.Body()), "app:grafana")
	assert.NotContains(t, string(mock.Ctx.Response.Body()), "app:portainer")

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"name":"app:grafana","username":"admin","expected_version":` + jsonInt(managed.Version) + `}`)
	AdminGroupUserPUT(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"name":"app:portainer","username":"admin","expected_version":` + jsonInt(other.Version) + `}`)
	AdminGroupUserPUT(mock.Ctx)
	assert.Equal(t, fasthttp.StatusForbidden, mock.Ctx.Response.StatusCode())
}

func TestAdminManagerUserMutationScope(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	managed := makeAdminManager(t, mock, store, "manager", "app:grafana")
	other, err := store.CreateMTLAdminGroup(t.Context(), "app:portainer", "admin")
	require.NoError(t, err)
	target, err := store.CreateMTLAdminUser(t.Context(), model.MTLAdminUserCreate{
		Username: "target", DisplayName: "Target", Email: "target@example.com", Groups: []string{managed.Name, other.Name},
	}, "admin")
	require.NoError(t, err)

	mock.Ctx.Request.SetBodyString(`{"username":"target","display_name":"Renamed","status":"active","expected_version":` + jsonInt(target.Version) + `}`)
	AdminUserPATCH(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())

	target, _, err = store.LoadMTLAdminUser(t.Context(), "target")
	require.NoError(t, err)
	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"username":"target","display_name":"Renamed","status":"disabled","expected_version":` + jsonInt(target.Version) + `}`)
	AdminUserPATCH(mock.Ctx)
	assert.Equal(t, fasthttp.StatusForbidden, mock.Ctx.Response.StatusCode())

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"username":"target","provider":"telegram","provider_user_id":"123","expected_version":` + jsonInt(target.Version) + `}`)
	AdminUserIdentityPUT(mock.Ctx)
	assert.Equal(t, fasthttp.StatusForbidden, mock.Ctx.Response.StatusCode())
}

func TestAdminManagerCreationApprovalAndRejectionScope(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	managed := makeAdminManager(t, mock, store, "manager", "app:grafana")
	other, err := store.CreateMTLAdminGroup(t.Context(), "app:portainer", "admin")
	require.NoError(t, err)

	mock.Ctx.Request.SetBodyString(`{"username":"own","email":"own@example.com","groups":["app:grafana"]}`)
	AdminUserPOST(mock.Ctx)
	assert.Equal(t, fasthttp.StatusCreated, mock.Ctx.Response.StatusCode())

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"username":"foreign","email":"foreign@example.com","groups":["app:portainer"]}`)
	AdminUserPOST(mock.Ctx)
	assert.Equal(t, fasthttp.StatusForbidden, mock.Ctx.Response.StatusCode())

	registration, err := store.UpsertMTLRegistration(context.Background(), model.MTLRegistrationCandidate{
		Provider: "telegram", ProviderUserID: "42", ProviderUsername: "pending",
	})
	require.NoError(t, err)
	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"id":` + jsonInt64(registration.ID) + `,"expected_version":` + jsonInt(registration.Version) + `,"groups":["app:portainer"]}`)
	AdminRegistrationApprovePOST(mock.Ctx)
	assert.Equal(t, fasthttp.StatusForbidden, mock.Ctx.Response.StatusCode())

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"id":` + jsonInt64(registration.ID) + `,"expected_version":` + jsonInt(registration.Version) + `}`)
	AdminRegistrationRejectPOST(mock.Ctx)
	assert.Equal(t, fasthttp.StatusForbidden, mock.Ctx.Response.StatusCode())

	_, _ = managed, other
}

func makeAdminManager(t *testing.T, mock *mocks.MockAutheliaCtx, store *storage.SQLiteProvider, username, groupName string) model.MTLAdminGroupDetails {
	t.Helper()
	_, err := store.CreateMTLAdminUser(t.Context(), model.MTLAdminUserCreate{Username: username, Email: username + "@example.com"}, "admin")
	require.NoError(t, err)
	group, err := store.CreateMTLAdminGroup(t.Context(), groupName, "admin")
	require.NoError(t, err)
	group, err = store.AddMTLAdminGroupManager(t.Context(), groupName, username, group.Version, "admin")
	require.NoError(t, err)
	userSession, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	userSession.Username = username
	userSession.Groups = nil
	userSession.AuthenticationMethodRefs.External = true
	require.NoError(t, mock.Ctx.SaveSession(userSession))
	return group
}
