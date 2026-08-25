package middlewares

import (
	"context"
	"crypto/x509"
	"errors"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/authentication"
	"github.com/authelia/authelia/v4/internal/authorization"
	"github.com/authelia/authelia/v4/internal/clock"
	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/expression"
	"github.com/authelia/authelia/v4/internal/metrics"
	"github.com/authelia/authelia/v4/internal/notification"
	"github.com/authelia/authelia/v4/internal/ntp"
	"github.com/authelia/authelia/v4/internal/oidc"
	"github.com/authelia/authelia/v4/internal/random"
	"github.com/authelia/authelia/v4/internal/regulation"
	"github.com/authelia/authelia/v4/internal/session"
	"github.com/authelia/authelia/v4/internal/storage"
	"github.com/authelia/authelia/v4/internal/telegram"
	"github.com/authelia/authelia/v4/internal/templates"
	"github.com/authelia/authelia/v4/internal/totp"
	"github.com/authelia/authelia/v4/internal/webauthn"
)

// SetContentTypeApplicationJSON sets the Content-Type header to `application/json; charset=utf-8`.
func SetContentTypeApplicationJSON(ctx *fasthttp.RequestCtx) {
	ctx.SetContentTypeBytes(contentTypeApplicationJSON)
}

// SetContentTypeTextPlain sets the Content-Type header to `text/plain; charset=utf-8`.
func SetContentTypeTextPlain(ctx *fasthttp.RequestCtx) {
	ctx.SetContentTypeBytes(contentTypeTextPlain)
}

// NewProviders provisions all providers based on the configuration provided.
func NewProviders(config *schema.Configuration, caCertPool *x509.CertPool) (providers Providers, warns, errs []error) {
	providers = NewProvidersBasic()

	providers.StorageProvider = storage.NewProvider(config, caCertPool)
	providers.Authorizer = authorization.NewAuthorizer(config)
	providers.NTP = ntp.NewProvider(&config.NTP)
	providers.PasswordPolicy = NewPasswordPolicyProvider(config.PasswordPolicy)
	providers.Regulator = regulation.NewRegulator(config.Regulation, providers.StorageProvider, providers.Clock)
	providers.SessionProvider = session.NewProvider(config.Session, caCertPool)
	providers.TOTP = totp.NewTimeBasedProvider(config.TOTP)
	providers.UserAttributeResolver = expression.NewUserAttributes(config)
	providers.UserProvider = NewAuthenticationProvider(config, caCertPool, providers.StorageProvider)

	if config.Telegram.Enabled {
		store, usersOK := providers.StorageProvider.(telegram.IdentityUserStore)
		links, linksOK := providers.StorageProvider.(telegram.IdentityLinkStore)
		replay, replayOK := providers.StorageProvider.(telegram.StateReplayStore)
		if !usersOK || !linksOK || !replayOK {
			errs = append(errs, errors.New("configured storage provider is not compatible with Telegram identities"))
		} else if client, err := telegram.NewClient(context.Background(), config.Telegram, nil); err != nil {
			errs = append(errs, err)
		} else {
			states := telegram.NewStateStore(5*time.Minute, providers.Clock.Now, nil, []byte(config.Telegram.ClientSecret), replay)
			providers.Telegram = telegram.NewLoginService(client, states, store)
			providers.TelegramLink = telegram.NewLinkService(client, states, links)
		}
	}

	var err error
	if providers.Templates, err = templates.New(templates.Config{EmailTemplatesPath: config.Notifier.TemplatePath}); err != nil {
		errs = append(errs, err)
	}

	if providers.MetaDataService, err = webauthn.NewMetaDataProvider(config, providers.StorageProvider); err != nil {
		errs = append(errs, err)
	}

	switch {
	case config.Notifier.SMTP != nil:
		providers.Notifier = notification.NewSMTPNotifier(config.Notifier.SMTP, caCertPool)
	case config.Notifier.FileSystem != nil:
		providers.Notifier = notification.NewFileNotifier(*config.Notifier.FileSystem)
	}

	providers.OpenIDConnect = oidc.NewOpenIDConnectProvider(config, providers.StorageProvider, providers.Templates)

	if config.Telemetry.Metrics.Enabled {
		if providers.Metrics, err = metrics.NewPrometheus(); err != nil {
			errs = append(errs, err)
		}
	}

	return providers, warns, errs
}

// NewProvidersBasic returns a new Providers with the simple providers.
func NewProvidersBasic() Providers {
	return Providers{
		Clock:  clock.New(),
		Random: random.New(),
	}
}

// NewAuthenticationProvider returns a new authentication.UserProvider.
func NewAuthenticationProvider(config *schema.Configuration, caCertPool *x509.CertPool, stores ...storage.Provider) (provider authentication.UserProvider) {
	switch {
	case config.AuthenticationBackend.File != nil:
		return authentication.NewFileUserProvider(config.AuthenticationBackend.File)
	case config.AuthenticationBackend.LDAP != nil:
		return authentication.NewLDAPUserProvider(config.AuthenticationBackend, caCertPool)
	case config.AuthenticationBackend.SQL != nil && len(stores) == 1:
		if store, ok := stores[0].(authentication.SQLUserStore); ok {
			return authentication.NewSQLUserProvider(config.AuthenticationBackend.SQL, store)
		}

		return nil
	default:
		return nil
	}
}
