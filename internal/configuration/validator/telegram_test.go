package validator

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
)

func TestValidateTelegram(t *testing.T) {
	testCases := []struct {
		name   string
		config schema.Telegram
		errs   []string
	}{
		{
			name: "ShouldAllowDisabledEmptyConfiguration",
		},
		{
			name: "ShouldAllowOfficialHTTPSConfiguration",
			config: schema.Telegram{
				Enabled:      true,
				Issuer:       mustParseTelegramURL(t, "https://oauth.telegram.org"),
				ClientID:     "123456789",
				ClientSecret: "secret",
				CallbackURL:  mustParseTelegramURL(t, "https://auth.example.com/api/telegram/callback"),
			},
		},
		{
			name: "ShouldAllowHTTPForLoopbackTestIssuer",
			config: schema.Telegram{
				Enabled:      true,
				Issuer:       mustParseTelegramURL(t, "http://127.0.0.1:8080"),
				ClientID:     "123456789",
				ClientSecret: "secret",
				CallbackURL:  mustParseTelegramURL(t, "http://localhost:9091/api/telegram/callback"),
			},
		},
		{
			name:   "ShouldRequireEnabledOptions",
			config: schema.Telegram{Enabled: true},
			errs: []string{
				"telegram: option 'issuer' is required when Telegram login is enabled",
				"telegram: option 'client_id' is required when Telegram login is enabled",
				"telegram: option 'client_secret' is required when Telegram login is enabled",
				"telegram: option 'callback_url' is required when Telegram login is enabled",
			},
		},
		{
			name: "ShouldRejectInsecurePublicURLs",
			config: schema.Telegram{
				Enabled:      true,
				Issuer:       mustParseTelegramURL(t, "http://oauth.telegram.org"),
				ClientID:     "123456789",
				ClientSecret: "secret",
				CallbackURL:  mustParseTelegramURL(t, "http://auth.example.com/api/telegram/callback"),
			},
			errs: []string{
				"telegram: option 'issuer' must use HTTPS unless it targets a loopback host",
				"telegram: option 'callback_url' must use HTTPS unless it targets a loopback host",
			},
		},
		{
			name: "ShouldRejectRelativeURLs",
			config: schema.Telegram{
				Enabled:      true,
				Issuer:       mustParseTelegramURL(t, "/issuer"),
				ClientID:     "123456789",
				ClientSecret: "secret",
				CallbackURL:  mustParseTelegramURL(t, "/api/telegram/callback"),
			},
			errs: []string{
				"telegram: option 'issuer' must be an absolute URL",
				"telegram: option 'callback_url' must be an absolute URL",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			validator := schema.NewStructValidator()

			ValidateTelegram(&tc.config, validator)

			require.Len(t, validator.Errors(), len(tc.errs))
			for i, expected := range tc.errs {
				assert.EqualError(t, validator.Errors()[i], expected, fmt.Sprintf("error %d", i))
			}
		})
	}
}

func mustParseTelegramURL(t *testing.T, value string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(value)
	require.NoError(t, err)

	return parsed
}
