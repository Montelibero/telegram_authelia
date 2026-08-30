package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMTLUserStatuses(t *testing.T) {
	assert.Equal(t, "active", MTLUserStatusActive)
	assert.Equal(t, "disabled", MTLUserStatusDisabled)
}
