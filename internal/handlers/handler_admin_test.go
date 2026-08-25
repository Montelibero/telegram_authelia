package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/mocks"
)

func TestAdminGETReturnsSafeSessionCapabilities(t *testing.T) {
	mock := mocks.NewMockAutheliaCtx(t)
	defer mock.Close()
	userSession, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	userSession.Username = "admin"
	userSession.Groups = []string{"admins"}
	userSession.AuthenticationMethodRefs.External = true
	require.NoError(t, mock.Ctx.SaveSession(userSession))

	AdminGET(mock.Ctx)

	assert.Equal(t, fasthttp.StatusOK, mock.Ctx.Response.StatusCode())
	var response struct {
		Status string `json:"status"`
		Data   struct {
			Username      string `json:"username"`
			PasswordFresh bool   `json:"password_fresh"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(mock.Ctx.Response.Body(), &response))
	assert.Equal(t, "OK", response.Status)
	assert.Equal(t, "admin", response.Data.Username)
	assert.False(t, response.Data.PasswordFresh)
	assert.NotContains(t, string(mock.Ctx.Response.Body()), "groups")
}

func TestAdminGETReportsFreshPasswordProof(t *testing.T) {
	mock := mocks.NewMockAutheliaCtx(t)
	defer mock.Close()
	mock.Ctx.Providers.Clock = &mock.Clock
	mock.Ctx.Configuration.IdentityValidation.ElevatedSession.ElevationLifespan = time.Minute
	userSession, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	userSession.Username = "admin"
	userSession.Groups = []string{"admins"}
	userSession.AuthenticationMethodRefs.External = true
	userSession.AuthenticationMethodRefs.UsernameAndPassword = true
	userSession.FirstFactorAuthnTimestamp = mock.Clock.Now().Unix()
	require.NoError(t, mock.Ctx.SaveSession(userSession))

	AdminGET(mock.Ctx)

	assert.Contains(t, string(mock.Ctx.Response.Body()), `"password_fresh":true`)
}
