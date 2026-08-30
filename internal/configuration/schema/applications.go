package schema

// Application maps an infrastructure application to the standard Authelia group used by ACL rules.
type Application struct {
	Slug    string `koanf:"slug" yaml:"slug" toml:"slug" json:"slug" jsonschema:"required,title=Slug" jsonschema_description:"The stable application identifier."`
	Name    string `koanf:"name" yaml:"name" toml:"name" json:"name" jsonschema:"required,title=Name" jsonschema_description:"The application display name."`
	Domain  string `koanf:"domain" yaml:"domain,omitempty" toml:"domain,omitempty" json:"domain,omitempty" jsonschema:"title=Domain" jsonschema_description:"The optional primary application domain shown in the administrative UI."`
	Group   string `koanf:"group" yaml:"group,omitempty" toml:"group,omitempty" json:"group" jsonschema:"title=Group" jsonschema_description:"The standard Authelia group granting access. Defaults to app:<slug>."`
	Enabled *bool  `koanf:"enabled" yaml:"enabled,omitempty" toml:"enabled,omitempty" json:"enabled,omitempty" jsonschema:"default=true,title=Enabled" jsonschema_description:"Whether the application is shown for permission management."`
}

// IsEnabled reports whether the application is enabled. Omitted values default to enabled.
func (a Application) IsEnabled() bool {
	return a.Enabled == nil || *a.Enabled
}
