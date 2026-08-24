package middlewares

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/authentication"
	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/storage"
)

func TestSetContentType(t *testing.T) {
	testCases := []struct {
		name     string
		fn       func(*fasthttp.RequestCtx)
		expected string
	}{
		{
			name:     "ShouldSetContentTypeApplicationJSON",
			fn:       SetContentTypeApplicationJSON,
			expected: "application/json; charset=utf-8",
		},
		{
			name:     "ShouldSetContentTypeTextPlain",
			fn:       SetContentTypeTextPlain,
			expected: "text/plain; charset=utf-8",
		},
	}

	for i := range testCases {
		t.Run(testCases[i].name, func(t *testing.T) {
			tc := testCases[i]

			var ctx fasthttp.RequestCtx

			tc.fn(&ctx)

			require.Equal(t, tc.expected, string(ctx.Response.Header.ContentType()))
		})
	}
}

func TestNewAuthenticationProviderSQL(t *testing.T) {
	config := schema.Configuration{
		AuthenticationBackend: schema.AuthenticationBackend{
			SQL: &schema.AuthenticationBackendSQL{GeneratedEmailDomain: "eurmtl.me", Password: schema.DefaultPasswordConfig},
		},
		Storage: schema.Storage{Local: &schema.StorageLocal{Path: filepath.Join(t.TempDir(), "db.sqlite3")}},
	}
	store := storage.NewSQLiteProvider(&config)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	provider := NewAuthenticationProvider(&config, nil, store)
	require.IsType(t, &authentication.SQLUserProvider{}, provider)
	require.NoError(t, provider.StartupCheck())

	tables, err := store.SchemaTables(context.Background())
	require.NoError(t, err)
	assert.Contains(t, tables, "mtl_schema_migrations")
}

func TestNewAuthenticationProviderSQLRequiresCompatibleStorage(t *testing.T) {
	config := schema.Configuration{AuthenticationBackend: schema.AuthenticationBackend{SQL: &schema.AuthenticationBackendSQL{}}}
	assert.Nil(t, NewAuthenticationProvider(&config, nil))
}

func TestNewAuthenticationProvider(t *testing.T) {
	testCases := []struct {
		name   string
		config schema.Configuration
	}{
		{
			name:   "ShouldReturnNilProviderWhenNoBackendConfigured",
			config: schema.Configuration{},
		},
	}

	for i := range testCases {
		t.Run(testCases[i].name, func(t *testing.T) {
			tc := testCases[i]
			provider := NewAuthenticationProvider(&tc.config, nil)
			require.Nil(t, provider)
		})
	}
}

func TestNewAuthenticationProviderFile(t *testing.T) {
	dir := t.TempDir()

	dbPath := filepath.Join(dir, "users.yml")

	require.NoError(t, os.WriteFile(dbPath, []byte("users: {}\n"), 0600))

	config := schema.Configuration{
		AuthenticationBackend: schema.AuthenticationBackend{
			File: &schema.AuthenticationBackendFile{
				Path:     dbPath,
				Password: schema.DefaultCIPasswordConfig,
			},
		},
	}

	provider := NewAuthenticationProvider(&config, nil)

	assert.NotNil(t, provider)
}

func TestNewAuthenticationProviderLDAP(t *testing.T) {
	config := schema.Configuration{
		AuthenticationBackend: schema.AuthenticationBackend{
			LDAP: &schema.AuthenticationBackendLDAP{
				Address: &schema.AddressLDAP{},
			},
		},
	}

	provider := NewAuthenticationProvider(&config, nil)

	assert.NotNil(t, provider)
}

func TestNewProvidersBasic(t *testing.T) {
	providers := NewProvidersBasic()

	assert.NotNil(t, providers.Clock)
	assert.NotNil(t, providers.Random)
	assert.Nil(t, providers.StorageProvider)
	assert.Nil(t, providers.Authorizer)
	assert.Nil(t, providers.UserProvider)
	assert.Nil(t, providers.SessionProvider)
	assert.Nil(t, providers.Notifier)
	assert.Nil(t, providers.OpenIDConnect)
	assert.Nil(t, providers.Metrics)
}

func TestNewProviders(t *testing.T) {
	testCases := []struct {
		name           string
		config         func(t *testing.T) *schema.Configuration
		expectErrs     int
		expectWarns    int
		expectNotifier bool
		expectMetrics  bool
	}{
		{
			"ShouldCreateWithMinimalConfig",
			func(t *testing.T) *schema.Configuration {
				return &schema.Configuration{}
			},
			0,
			0,
			false,
			false,
		},
		{
			"ShouldCreateWithFileNotifier",
			func(t *testing.T) *schema.Configuration {
				return &schema.Configuration{
					Notifier: schema.Notifier{
						FileSystem: &schema.NotifierFileSystem{
							Filename: filepath.Join(t.TempDir(), "notification.txt"),
						},
					},
				}
			},
			0,
			0,
			true,
			false,
		},
		{
			"ShouldCreateWithSMTPNotifier",
			func(t *testing.T) *schema.Configuration {
				return &schema.Configuration{
					Notifier: schema.Notifier{
						SMTP: &schema.NotifierSMTP{
							Address: &schema.AddressSMTP{},
						},
					},
				}
			},
			0,
			0,
			true,
			false,
		},
		{
			"ShouldCreateWithMetrics",
			func(t *testing.T) *schema.Configuration {
				return &schema.Configuration{
					Telemetry: schema.Telemetry{
						Metrics: schema.TelemetryMetrics{
							Enabled: true,
						},
					},
				}
			},
			0,
			0,
			false,
			true,
		},
		{
			"ShouldCreateWithFileAuthBackend",
			func(t *testing.T) *schema.Configuration {
				dir := t.TempDir()
				dbPath := filepath.Join(dir, "users.yml")

				require.NoError(t, os.WriteFile(dbPath, []byte("users: {}\n"), 0600))

				return &schema.Configuration{
					AuthenticationBackend: schema.AuthenticationBackend{
						File: &schema.AuthenticationBackendFile{
							Path:     dbPath,
							Password: schema.DefaultCIPasswordConfig,
						},
					},
				}
			},
			0,
			0,
			false,
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := tc.config(t)

			providers, warns, errs := NewProviders(config, nil)

			assert.Len(t, errs, tc.expectErrs)
			assert.Len(t, warns, tc.expectWarns)

			assert.NotNil(t, providers.Clock)
			assert.NotNil(t, providers.Random)
			assert.NotNil(t, providers.Authorizer)
			assert.NotNil(t, providers.Regulator)

			if tc.expectNotifier {
				assert.NotNil(t, providers.Notifier)
			} else {
				assert.Nil(t, providers.Notifier)
			}

			if tc.expectMetrics {
				assert.NotNil(t, providers.Metrics)
			} else {
				assert.Nil(t, providers.Metrics)
			}
		})
	}
}
