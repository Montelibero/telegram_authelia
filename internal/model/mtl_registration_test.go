package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMTLRegistrationStatusValid(t *testing.T) {
	assert.True(t, MTLRegistrationStatusPending.Valid())
	assert.True(t, MTLRegistrationStatusApproved.Valid())
	assert.True(t, MTLRegistrationStatusRejected.Valid())
	assert.False(t, MTLRegistrationStatus("unknown").Valid())
}
