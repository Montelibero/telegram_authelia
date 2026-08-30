package authentication

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/authelia/authelia/v4/internal/model"
)

// SQLUserImportReport is a deterministic summary of a YAML user import.
type SQLUserImportReport struct {
	Created   []string
	Unchanged []string
	Conflicts []string
}

// ImportFileUsers plans or executes an idempotent import from users_database.yml.
func ImportFileUsers(ctx context.Context, path, generatedEmailDomain string, store SQLUserImportStore, dryRun bool) (report SQLUserImportReport, err error) {
	if strings.TrimSpace(generatedEmailDomain) == "" {
		return report, errors.New("generated email domain is required")
	}

	var database FileDatabaseModel
	if err = database.Read(path); err != nil {
		return report, err
	}

	if err = store.MigrateMTL(ctx); err != nil {
		return report, fmt.Errorf("failed to migrate SQL user store: %w", err)
	}

	usernames := make([]string, 0, len(database.Users))
	for username := range database.Users {
		usernames = append(usernames, username)
	}
	sort.Strings(usernames)

	imports := make([]model.MTLUserImport, 0, len(usernames))
	inputUsernames := map[string]string{}
	inputEmails := map[string]string{}
	for _, username := range usernames {
		source := database.Users[username]
		candidate := fileUserImport(username, source, generatedEmailDomain)
		normalizedUsername := strings.ToLower(username)
		normalizedEmail := strings.ToLower(candidate.Emails[0].Email)
		if owner, duplicate := inputUsernames[normalizedUsername]; duplicate {
			report.Conflicts = appendUnique(report.Conflicts, owner, username)
		} else {
			inputUsernames[normalizedUsername] = username
		}
		if owner, duplicate := inputEmails[normalizedEmail]; duplicate {
			report.Conflicts = appendUnique(report.Conflicts, owner, username)
		} else {
			inputEmails[normalizedEmail] = username
		}

		existing, found, loadErr := store.LoadMTLUser(ctx, username)
		if loadErr != nil {
			return report, fmt.Errorf("failed to inspect existing SQL user %q: %w", username, loadErr)
		}

		if !found {
			owner, emailFound, emailErr := store.FindMTLUserByEmail(ctx, normalizedEmail)
			if emailErr != nil {
				return report, fmt.Errorf("failed to inspect existing SQL email owner for %q: %w", username, emailErr)
			}
			if emailFound && !strings.EqualFold(owner, username) {
				report.Conflicts = appendUnique(report.Conflicts, username)
				continue
			}

			report.Created = append(report.Created, username)
			imports = append(imports, candidate)
			continue
		}

		if sameImportedUser(existing, candidate) {
			report.Unchanged = append(report.Unchanged, username)
		} else {
			report.Conflicts = append(report.Conflicts, username)
		}
	}

	if len(report.Conflicts) != 0 {
		sort.Strings(report.Conflicts)
		return report, fmt.Errorf("user import has %d conflict(s)", len(report.Conflicts))
	}

	if dryRun || len(imports) == 0 {
		return report, nil
	}

	if err = store.ImportMTLUsers(ctx, imports); err != nil {
		return report, fmt.Errorf("failed to import SQL users: %w", err)
	}

	return report, nil
}

func appendUnique(values []string, additions ...string) []string {
	for _, addition := range additions {
		found := false
		for _, value := range values {
			if value == addition {
				found = true
				break
			}
		}
		if !found {
			values = append(values, addition)
		}
	}

	return values
}

func fileUserImport(username string, source FileDatabaseUserDetailsModel, generatedEmailDomain string) model.MTLUserImport {
	email := source.Email
	if email == "" {
		email = username + "@" + generatedEmailDomain
	}

	status := model.MTLUserStatusActive
	if source.Disabled {
		status = model.MTLUserStatusDisabled
	}

	password := source.Password
	groups := append([]string(nil), source.Groups...)
	sort.Strings(groups)

	return model.MTLUserImport{
		Username: username, DisplayName: source.DisplayName, Status: status, PasswordHash: &password,
		Emails: []model.MTLUserImportEmail{{Email: strings.ToLower(email), Primary: true, Verified: source.Email != ""}},
		Groups: groups,
	}
}

func sameImportedUser(existing model.MTLUserDetails, candidate model.MTLUserImport) bool {
	if existing.User.Username != candidate.Username || existing.User.DisplayName != candidate.DisplayName || existing.User.Status != candidate.Status {
		return false
	}

	if !existing.User.PasswordHash.Valid || candidate.PasswordHash == nil || existing.User.PasswordHash.String != *candidate.PasswordHash {
		return false
	}

	if !strings.EqualFold(existing.PrimaryEmail, candidate.Emails[0].Email) {
		return false
	}

	return slicesEqual(existing.Groups, candidate.Groups)
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}

	return true
}
