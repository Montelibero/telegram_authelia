package handlers

import (
	"context"
	"encoding/json"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/mocks"
	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/storage"
)

func TestAdminUsersAPIWorkflow(t *testing.T) {
	mock, store := newAdminAPITestContext(t)

	mock.Ctx.Request.SetBodyString(`{"username":"bublik","display_name":"Bublik","email":"bublik@example.com","telegram_id":"42"}`)
	AdminUserPOST(mock.Ctx)
	assert.Equal(t, fasthttp.StatusCreated, mock.Ctx.Response.StatusCode())

	mock.Ctx.Response.Reset()
	AdminUsersGET(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.Contains(t, string(mock.Ctx.Response.Body()), "bublik@example.com")
	assert.NotContains(t, string(mock.Ctx.Response.Body()), "password_hash")
	mock.Ctx.Response.Reset()
	mock.Ctx.QueryArgs().Set("username", "bublik")
	AdminUserGET(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	mock.Ctx.QueryArgs().Del("username")

	details, found, err := store.LoadMTLAdminUser(context.Background(), "bublik")
	require.NoError(t, err)
	require.True(t, found)
	resolved, found, err := store.LoadMTLUserByIdentity(context.Background(), "telegram", "42")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "bublik", resolved.User.Username)
	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"username":"bublik","expected_version":` + jsonInt(details.Version) + `,"email":"other@example.com","primary":true}`)
	AdminUserEmailPOST(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	details, _, err = store.LoadMTLAdminUser(context.Background(), "bublik")
	require.NoError(t, err)
	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"username":"bublik","expected_version":` + jsonInt(details.Version) + `,"email":"other@example.com"}`)
	AdminUserEmailPrimaryPUT(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	details, _, err = store.LoadMTLAdminUser(context.Background(), "bublik")
	require.NoError(t, err)
	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"username":"bublik","expected_version":` + jsonInt(details.Version) + `,"email":"bublik@example.com"}`)
	AdminUserEmailDELETE(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())

	details, _, err = store.LoadMTLAdminUser(context.Background(), "bublik")
	require.NoError(t, err)
	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"username":"bublik","provider":"telegram","expected_version":` + jsonInt(details.Version) + `}`)
	AdminUserIdentityDELETE(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())

	details, _, err = store.LoadMTLAdminUser(context.Background(), "bublik")
	require.NoError(t, err)
	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"username":"bublik","display_name":"Bublik Disabled","status":"disabled","expected_version":` + jsonInt(details.Version+1) + `}`)
	AdminUserPATCH(mock.Ctx)
	assert.Equal(t, fasthttp.StatusConflict, mock.Ctx.Response.StatusCode())
}

func TestAdminUserCanDirectlyLinkTelegramID(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	created, err := store.CreateMTLAdminUser(context.Background(), model.MTLAdminUserCreate{Username: "target", Email: "target@example.com"}, "admin")
	require.NoError(t, err)

	mock.Ctx.Request.SetBodyString(`{"username":"target","provider":"telegram","provider_user_id":"987654321","expected_version":` + jsonInt(created.Version) + `}`)
	AdminUserIdentityPUT(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())

	resolved, found, err := store.LoadMTLUserByIdentity(context.Background(), "telegram", "987654321")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "target", resolved.User.Username)

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"username":"admin","provider":"telegram","provider_user_id":"987654321","expected_version":0}`)
	AdminUserIdentityPUT(mock.Ctx)
	assert.Equal(t, fasthttp.StatusConflict, mock.Ctx.Response.StatusCode())
}

func TestAdminUserCreateRequiresEmailOrTelegramID(t *testing.T) {
	mock, _ := newAdminAPITestContext(t)

	mock.Ctx.Request.SetBodyString(`{"username":"unused"}`)
	AdminUserPOST(mock.Ctx)

	assert.Equal(t, fasthttp.StatusBadRequest, mock.Ctx.Response.StatusCode())
}

func TestAdminUserSelfDisableRequiresTypedUsername(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	details, found, err := store.LoadMTLAdminUser(context.Background(), "admin")
	require.NoError(t, err)
	require.True(t, found)

	mock.Ctx.Request.SetBodyString(`{"username":"admin","display_name":"Admin","status":"disabled","expected_version":` + jsonInt(details.Version) + `}`)
	AdminUserPATCH(mock.Ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, mock.Ctx.Response.StatusCode())

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"username":"admin","display_name":"Admin","status":"disabled","expected_version":` + jsonInt(details.Version) + `,"confirm_username":"admin"}`)
	AdminUserPATCH(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
}

func TestAdminUserSelfIdentityUnlinkRequiresConfirmationOnlyForFinalLoginMethod(t *testing.T) {
	mock, store := newAdminAPITestContext(t)
	require.NoError(t, store.LinkMTLUserIdentity(context.Background(), "admin", "telegram", "42", "admin"))

	details, found, err := store.LoadMTLAdminUser(context.Background(), "admin")
	require.NoError(t, err)
	require.True(t, found)
	mock.Ctx.Request.SetBodyString(`{"username":"admin","provider":"telegram","expected_version":` + jsonInt(details.Version) + `}`)
	AdminUserIdentityDELETE(mock.Ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, mock.Ctx.Response.StatusCode())

	authDetails, found, err := store.LoadMTLUser(context.Background(), "admin")
	require.NoError(t, err)
	require.True(t, found)
	hash := "$plaintext$password"
	require.NoError(t, store.UpdateMTLUserPassword(context.Background(), authDetails.User.ID, &hash, authDetails.User.Version))
	details, found, err = store.LoadMTLAdminUser(context.Background(), "admin")
	require.NoError(t, err)
	require.True(t, found)

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"username":"admin","provider":"telegram","expected_version":` + jsonInt(details.Version) + `}`)
	AdminUserIdentityDELETE(mock.Ctx)
	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
}

func TestAdminUserSetupLinkUsesResetPasswordCompletionWithoutEmail(t *testing.T) {
	mock, _ := newAdminAPITestContext(t)
	mock.Ctx.Configuration.IdentityValidation.ResetPassword.JWTSecret = "test-secret"
	mock.Ctx.Configuration.IdentityValidation.ResetPassword.JWTAlgorithm = "HS256"
	mock.Ctx.Configuration.IdentityValidation.ResetPassword.JWTExpiration = 5 * time.Minute
	mock.Ctx.Configuration.Session.Cookies = []schema.SessionCookie{{Domain: "example.com"}}
	mock.Ctx.Request.Header.SetHost("login.example.com")
	mock.Ctx.Request.Header.Set(fasthttp.HeaderXForwardedHost, "login.example.com")
	mock.Ctx.Request.Header.Set(fasthttp.HeaderXForwardedProto, "https")
	mock.Ctx.SetUserValue(middlewares.UserValueKeyBaseURL, "/auth")
	mock.Ctx.Request.SetBodyString(`{"username":"admin"}`)
	_, err := mock.Ctx.IssuerURL()
	require.NoError(t, err)

	AdminUserSetupLinkPOST(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	assert.Equal(t, "no-store", string(mock.Ctx.Response.Header.Peek("Cache-Control")))
	var response struct {
		Data struct {
			SetupURL string `json:"setup_url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(mock.Ctx.Response.Body(), &response))
	setupURL, err := url.Parse(response.Data.SetupURL)
	require.NoError(t, err)
	assert.Equal(t, "/auth/reset-password/step2", setupURL.Path)
	token := setupURL.Query().Get("token")
	require.NotEmpty(t, token)

	mock.Ctx.Response.Reset()
	mock.Ctx.Request.SetBodyString(`{"token":"` + token + `"}`)
	ResetPasswordIdentityFinish(mock.Ctx)
	require.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	userSession, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	require.NotNil(t, userSession.PasswordResetUsername)
	assert.Equal(t, "admin", *userSession.PasswordResetUsername)
	assert.Empty(t, mock.Ctx.Providers.StorageProvider.(adminAPITestStore).identityVerifications)
}

func newAdminAPITestContext(t *testing.T) (*mocks.MockAutheliaCtx, *storage.SQLiteProvider) {
	t.Helper()
	mock := mocks.NewMockAutheliaCtx(t)
	t.Cleanup(mock.Close)
	config := &schema.Configuration{Storage: schema.Storage{Local: &schema.StorageLocal{Path: filepath.Join(t.TempDir(), "db.sqlite3")}}}
	store := storage.NewSQLiteProvider(config)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.MigrateMTL(context.Background()))
	_, err := store.CreateMTLAdminUser(context.Background(), model.MTLAdminUserCreate{Username: "admin", Email: "admin@example.com", Groups: nil}, "")
	require.NoError(t, err)
	mock.Ctx.Providers.StorageProvider = adminAPITestStore{SQLiteProvider: store, identityVerifications: map[string]bool{}}
	userSession, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	userSession.Username = "admin"
	userSession.Groups = []string{"admins"}
	userSession.AuthenticationMethodRefs.External = true
	require.NoError(t, mock.Ctx.SaveSession(userSession))
	return mock, store
}

type adminAPITestStore struct {
	*storage.SQLiteProvider
	identityVerifications map[string]bool
}

func (s adminAPITestStore) ListMTLAdminUsers(context.Context) ([]model.MTLAdminUserSummary, error) {
	return s.SQLiteProvider.ListMTLAdminUsers(context.Background())
}

func (s adminAPITestStore) LoadMTLAdminUser(_ context.Context, username string) (model.MTLAdminUserDetails, bool, error) {
	return s.SQLiteProvider.LoadMTLAdminUser(context.Background(), username)
}

func (s adminAPITestStore) CreateMTLAdminUser(_ context.Context, create model.MTLAdminUserCreate, actor string) (model.MTLAdminUserDetails, error) {
	return s.SQLiteProvider.CreateMTLAdminUser(context.Background(), create, actor)
}

func (s adminAPITestStore) UpdateMTLAdminUser(_ context.Context, username string, update model.MTLAdminUserUpdate, actor string) (model.MTLAdminUserDetails, error) {
	return s.SQLiteProvider.UpdateMTLAdminUser(context.Background(), username, update, actor)
}

func (s adminAPITestStore) AddMTLAdminUserEmail(_ context.Context, username string, create model.MTLAdminEmailCreate, actor string) (model.MTLAdminUserDetails, error) {
	return s.SQLiteProvider.AddMTLAdminUserEmail(context.Background(), username, create, actor)
}

func (s adminAPITestStore) SetMTLAdminPrimaryEmail(_ context.Context, username, email string, version int, actor string) (model.MTLAdminUserDetails, error) {
	return s.SQLiteProvider.SetMTLAdminPrimaryEmail(context.Background(), username, email, version, actor)
}

func (s adminAPITestStore) DeleteMTLAdminUserEmail(_ context.Context, username, email string, version int, actor string) (model.MTLAdminUserDetails, error) {
	return s.SQLiteProvider.DeleteMTLAdminUserEmail(context.Background(), username, email, version, actor)
}

func (s adminAPITestStore) UnlinkMTLAdminUserIdentity(_ context.Context, username, provider string, version int, actor string) (model.MTLAdminUserDetails, error) {
	return s.SQLiteProvider.UnlinkMTLAdminUserIdentity(context.Background(), username, provider, version, actor)
}

func (s adminAPITestStore) LinkMTLAdminUserIdentity(_ context.Context, username string, link model.MTLAdminIdentityLink, actor string) (model.MTLAdminUserDetails, error) {
	return s.SQLiteProvider.LinkMTLAdminUserIdentity(context.Background(), username, link, actor)
}

func (s adminAPITestStore) ListMTLAdminGroups(context.Context) ([]model.MTLAdminGroupSummary, error) {
	return s.SQLiteProvider.ListMTLAdminGroups(context.Background())
}

func (s adminAPITestStore) LoadMTLAdminGroup(_ context.Context, name string) (model.MTLAdminGroupDetails, bool, error) {
	return s.SQLiteProvider.LoadMTLAdminGroup(context.Background(), name)
}

func (s adminAPITestStore) CreateMTLAdminGroup(_ context.Context, name, actor string) (model.MTLAdminGroupDetails, error) {
	return s.SQLiteProvider.CreateMTLAdminGroup(context.Background(), name, actor)
}

func (s adminAPITestStore) RenameMTLAdminGroup(_ context.Context, name, newName string, version int, actor string) (model.MTLAdminGroupDetails, []string, error) {
	return s.SQLiteProvider.RenameMTLAdminGroup(context.Background(), name, newName, version, actor)
}

func (s adminAPITestStore) DeleteMTLAdminGroup(_ context.Context, name string, version int, actor string) ([]string, error) {
	return s.SQLiteProvider.DeleteMTLAdminGroup(context.Background(), name, version, actor)
}

func (s adminAPITestStore) AddMTLAdminGroupUser(_ context.Context, name, username string, version int, actor string) (model.MTLAdminGroupDetails, error) {
	return s.SQLiteProvider.AddMTLAdminGroupUser(context.Background(), name, username, version, actor)
}

func (s adminAPITestStore) RemoveMTLAdminGroupUser(_ context.Context, name, username string, version int, actor string) (model.MTLAdminGroupDetails, error) {
	return s.SQLiteProvider.RemoveMTLAdminGroupUser(context.Background(), name, username, version, actor)
}

func (s adminAPITestStore) ListMTLRegistrations(_ context.Context, status model.MTLRegistrationStatus) ([]model.MTLRegistrationRequest, error) {
	return s.SQLiteProvider.ListMTLRegistrations(context.Background(), status)
}

func (s adminAPITestStore) LoadMTLRegistration(_ context.Context, id int64) (model.MTLRegistrationRequest, bool, error) {
	return s.SQLiteProvider.LoadMTLRegistration(context.Background(), id)
}

func (s adminAPITestStore) ApproveMTLRegistration(_ context.Context, approval model.MTLRegistrationApproval) (string, error) {
	return s.SQLiteProvider.ApproveMTLRegistration(context.Background(), approval)
}

func (s adminAPITestStore) RejectMTLRegistration(_ context.Context, id int64, version int, actor string) (model.MTLRegistrationRequest, error) {
	return s.SQLiteProvider.RejectMTLRegistration(context.Background(), id, version, actor)
}

func (s adminAPITestStore) SaveIdentityVerification(_ context.Context, verification model.IdentityVerification) error {
	s.identityVerifications[verification.JTI.String()] = true
	return nil
}

func (s adminAPITestStore) FindIdentityVerification(_ context.Context, jti string) (bool, error) {
	return s.identityVerifications[jti], nil
}

func (s adminAPITestStore) ConsumeIdentityVerification(_ context.Context, jti string, ip model.NullIP) error {
	delete(s.identityVerifications, jti)
	return nil
}

func jsonInt(value int) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func jsonInt64(value int64) string {
	data, _ := json.Marshal(value)
	return string(data)
}
