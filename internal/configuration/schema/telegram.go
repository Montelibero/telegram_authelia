package schema

import "net/url"

// Telegram configures Telegram as an upstream OpenID Connect authentication provider.
type Telegram struct {
	Enabled      bool     `koanf:"enabled" yaml:"enabled,omitempty" toml:"enabled,omitempty" json:"enabled,omitempty" jsonschema:"default=false,title=Enabled" jsonschema_description:"Enables Telegram login."`
	Issuer       *url.URL `koanf:"issuer" yaml:"issuer,omitempty" toml:"issuer,omitempty" json:"issuer,omitempty" jsonschema:"format=uri,title=Issuer URL" jsonschema_description:"The Telegram OpenID Connect issuer URL."`
	ClientID     string   `koanf:"client_id" yaml:"client_id,omitempty" toml:"client_id,omitempty" json:"client_id,omitempty" jsonschema:"title=Client ID" jsonschema_description:"The Telegram Login client identifier provided by BotFather."`
	ClientSecret string   `koanf:"client_secret" yaml:"client_secret,omitempty" toml:"client_secret,omitempty" json:"-" jsonschema:"title=Client Secret" jsonschema_description:"The Telegram Login client secret provided by BotFather."`
	CallbackURL  *url.URL `koanf:"callback_url" yaml:"callback_url,omitempty" toml:"callback_url,omitempty" json:"callback_url,omitempty" jsonschema:"format=uri,title=Callback URL" jsonschema_description:"The exact public callback URL registered with Telegram."`
}
