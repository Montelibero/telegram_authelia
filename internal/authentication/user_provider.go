package authentication

import (
	"context"

	"github.com/authelia/authelia/v4/internal/model"
)

// SQLUserStore is the narrow storage contract required by SQLUserProvider.
type SQLUserStore interface {
	MigrateMTL(ctx context.Context) (err error)
	ReconcileMTLGroups(ctx context.Context, groups []string) (err error)
	LoadMTLUser(ctx context.Context, username string) (details model.MTLUserDetails, found bool, err error)
	FindMTLUserByEmail(ctx context.Context, email string) (username string, found bool, err error)
	UpdateMTLUserPassword(ctx context.Context, userID int64, passwordHash *string, expectedVersion int) (err error)
	SetMTLSelfServicePassword(ctx context.Context, username, passwordHash string, expectedVersion int, actor string) (details model.MTLAdminUserDetails, err error)
}

// SQLUserImportStore extends SQLUserStore with atomic user creation.
type SQLUserImportStore interface {
	SQLUserStore
	ImportMTLUsers(ctx context.Context, users []model.MTLUserImport) (err error)
}

// UserProvider is the interface for interacting with the authentication backends.
type UserProvider interface {
	model.StartupCheck

	// CheckUserPassword is used to check if a password matches for a specific user.
	CheckUserPassword(username string, password string) (valid bool, err error)

	// GetDetails is used to get a user's information.
	GetDetails(username string) (details *UserDetails, err error)

	GetDetailsExtended(username string) (details *UserDetailsExtended, err error)

	// UpdatePassword is used to change a user's password without verifying their old password.
	UpdatePassword(username string, newPassword string) (err error)

	// ChangePassword is used to change a user's password but requires their old password to be successfully verified.
	ChangePassword(username string, oldPassword string, newPassword string) (err error)

	Close() (err error)
}

// SelfServicePasswordProvider is implemented by providers that support password setup without an old password.
type SelfServicePasswordProvider interface {
	SetPasswordFromProof(username, newPassword string) (details *UserDetails, err error)
}
