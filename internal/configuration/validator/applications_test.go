package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
)

func TestValidateApplications(t *testing.T) {
	t.Run("ShouldDeriveGroupAndDefaultEnabled", func(t *testing.T) {
		applications := []schema.Application{{Slug: "grafana", Name: "Grafana", Domain: "grafana.example.com"}}
		validator := schema.NewStructValidator()

		ValidateApplications(&applications, validator)

		require.Empty(t, validator.Errors())
		assert.Equal(t, "app:grafana", applications[0].Group)
		assert.True(t, applications[0].IsEnabled())
	})

	t.Run("ShouldPreserveExplicitUnrestrictedGroupAndDisabledState", func(t *testing.T) {
		disabled := false
		applications := []schema.Application{{
			Slug: "odd", Name: "Odd", Domain: "odd.example.com", Group: " team, odd:readers ", Enabled: &disabled,
		}}
		validator := schema.NewStructValidator()

		ValidateApplications(&applications, validator)

		require.Empty(t, validator.Errors())
		assert.Equal(t, " team, odd:readers ", applications[0].Group)
		assert.False(t, applications[0].IsEnabled())
	})

	t.Run("ShouldRejectMissingAndAmbiguousMappings", func(t *testing.T) {
		applications := []schema.Application{
			{Slug: "", Name: "Missing Slug", Domain: "one.example.com"},
			{Slug: "duplicate", Name: "", Domain: ""},
			{Slug: "duplicate", Name: "Duplicate", Domain: "two.example.com", Group: "shared"},
			{Slug: "other", Name: "Other", Domain: "three.example.com", Group: "shared"},
		}
		validator := schema.NewStructValidator()

		ValidateApplications(&applications, validator)

		require.Len(t, validator.Errors(), 4)
		assert.EqualError(t, validator.Errors()[0], "applications: option 'slug' is required for entry 1")
		assert.EqualError(t, validator.Errors()[1], "applications: option 'name' is required for entry 2")
		assert.EqualError(t, validator.Errors()[2], "applications: option 'domain' is required for entry 2")
		assert.EqualError(t, validator.Errors()[3], "applications: duplicate slug 'duplicate'")
		assert.Equal(t, "shared", applications[2].Group)
		assert.Equal(t, applications[2].Group, applications[3].Group)
	})
}
