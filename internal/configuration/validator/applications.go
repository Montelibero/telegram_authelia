package validator

import (
	"fmt"
	"strings"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
)

// ValidateApplications validates and resolves application-to-group mappings.
func ValidateApplications(config *[]schema.Application, validator *schema.StructValidator) {
	slugs := make(map[string]struct{}, len(*config))
	groups := make(map[string]struct{}, len(*config))

	for i := range *config {
		application := &(*config)[i]
		entry := i + 1

		if strings.TrimSpace(application.Slug) == "" {
			validator.Push(fmt.Errorf("applications: option 'slug' is required for entry %d", entry))
		} else if _, exists := slugs[application.Slug]; exists {
			validator.Push(fmt.Errorf("applications: duplicate slug '%s'", application.Slug))
		} else {
			slugs[application.Slug] = struct{}{}
		}

		if strings.TrimSpace(application.Name) == "" {
			validator.Push(fmt.Errorf("applications: option 'name' is required for entry %d", entry))
		}
		if strings.TrimSpace(application.Domain) == "" {
			validator.Push(fmt.Errorf("applications: option 'domain' is required for entry %d", entry))
		}

		if application.Group == "" && application.Slug != "" {
			application.Group = "app:" + application.Slug
		}
		if strings.EqualFold(application.Group, "admins") {
			validator.Push(fmt.Errorf("applications: group 'admins' is reserved for administrative access in entry %d", entry))
		} else if application.Group != "" {
			if _, exists := groups[application.Group]; exists {
				validator.Push(fmt.Errorf("applications: duplicate group '%s'", application.Group))
			} else {
				groups[application.Group] = struct{}{}
			}
		}
	}
}
