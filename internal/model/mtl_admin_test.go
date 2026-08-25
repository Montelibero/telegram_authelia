package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMTLAdminUserSummaryDoesNotExposePasswordHash(t *testing.T) {
	data, err := json.Marshal(MTLAdminUserSummary{Username: "bublik", PasswordEnabled: true})
	require.NoError(t, err)
	assert.JSONEq(t, `{"username":"bublik","display_name":"","status":"","version":0,"password_enabled":true,"primary_email":"","groups":null}`, string(data))
	assert.NotContains(t, string(data), "password_hash")
}
