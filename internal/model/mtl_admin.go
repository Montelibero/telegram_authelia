package model

// MTLAdminUserSummary is the safe administrative list representation of a user.
type MTLAdminUserSummary struct {
	Username        string   `json:"username"`
	DisplayName     string   `json:"display_name"`
	Status          string   `json:"status"`
	Version         int      `json:"version"`
	PasswordEnabled bool     `json:"password_enabled"`
	PrimaryEmail    string   `json:"primary_email"`
	Groups          []string `json:"groups"`
}

// MTLAdminUserDetails is the safe administrative detail representation of a user.
type MTLAdminUserDetails struct {
	MTLAdminUserSummary
	SessionEpoch int               `json:"session_epoch"`
	Emails       []MTLUserEmail    `json:"emails"`
	Identities   []MTLUserIdentity `json:"identities"`
}

// MTLAdminUserCreate contains the fields used to create an administrative user record.
type MTLAdminUserCreate struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email"`
	Groups      []string `json:"groups"`
}

// MTLAdminUserUpdate contains optimistic user profile changes.
type MTLAdminUserUpdate struct {
	ExpectedVersion int    `json:"expected_version"`
	DisplayName     string `json:"display_name"`
	Status          string `json:"status"`
}

// MTLAdminEmailCreate contains a verified email created by an administrator.
type MTLAdminEmailCreate struct {
	ExpectedVersion int    `json:"expected_version"`
	Email           string `json:"email"`
	Primary         bool   `json:"primary"`
}

// MTLAdminGroupCreate contains a new unrestricted group name.
type MTLAdminGroupCreate struct {
	Name string `json:"name"`
}

// MTLAdminGroupUpdate contains an optimistic group rename.
type MTLAdminGroupUpdate struct {
	ExpectedVersion int    `json:"expected_version"`
	Name            string `json:"name"`
}
