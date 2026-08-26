package middlewares_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/authentication"
	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/mocks"
)

func TestRequireAdminRejectsRevokedMTLSession(t *testing.T) {
	mock := mocks.NewMockAutheliaCtx(t)
	defer mock.Close()
	sessionEpoch := 1
	currentEpoch := 2
	userSession, err := mock.Ctx.GetSession()
	require.NoError(t, err)
	userSession.Username = john
	userSession.Groups = []string{"admins"}
	userSession.SessionEpoch = &sessionEpoch
	userSession.AuthenticationMethodRefs.External = true
	require.NoError(t, mock.Ctx.SaveSession(userSession))
	mock.Ctx.Providers.UserProvider = mtlTestUserProvider{mock.UserProviderMock}
	mock.UserProviderMock.EXPECT().GetDetails(john).Return(&authentication.UserDetails{Username: john, SessionEpoch: &currentEpoch}, nil)

	middlewares.RequireAdmin(NilHandler)(mock.Ctx)

	assert.Equal(t, fasthttp.StatusUnauthorized, mock.Ctx.Response.StatusCode())
}

func TestRequireAdmin(t *testing.T) {
	tests := []struct {
		name     string
		username string
		groups   []string
		expected int
	}{
		{name: "anonymous", expected: fasthttp.StatusUnauthorized},
		{name: "non-admin", username: john, groups: []string{"users"}, expected: fasthttp.StatusForbidden},
		{name: "admin", username: john, groups: []string{"users", "admins"}, expected: fasthttp.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := mocks.NewMockAutheliaCtx(t)
			defer mock.Close()
			if tc.username != "" {
				userSession, err := mock.Ctx.GetSession()
				require.NoError(t, err)
				userSession.Username = tc.username
				userSession.Groups = tc.groups
				userSession.AuthenticationMethodRefs.External = true
				require.NoError(t, mock.Ctx.SaveSession(userSession))
			}

			middlewares.RequireAdmin(NilHandler)(mock.Ctx)

			assert.Equal(t, tc.expected, mock.Ctx.Response.StatusCode())
		})
	}
}

func TestRequireAdminMutation(t *testing.T) {
	tests := []struct {
		name     string
		password bool
		fresh    bool
		expected int
	}{
		{name: "fresh telegram admin without password", fresh: true, expected: fasthttp.StatusOK},
		{name: "fresh password without email elevation", password: true, fresh: true, expected: fasthttp.StatusOK},
		{name: "stale password", password: true, expected: fasthttp.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := mocks.NewMockAutheliaCtx(t)
			defer mock.Close()
			mock.Ctx.Configuration.IdentityValidation.ElevatedSession.ElevationLifespan = time.Minute
			mock.Ctx.Configuration.IdentityValidation.ElevatedSession.DisableOneTimeCode = true
			mock.Ctx.Providers.Clock = &mock.Clock
			mock.Ctx.Request.Header.Set(fasthttp.HeaderXForwardedFor, "127.0.0.1")
			mock.Ctx.Request.Header.SetHost("auth.example.com")
			mock.Ctx.Request.Header.Set(fasthttp.HeaderXForwardedHost, "auth.example.com")
			mock.Ctx.Request.Header.Set("Origin", "https://auth.example.com")
			mock.Ctx.Request.Header.Set(fasthttp.HeaderXForwardedProto, "https")
			userSession, err := mock.Ctx.GetSession()
			require.NoError(t, err)
			userSession.Username = john
			userSession.Groups = []string{"admins"}
			userSession.AuthenticationMethodRefs.External = true
			userSession.AuthenticationMethodRefs.UsernameAndPassword = tc.password
			if tc.fresh {
				userSession.FirstFactorAuthnTimestamp = mock.Clock.Now().Unix()
			} else {
				userSession.FirstFactorAuthnTimestamp = mock.Clock.Now().Add(-time.Hour).Unix()
			}
			require.NoError(t, mock.Ctx.SaveSession(userSession))

			middlewares.RequireAdminMutation(NilHandler)(mock.Ctx)

			assert.Equal(t, tc.expected, mock.Ctx.Response.StatusCode())
		})
	}
}

func TestRequireAdminMutationSameOrigin(t *testing.T) {
	tests := []struct {
		name     string
		origin   string
		proto    string
		expected int
	}{
		{name: "missing origin", expected: fasthttp.StatusForbidden},
		{name: "cross origin", origin: "https://evil.example.com", proto: "https", expected: fasthttp.StatusForbidden},
		{name: "same forwarded HTTPS origin", origin: "https://auth.example.com", proto: "https", expected: fasthttp.StatusOK},
		{name: "same direct HTTP origin", origin: "http://auth.example.com", expected: fasthttp.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := mocks.NewMockAutheliaCtx(t)
			defer mock.Close()
			mock.Ctx.Configuration.IdentityValidation.ElevatedSession.ElevationLifespan = time.Minute
			mock.Ctx.Configuration.IdentityValidation.ElevatedSession.DisableOneTimeCode = true
			mock.Ctx.Providers.Clock = &mock.Clock
			mock.Ctx.Request.Header.SetHost("auth.example.com")
			mock.Ctx.Request.Header.Set(fasthttp.HeaderXForwardedHost, "auth.example.com")
			mock.Ctx.Request.Header.Del(fasthttp.HeaderXForwardedProto)
			if tc.proto != "" {
				mock.Ctx.Request.Header.Set(fasthttp.HeaderXForwardedProto, tc.proto)
			}
			mock.Ctx.Request.Header.Set(fasthttp.HeaderXForwardedFor, "127.0.0.1")
			if tc.origin != "" {
				mock.Ctx.Request.Header.Set("Origin", tc.origin)
			}
			assert.Equal(t, "auth.example.com", string(mock.Ctx.GetXForwardedHost()))
			userSession, err := mock.Ctx.GetSession()
			require.NoError(t, err)
			userSession.Username = john
			userSession.Groups = []string{"admins"}
			userSession.AuthenticationMethodRefs.External = true
			userSession.AuthenticationMethodRefs.UsernameAndPassword = true
			userSession.FirstFactorAuthnTimestamp = mock.Clock.Now().Unix()
			require.NoError(t, mock.Ctx.SaveSession(userSession))

			middlewares.RequireAdminMutation(NilHandler)(mock.Ctx)

			assert.Equal(t, tc.expected, mock.Ctx.Response.StatusCode())
		})
	}
}

func TestRequireSameOriginMutation(t *testing.T) {
	for _, tc := range []struct {
		name, origin string
		expected     int
	}{
		{name: "missing origin", expected: fasthttp.StatusForbidden},
		{name: "cross origin", origin: "https://evil.example.com", expected: fasthttp.StatusForbidden},
		{name: "same origin", origin: "https://auth.example.com", expected: fasthttp.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := mocks.NewMockAutheliaCtx(t)
			defer mock.Close()
			mock.Ctx.Request.Header.SetHost("auth.example.com")
			mock.Ctx.Request.Header.Set(fasthttp.HeaderXForwardedHost, "auth.example.com")
			mock.Ctx.Request.Header.Set(fasthttp.HeaderXForwardedProto, "https")
			if tc.origin != "" {
				mock.Ctx.Request.Header.Set("Origin", tc.origin)
			}

			middlewares.RequireSameOriginMutation(NilHandler)(mock.Ctx)

			assert.Equal(t, tc.expected, mock.Ctx.Response.StatusCode())
		})
	}
}
