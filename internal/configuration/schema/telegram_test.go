package schema

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelegramConfigurationDoesNotSerializeClientSecret(t *testing.T) {
	configuration := Telegram{
		Enabled:      true,
		Issuer:       &url.URL{Scheme: "https", Host: "oauth.telegram.org"},
		ClientID:     "123456789",
		ClientSecret: "telegram-client-secret",
		CallbackURL:  &url.URL{Scheme: "https", Host: "auth.example.com", Path: "/api/telegram/callback"},
	}

	data, err := json.Marshal(configuration)
	require.NoError(t, err)

	assert.NotContains(t, string(data), configuration.ClientSecret)
	assert.Contains(t, string(data), configuration.ClientID)
}
