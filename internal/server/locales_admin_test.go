package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRussianSettingsLocaleContainsAdminTranslations(t *testing.T) {
	keys := []string{
		"Accounts", "Active", "Add email", "Add group", "Add member",
		"Administrator actions unlocked", "Administrator password", "Affected users", "All", "Approve", "Approved",
		"Approving or rejecting a registration requires a recent administrator password check.",
		"Confirmation username", "Copy link", "Create group", "Create user", "Delete", "Delete group", "Disabled", "Display name",
		"Email addresses", "Expires", "External ACL configuration was not changed", "Failed to copy setup link",
		"Failed to create user; reauthenticate and try again", "Failed to generate setup link", "Failed to load group", "Failed to load groups",
		"Failed to load registration", "Failed to load registrations", "Failed to load user", "Failed to load users", "Filter users", "Generate setup link",
		"Group changed elsewhere; the latest version has been loaded", "Group changes require a recent administrator password check.",
		"Group created", "Group creation failed; reauthenticate and try again", "Group name",
		"Group update failed; reauthenticate or reload and try again", "Group updated", "Groups",
		"Incorrect password or reauthentication failed", "Linked identities", "Loading", "Make primary", "New email", "New group", "New group name",
		"One-time setup link", "Password enabled", "Pending", "Pending registrations", "Primary",
		"Registration changed elsewhere; the latest version has been loaded", "Registration resolved",
		"Registration update failed; reauthenticate and try again", "Reject", "Rejected", "Rename group",
		"Required for changes that can remove your own access", "Save new user", "Save user", "Setup link copied", "Status", "Status filter",
		"Unlink", "Unlock changes", "User changed elsewhere; the latest version has been loaded",
		"User changes require a recent password check. Telegram login remains active after reauthentication.",
		"User update failed; reauthenticate or reload and try again", "User updated", "Username to add", "Users", "none",
		"Failed to load permissions", "Filter applications", "No applications are configured", "Permission changes require a recent administrator password check.",
		"Permission update failed; reauthenticate or reload and try again", "Permission updated", "Permissions", "Permissions changed elsewhere; the latest version has been loaded",
		"Managed application group",
	}

	english := readSettingsLocale(t, "locales/en/settings.json")
	russian := readSettingsLocale(t, "locales/ru-RU/settings.json")

	for _, key := range keys {
		require.Contains(t, english, key)
		translation, ok := russian[key]
		if !assert.Truef(t, ok, "Russian settings locale does not contain %q", key) {
			continue
		}
		assert.NotEmpty(t, translation)
		assert.NotEqual(t, key, translation)
	}
}

func readSettingsLocale(t *testing.T, path string) map[string]string {
	t.Helper()

	data, err := locales.ReadFile(path)
	require.NoError(t, err)

	translations := map[string]string{}
	require.NoError(t, json.Unmarshal(data, &translations))

	return translations
}
