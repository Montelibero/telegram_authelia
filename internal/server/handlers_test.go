package server

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/oidc"
	"github.com/authelia/authelia/v4/internal/random"
	"github.com/authelia/authelia/v4/internal/session"
	"github.com/authelia/authelia/v4/internal/templates"
)

func TestHandleError(t *testing.T) {
	handler := handleError("server")

	testCases := []struct {
		name               string
		err                error
		xff                string
		isTLS              bool
		expectedStatusCode int
	}{
		{
			"ShouldHandleSmallBufferError",
			&fasthttp.ErrSmallBuffer{},
			"",
			false,
			fasthttp.StatusRequestHeaderFieldsTooLarge,
		},
		{
			"ShouldHandleNetOpErrorTimeout",
			&net.OpError{Op: "read", Err: &timeoutError{}},
			"",
			false,
			fasthttp.StatusRequestTimeout,
		},
		{
			"ShouldHandleNetOpErrorNonTimeout",
			&net.OpError{Op: "read", Err: fmt.Errorf("connection reset")},
			"",
			false,
			fasthttp.StatusBadRequest,
		},
		{
			"ShouldHandleGenericError",
			fmt.Errorf("some unknown error"),
			"",
			false,
			fasthttp.StatusBadRequest,
		},
		{
			"ShouldHandleTLSHandshakeOnPlainText",
			fmt.Errorf(`error when reading request headers: contents: \x16\x03\x03`),
			"",
			false,
			fasthttp.StatusBadRequest,
		},
		{
			"ShouldHandleGenericErrorWithXFF",
			fmt.Errorf("some error"),
			"10.0.0.1, 192.168.1.1",
			false,
			fasthttp.StatusBadRequest,
		},
		{
			"ShouldHandleGenericErrorOnTLS",
			fmt.Errorf("some error"),
			"",
			true,
			fasthttp.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				ctx fasthttp.RequestCtx
				req fasthttp.Request
			)

			if tc.xff != "" {
				req.Header.Set(fasthttp.HeaderXForwardedFor, tc.xff)
			}

			ctx.Init(&req, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, nil)

			if tc.isTLS {
				ctx.Request.SetRequestURI("https://example.com/")
			}

			handler(&ctx, tc.err)

			assert.Equal(t, tc.expectedStatusCode, ctx.Response.StatusCode())
		})
	}
}

func TestHandleNotFound(t *testing.T) {
	next := func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBodyString("next called")
	}

	handler := handleNotFound(next)

	testCases := []struct {
		name               string
		path               string
		expectedStatusCode int
		expectNext         bool
	}{
		{"ShouldReturn404ForAPIPath", "/api", fasthttp.StatusNotFound, false},
		{"ShouldReturn404ForAPISubpath", "/api/something", fasthttp.StatusNotFound, false},
		{"ShouldReturn404ForWellKnown", "/.well-known", fasthttp.StatusNotFound, false},
		{"ShouldReturn404ForWellKnownSubpath", "/.well-known/openid-configuration", fasthttp.StatusNotFound, false},
		{"ShouldReturn404ForStatic", "/static", fasthttp.StatusNotFound, false},
		{"ShouldReturn404ForStaticSubpath", "/static/js/app.js", fasthttp.StatusNotFound, false},
		{"ShouldReturn404ForLocales", "/locales", fasthttp.StatusNotFound, false},
		{"ShouldReturn404ForLocalesSubpath", "/locales/en/translation.json", fasthttp.StatusNotFound, false},
		{"ShouldCallNextForRootPath", "/", fasthttp.StatusOK, true},
		{"ShouldCallNextForUnknownPath", "/other", fasthttp.StatusOK, true},
		{"ShouldReturn404ForUppercaseAPI", "/API", fasthttp.StatusNotFound, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				ctx fasthttp.RequestCtx
				req fasthttp.Request
			)

			req.SetRequestURI(tc.path)
			ctx.Init(&req, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, nil)

			handler(&ctx)

			assert.Equal(t, tc.expectedStatusCode, ctx.Response.StatusCode())

			if tc.expectNext {
				assert.Equal(t, "next called", string(ctx.Response.Body()))
			}
		})
	}
}

func TestHandleMethodNotAllowed(t *testing.T) {
	testCases := []struct {
		name string
	}{
		{"ShouldReturn405"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				ctx fasthttp.RequestCtx
				req fasthttp.Request
			)

			ctx.Init(&req, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, nil)

			handleMethodNotAllowed(&ctx)

			assert.Equal(t, fasthttp.StatusMethodNotAllowed, ctx.Response.StatusCode())
			assert.Contains(t, string(ctx.Response.Body()), "405")
			assert.Contains(t, string(ctx.Response.Body()), "Method Not Allowed")
		})
	}
}

func TestHandlerMainWithAuthzEndpoints(t *testing.T) {
	provider, err := templates.New(templates.Config{})
	require.NoError(t, err)

	require.NoError(t, provider.LoadTemplatedAssets(assets))

	testCases := []struct {
		name  string
		authz map[string]schema.ServerEndpointsAuthz
	}{
		{
			"ShouldSucceedWithDefaultEndpoints",
			schema.DefaultServerConfiguration.Endpoints.Authz,
		},
		{
			"ShouldSucceedWithLegacyEndpoint",
			map[string]schema.ServerEndpointsAuthz{
				"legacy": {Implementation: schema.AuthzImplementationLegacy},
			},
		},
		{
			"ShouldSucceedWithExtAuthzEndpoint",
			map[string]schema.ServerEndpointsAuthz{
				"ext-authz": {Implementation: schema.AuthzImplementationExtAuthz},
			},
		},
		{
			"ShouldSucceedWithForwardAuthEndpoint",
			map[string]schema.ServerEndpointsAuthz{
				"forward-auth": {Implementation: schema.AuthzImplementationForwardAuth},
			},
		},
		{
			"ShouldSucceedWithAuthRequestEndpoint",
			map[string]schema.ServerEndpointsAuthz{
				"auth-request": {Implementation: schema.AuthzImplementationAuthRequest},
			},
		},
		{
			"ShouldSucceedWithNoAuthzEndpoints",
			nil,
		},
		{
			"ShouldSucceedWithMultipleEndpoints",
			map[string]schema.ServerEndpointsAuthz{
				"legacy":       {Implementation: schema.AuthzImplementationLegacy},
				"forward-auth": {Implementation: schema.AuthzImplementationForwardAuth},
				"ext-authz":    {Implementation: schema.AuthzImplementationExtAuthz},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &schema.Configuration{
				Server: schema.Server{
					Address: schema.DefaultServerConfiguration.Address,
					Endpoints: schema.ServerEndpoints{
						Authz: tc.authz,
					},
				},
			}

			providers := middlewares.NewProvidersBasic()
			providers.Random = random.NewMathematical()
			providers.Templates = provider

			handler, err := handlerMain(t.Context(), config, providers)

			require.NoError(t, err)
			assert.NotNil(t, handler)
		})
	}
}

func TestHandlerMainWithOptionalFeatures(t *testing.T) {
	provider, err := templates.New(templates.Config{})
	require.NoError(t, err)

	require.NoError(t, provider.LoadTemplatedAssets(assets))

	testCases := []struct {
		name    string
		config  func() *schema.Configuration
		setOIDC bool
	}{
		{
			"ShouldSucceedWithPasskeyLogin",
			func() *schema.Configuration {
				return &schema.Configuration{
					Server: schema.Server{
						Address:   schema.DefaultServerConfiguration.Address,
						Endpoints: schema.DefaultServerConfiguration.Endpoints,
					},
					WebAuthn: schema.WebAuthn{
						EnablePasskeyLogin: true,
					},
				}
			},
			false,
		},
		{
			"ShouldSucceedWithPprof",
			func() *schema.Configuration {
				return &schema.Configuration{
					Server: schema.Server{
						Address: schema.DefaultServerConfiguration.Address,
						Endpoints: schema.ServerEndpoints{
							EnablePprof: true,
							Authz:       schema.DefaultServerConfiguration.Endpoints.Authz,
						},
					},
				}
			},
			false,
		},
		{
			"ShouldSucceedWithExpvars",
			func() *schema.Configuration {
				return &schema.Configuration{
					Server: schema.Server{
						Address: schema.DefaultServerConfiguration.Address,
						Endpoints: schema.ServerEndpoints{
							EnableExpvars: true,
							Authz:         schema.DefaultServerConfiguration.Endpoints.Authz,
						},
					},
				}
			},
			false,
		},
		{
			"ShouldSucceedWithOpenIDConnect",
			func() *schema.Configuration {
				return &schema.Configuration{
					Server: schema.Server{
						Address:   schema.DefaultServerConfiguration.Address,
						Endpoints: schema.DefaultServerConfiguration.Endpoints,
					},
					IdentityProviders: schema.IdentityProviders{
						OIDC: &schema.IdentityProvidersOpenIDConnect{},
					},
				}
			},
			true,
		},
		{
			"ShouldSucceedWithAllFeatures",
			func() *schema.Configuration {
				return &schema.Configuration{
					Server: schema.Server{
						Address: schema.DefaultServerConfiguration.Address,
						Endpoints: schema.ServerEndpoints{
							EnablePprof:   true,
							EnableExpvars: true,
							Authz:         schema.DefaultServerConfiguration.Endpoints.Authz,
						},
					},
					WebAuthn: schema.WebAuthn{
						EnablePasskeyLogin: true,
					},
					IdentityProviders: schema.IdentityProviders{
						OIDC: &schema.IdentityProvidersOpenIDConnect{},
					},
				}
			},
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := tc.config()

			providers := middlewares.NewProvidersBasic()
			providers.Random = random.NewMathematical()
			providers.Templates = provider

			if tc.setOIDC {
				providers.OpenIDConnect = oidc.NewOpenIDConnectProvider(config, nil, provider)
			}

			handler, err := handlerMain(t.Context(), config, providers)

			require.NoError(t, err)
			assert.NotNil(t, handler)
		})
	}
}

func TestHandlerMainAdminRouteWithBasePathAndMethodRestriction(t *testing.T) {
	provider, err := templates.New(templates.Config{})
	require.NoError(t, err)
	require.NoError(t, provider.LoadTemplatedAssets(assets))
	address := *schema.DefaultServerConfiguration.Address
	address.SetPath("/auth")
	config := &schema.Configuration{Server: schema.Server{Address: &address, Endpoints: schema.DefaultServerConfiguration.Endpoints}}
	providers := middlewares.NewProvidersBasic()
	providers.Random = random.NewMathematical()
	providers.Templates = provider
	providers.SessionProvider = session.NewProvider(config.Session, nil)
	handler, err := handlerMain(t.Context(), config, providers)
	require.NoError(t, err)

	for _, tc := range []struct {
		method   string
		uri      string
		expected int
	}{
		{method: fasthttp.MethodGet, uri: "/auth/api/admin", expected: fasthttp.StatusUnauthorized},
		{method: fasthttp.MethodPost, uri: "/auth/api/admin", expected: fasthttp.StatusMethodNotAllowed},
		{method: fasthttp.MethodGet, uri: "/auth/api/admin/users", expected: fasthttp.StatusUnauthorized},
		{method: fasthttp.MethodGet, uri: "/auth/api/admin/applications", expected: fasthttp.StatusUnauthorized},
		{method: fasthttp.MethodPost, uri: "/auth/api/admin/applications", expected: fasthttp.StatusMethodNotAllowed},
		{method: fasthttp.MethodPut, uri: "/auth/api/admin/application/user", expected: fasthttp.StatusUnauthorized},
		{method: fasthttp.MethodDelete, uri: "/auth/api/admin/application/user", expected: fasthttp.StatusUnauthorized},
		{method: fasthttp.MethodGet, uri: "/auth/api/admin/application/user", expected: fasthttp.StatusMethodNotAllowed},
		{method: fasthttp.MethodPost, uri: "/auth/api/admin/users", expected: fasthttp.StatusUnauthorized},
		{method: fasthttp.MethodGet, uri: "/auth/api/admin/users/email", expected: fasthttp.StatusMethodNotAllowed},
		{method: fasthttp.MethodPost, uri: "/auth/api/admin/users/setup-link", expected: fasthttp.StatusUnauthorized},
		{method: fasthttp.MethodGet, uri: "/auth/api/admin/users/setup-link", expected: fasthttp.StatusMethodNotAllowed},
		{method: fasthttp.MethodGet, uri: "/auth/api/admin/registrations", expected: fasthttp.StatusUnauthorized},
		{method: fasthttp.MethodGet, uri: "/auth/api/admin/registration?id=1", expected: fasthttp.StatusUnauthorized},
		{method: fasthttp.MethodPost, uri: "/auth/api/admin/registration/approve", expected: fasthttp.StatusUnauthorized},
		{method: fasthttp.MethodPost, uri: "/auth/api/admin/registration/reject", expected: fasthttp.StatusUnauthorized},
		{method: fasthttp.MethodDelete, uri: "/auth/api/admin/registration/reject", expected: fasthttp.StatusMethodNotAllowed},
	} {
		var ctx fasthttp.RequestCtx
		ctx.Request.Header.SetMethod(tc.method)
		ctx.Request.Header.SetHost("login.example.com")
		ctx.Request.SetRequestURI(tc.uri)
		handler(&ctx)
		assert.Equal(t, tc.expected, ctx.Response.StatusCode())
	}
}

type timeoutError struct{}

func (e *timeoutError) Error() string { return "i/o timeout" }

func (e *timeoutError) Timeout() bool { return true }

func (e *timeoutError) Temporary() bool { return true }
