package telegram

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
)

func TestClientAuthorizationURL(t *testing.T) {
	issuer := newMockIssuer(t, tokenClaims{})
	client := newTestClient(t, issuer)
	flow := Flow{State: "state", Nonce: "nonce", CodeVerifier: "verifier"}

	authorizationURL := client.AuthorizationURL(flow)
	parsed, err := url.Parse(authorizationURL)
	require.NoError(t, err)

	assert.Equal(t, issuer.server.URL+"/auth", parsed.Scheme+"://"+parsed.Host+parsed.Path)
	assert.Equal(t, "code", parsed.Query().Get("response_type"))
	assert.Equal(t, "openid profile email", parsed.Query().Get("scope"))
	assert.Equal(t, flow.State, parsed.Query().Get("state"))
	assert.Equal(t, flow.Nonce, parsed.Query().Get("nonce"))
	assert.Equal(t, "S256", parsed.Query().Get("code_challenge_method"))
	assert.Equal(t, PKCEChallenge(flow.CodeVerifier), parsed.Query().Get("code_challenge"))
}

func TestClientExchangeVerifiesTokenAndExtractsStableIdentity(t *testing.T) {
	issuer := newMockIssuer(t, tokenClaims{
		subject:       "987654321",
		telegramID:    84131737,
		username:      "bublik",
		name:          "Bublik",
		email:         "bublik@example.com",
		emailVerified: true,
		nonce:         "expected-nonce",
	})
	client := newTestClient(t, issuer)
	flow := Flow{Nonce: "expected-nonce", CodeVerifier: "expected-verifier"}

	identity, err := client.Exchange(context.Background(), "authorization-code", flow)
	require.NoError(t, err)

	assert.Equal(t, "84131737", identity.ProviderUserID)
	assert.Equal(t, "bublik", identity.Username)
	assert.Equal(t, "Bublik", identity.Name)
	assert.Equal(t, "bublik@example.com", identity.Email)
	assert.True(t, identity.EmailVerified)
	assert.True(t, issuer.sawTokenRequest)
	assert.Equal(t, flow.CodeVerifier, issuer.codeVerifier)
}

func TestClientExchangePreservesStringTelegramIDBeyondInt64(t *testing.T) {
	issuer := newMockIssuer(t, tokenClaims{
		subject:    "9568533088775932314",
		telegramID: "9568533088775932314",
		nonce:      "expected-nonce",
	})
	client := newTestClient(t, issuer)

	identity, err := client.Exchange(context.Background(), "authorization-code", Flow{Nonce: "expected-nonce", CodeVerifier: "expected-verifier"})
	require.NoError(t, err)

	assert.Equal(t, "9568533088775932314", identity.ProviderUserID)
}

func TestClientExchangeRejectsInvalidTokens(t *testing.T) {
	testCases := []struct {
		name   string
		claims tokenClaims
		flow   Flow
	}{
		{name: "WrongNonce", claims: tokenClaims{subject: "1", nonce: "wrong"}, flow: Flow{Nonce: "expected", CodeVerifier: "verifier"}},
		{name: "MissingSubject", claims: tokenClaims{nonce: "nonce"}, flow: Flow{Nonce: "nonce", CodeVerifier: "verifier"}},
		{name: "MissingTelegramID", claims: tokenClaims{subject: "1", nonce: "nonce"}, flow: Flow{Nonce: "nonce", CodeVerifier: "verifier"}},
		{name: "WrongAudience", claims: tokenClaims{subject: "1", nonce: "nonce", audience: "other-client"}, flow: Flow{Nonce: "nonce", CodeVerifier: "verifier"}},
		{name: "Expired", claims: tokenClaims{subject: "1", nonce: "nonce", expiresAt: time.Now().Add(-time.Hour)}, flow: Flow{Nonce: "nonce", CodeVerifier: "verifier"}},
		{name: "WrongIssuer", claims: tokenClaims{subject: "1", nonce: "nonce", issuer: "https://issuer.invalid"}, flow: Flow{Nonce: "nonce", CodeVerifier: "verifier"}},
		{name: "InvalidSignature", claims: tokenClaims{subject: "1", nonce: "nonce", invalidSignature: true}, flow: Flow{Nonce: "nonce", CodeVerifier: "verifier"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			issuer := newMockIssuer(t, tc.claims)
			client := newTestClient(t, issuer)

			_, err := client.Exchange(context.Background(), "authorization-code", tc.flow)
			assert.Error(t, err)
		})
	}
}

func newTestClient(t *testing.T, issuer *mockIssuer) *Client {
	t.Helper()

	configuration := schema.Telegram{
		Enabled:      true,
		Issuer:       mustParseTelegramURL(t, issuer.server.URL),
		ClientID:     "123456789",
		ClientSecret: "client-secret",
		CallbackURL:  mustParseTelegramURL(t, "https://auth.example.com/api/telegram/callback"),
	}

	client, err := NewClient(context.Background(), configuration, issuer.server.Client())
	require.NoError(t, err)

	return client
}

type tokenClaims struct {
	subject          string
	telegramID       any
	username         string
	name             string
	email            string
	emailVerified    bool
	nonce            string
	issuer           string
	audience         string
	expiresAt        time.Time
	invalidSignature bool
}

type mockIssuer struct {
	server          *httptest.Server
	privateKey      *rsa.PrivateKey
	invalidKey      *rsa.PrivateKey
	claims          tokenClaims
	sawTokenRequest bool
	codeVerifier    string
}

func newMockIssuer(t *testing.T, claims tokenClaims) *mockIssuer {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	invalidKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	issuer := &mockIssuer{privateKey: privateKey, invalidKey: invalidKey, claims: claims}
	issuer.server = httptest.NewServer(http.HandlerFunc(issuer.serveHTTP))
	t.Cleanup(issuer.server.Close)

	return issuer
}

func (m *mockIssuer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		writeJSON(w, map[string]any{
			"issuer":                                m.server.URL,
			"authorization_endpoint":                m.server.URL + "/auth",
			"token_endpoint":                        m.server.URL + "/token",
			"jwks_uri":                              m.server.URL + "/jwks",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	case "/jwks":
		writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &m.privateKey.PublicKey, KeyID: "telegram-test", Algorithm: "RS256", Use: "sig"}}})
	case "/token":
		m.sawTokenRequest = true
		_ = r.ParseForm()
		m.codeVerifier = r.Form.Get("code_verifier")
		clientID, clientSecret, ok := r.BasicAuth()
		if !ok || clientID != "123456789" || clientSecret != "client-secret" || r.Form.Get("code") == "" {
			http.Error(w, "invalid token request", http.StatusUnauthorized)
			return
		}

		token, err := m.signToken()
		if err != nil {
			http.Error(w, "token signing failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"access_token": "access-token", "token_type": "Bearer", "expires_in": 3600, "id_token": token})
	default:
		http.NotFound(w, r)
	}
}

func (m *mockIssuer) signToken() (string, error) {
	now := time.Now()
	issuer := m.claims.issuer
	if issuer == "" {
		issuer = m.server.URL
	}
	audience := m.claims.audience
	if audience == "" {
		audience = "123456789"
	}
	expiresAt := m.claims.expiresAt
	if expiresAt.IsZero() {
		expiresAt = now.Add(time.Hour)
	}

	claims := jwt.MapClaims{
		"iss":                issuer,
		"aud":                audience,
		"sub":                m.claims.subject,
		"id":                 m.claims.telegramID,
		"iat":                now.Unix(),
		"exp":                expiresAt.Unix(),
		"nonce":              m.claims.nonce,
		"preferred_username": m.claims.username,
		"name":               m.claims.name,
		"email":              m.claims.email,
		"email_verified":     m.claims.emailVerified,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "telegram-test"
	key := m.privateKey
	if m.claims.invalidSignature {
		key = m.invalidKey
	}

	return token.SignedString(key)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func mustParseTelegramURL(t *testing.T, value string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(strings.TrimSpace(value))
	require.NoError(t, err)

	return parsed
}
