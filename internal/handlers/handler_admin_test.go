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
			Username          string   `json:"username"`
			MutationReady     bool     `json:"mutation_ready"`
			FullAdministrator bool     `json:"full_administrator"`
			ManagedGroups     []string `json:"managed_groups"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(mock.Ctx.Response.Body(), &response))
	assert.Equal(t, "OK", response.Status)
	assert.Equal(t, "admin", response.Data.Username)
	assert.False(t, response.Data.MutationReady)
	assert.True(t, response.Data.FullAdministrator)
	assert.Empty(t, response.Data.ManagedGroups)
	assert.NotContains(t, string(mock.Ctx.Response.Body()), "session_epoch")
}

func TestAdminGETReportsFreshAuthenticationReadyForMutation(t *testing.T) {
	testCases := []struct {
		name     string
		password bool
		passkey  bool
		external bool
	}{
		{name: "Password", password: true},
		{name: "Passkey", passkey: true},
		{name: "Telegram", external: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := mocks.NewMockAutheliaCtx(t)
			defer mock.Close()
			mock.Ctx.Providers.Clock = &mock.Clock
			mock.Ctx.Configuration.IdentityValidation.ElevatedSession.ElevationLifespan = time.Minute
			mock.Ctx.Configuration.IdentityValidation.ElevatedSession.DisableOneTimeCode = true
			userSession, err := mock.Ctx.GetSession()
			require.NoError(t, err)
			userSession.Username = "admin"
			userSession.Groups = []string{"admins"}
			userSession.AuthenticationMethodRefs.UsernameAndPassword = tc.password
			userSession.AuthenticationMethodRefs.WebAuthn = tc.passkey
			userSession.AuthenticationMethodRefs.External = tc.external
			userSession.FirstFactorAuthnTimestamp = mock.Clock.Now().Unix()
			require.NoError(t, mock.Ctx.SaveSession(userSession))

			AdminGET(mock.Ctx)

			assert.Contains(t, string(mock.Ctx.Response.Body()), `"mutation_ready":true`)
		})
	}
}
