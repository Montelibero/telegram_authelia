package model

import "time"

// MTLAdminUserSummary is the safe administrative list representation of a user.
type MTLAdminUserSummary struct {
	Username           string   `json:"username"`
	DisplayName        string   `json:"display_name"`
	Status             string   `json:"status"`
	Version            int      `json:"version"`
	PasswordEnabled    bool     `json:"password_enabled"`
	PrimaryEmail       string   `json:"primary_email"`
	TelegramID         string   `json:"telegram_id,omitempty"`
	ProvisioningStatus string   `json:"provisioning_status,omitempty"`
	Groups             []string `json:"groups"`
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
	TelegramID  string   `json:"telegram_id"`
}

// MTLAdminIdentityLink contains an identity directly assigned by an administrator.
type MTLAdminIdentityLink struct {
	ExpectedVersion int    `json:"expected_version"`
	Provider        string `json:"provider"`
	ProviderUserID  string `json:"provider_user_id"`
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

// MTLSelfServiceProfile is the safe current-user profile representation.
type MTLSelfServiceProfile struct {
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	Version         int    `json:"version"`
	PasswordEnabled bool   `json:"password_enabled"`
	TelegramLinked  bool   `json:"telegram_linked"`
}

// MTLSelfServiceProfileUpdate contains the current user's editable profile fields.
type MTLSelfServiceProfileUpdate struct {
	ExpectedVersion int    `json:"expected_version"`
	DisplayName     string `json:"display_name"`
}

// MTLAdminGroupSummary is the safe administrative list representation of a group.
type MTLAdminGroupSummary struct {
	Name      string    `db:"name" json:"name"`
	Version   int       `db:"version" json:"version"`
	UserCount int       `db:"user_count" json:"user_count"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
	Managed   bool      `db:"-" json:"managed"`
}

// MTLAdminGroupDetails is the safe administrative detail representation of a group.
type MTLAdminGroupDetails struct {
	MTLAdminGroupSummary
	Users []string `json:"users"`
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
