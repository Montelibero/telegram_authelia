package model

import (
	"database/sql"
	"time"
)

// MTLRegistrationStatus is the lifecycle state of a pending external registration.
type MTLRegistrationStatus string

const (
	MTLRegistrationStatusPending  MTLRegistrationStatus = "pending"
	MTLRegistrationStatusApproved MTLRegistrationStatus = "approved"
	MTLRegistrationStatusRejected MTLRegistrationStatus = "rejected"
)

// Valid reports whether the status is supported by the registration workflow.
func (s MTLRegistrationStatus) Valid() bool {
	switch s {
	case MTLRegistrationStatusPending, MTLRegistrationStatusApproved, MTLRegistrationStatusRejected:
		return true
	default:
		return false
	}
}

// MTLRegistrationRequest is an external identity awaiting an administrative decision.
type MTLRegistrationRequest struct {
	ID               int64                 `db:"id"`
	Provider         string                `db:"provider"`
	ProviderUserID   string                `db:"provider_user_id"`
	ProviderUsername sql.NullString        `db:"provider_username"`
	DisplayName      sql.NullString        `db:"display_name"`
	ProposedUsername sql.NullString        `db:"proposed_username"`
	ProposedEmail    sql.NullString        `db:"proposed_email"`
	Status           MTLRegistrationStatus `db:"status"`
	Version          int                   `db:"version"`
	RequestedAt      time.Time             `db:"requested_at"`
	UpdatedAt        time.Time             `db:"updated_at"`
	ResolvedAt       sql.NullTime          `db:"resolved_at"`
	ResolvedByUserID sql.NullInt64         `db:"resolved_by_user_id"`
	ApprovedUserID   sql.NullInt64         `db:"approved_user_id"`
}

// MTLRegistrationCandidate contains the current provider data used to upsert a request.
type MTLRegistrationCandidate struct {
	Provider         string
	ProviderUserID   string
	ProviderUsername string
	DisplayName      string
	ProposedUsername string
	ProposedEmail    string
}

// MTLRegistrationApproval contains the exact values required to approve a request.
type MTLRegistrationApproval struct {
	RequestID       int64
	ExpectedVersion int
	Username        string
	DisplayName     string
	Email           string
	Groups          []string
	ActorUsername   string
}
