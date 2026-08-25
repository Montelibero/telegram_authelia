package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/authelia/authelia/v4/internal/authentication"
	"github.com/authelia/authelia/v4/internal/mocks"
	"github.com/authelia/authelia/v4/internal/session"
)

type authzMTLTestUserProvider struct{ authentication.UserProvider }

func (authzMTLTestUserProvider) IsMTLProvider() bool { return true }

func TestHandleSessionValidateEpochRejectsRevokedMTLSession(t *testing.T) {
	mock := mocks.NewMockAutheliaCtx(t)
	defer mock.Close()
	oldEpoch, currentEpoch := 1, 2
	userSession := session.UserSession{Username: "bublik", SessionEpoch: &oldEpoch}
	userSession.AuthenticationMethodRefs.External = true
	mock.Ctx.Providers.UserProvider = authzMTLTestUserProvider{mock.UserProviderMock}
	mock.UserProviderMock.EXPECT().GetDetails("bublik").Return(&authentication.UserDetails{Username: "bublik", SessionEpoch: &currentEpoch}, nil)

	modified, invalid := handleSessionValidateEpoch(mock.Ctx, &userSession)
	assert.False(t, modified)
	assert.True(t, invalid)
}

func TestHandleSessionValidateEpochSkipsNonMTLSession(t *testing.T) {
	mock := mocks.NewMockAutheliaCtx(t)
	defer mock.Close()
	userSession := session.UserSession{Username: "ldap-user"}
	userSession.AuthenticationMethodRefs.External = true
	modified, invalid := handleSessionValidateEpoch(mock.Ctx, &userSession)
	assert.False(t, modified)
	assert.False(t, invalid)
}

func TestHandleSessionValidateEpochRejectsLegacyMTLSession(t *testing.T) {
	mock := mocks.NewMockAutheliaCtx(t)
	defer mock.Close()
	userSession := session.UserSession{Username: "bublik"}
	userSession.AuthenticationMethodRefs.External = true
	mock.Ctx.Providers.UserProvider = authzMTLTestUserProvider{mock.UserProviderMock}
	modified, invalid := handleSessionValidateEpoch(mock.Ctx, &userSession)
	assert.False(t, modified)
	assert.True(t, invalid)
}
