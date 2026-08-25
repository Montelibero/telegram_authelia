package validator

import (
	"errors"
	"net"
	"net/url"
	"strings"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
)

// ValidateTelegram validates Telegram Login configuration.
func ValidateTelegram(config *schema.Telegram, validator *schema.StructValidator) {
	if !config.Enabled {
		return
	}

	validateTelegramURL("issuer", config.Issuer, validator)

	if config.ClientID == "" {
		validator.Push(errors.New("telegram: option 'client_id' is required when Telegram login is enabled"))
	}

	if config.ClientSecret == "" {
		validator.Push(errors.New("telegram: option 'client_secret' is required when Telegram login is enabled"))
	}

	validateTelegramURL("callback_url", config.CallbackURL, validator)
}

func validateTelegramURL(name string, value *url.URL, validator *schema.StructValidator) {
	if value == nil || value.String() == "" {
		validator.Push(errors.New("telegram: option '" + name + "' is required when Telegram login is enabled"))

		return
	}

	if !value.IsAbs() || value.Hostname() == "" {
		validator.Push(errors.New("telegram: option '" + name + "' must be an absolute URL"))

		return
	}

	if value.Scheme != schemeHTTPS && !(value.Scheme == schemeHTTP && isTelegramLoopbackHost(value.Hostname())) {
		validator.Push(errors.New("telegram: option '" + name + "' must use HTTPS unless it targets a loopback host"))
	}
}

func isTelegramLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}
