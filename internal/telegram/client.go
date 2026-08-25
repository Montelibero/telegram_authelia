package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	coreoidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
)

// Identity is the verified stable Telegram identity extracted from an ID token.
type Identity struct {
	ProviderUserID string
	Username       string
	Name           string
	Email          string
	EmailVerified  bool
}

// Client implements Telegram's OpenID Connect Authorization Code flow.
type Client struct {
	oauth      oauth2.Config
	verifier   *coreoidc.IDTokenVerifier
	httpClient *http.Client
}

// NewClient discovers the configured issuer and constructs a verified OIDC client.
func NewClient(ctx context.Context, config schema.Telegram, httpClient *http.Client) (*Client, error) {
	if config.Issuer == nil || config.CallbackURL == nil {
		return nil, errors.New("Telegram OIDC configuration is incomplete")
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	ctx = coreoidc.ClientContext(ctx, httpClient)
	provider, err := coreoidc.NewProvider(ctx, config.Issuer.String())
	if err != nil {
		return nil, fmt.Errorf("discovering Telegram OIDC issuer: %w", err)
	}

	return &Client{
		oauth: oauth2.Config{
			ClientID:     config.ClientID,
			ClientSecret: config.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  config.CallbackURL.String(),
			Scopes:       []string{coreoidc.ScopeOpenID, "profile", "email"},
		},
		verifier:   provider.Verifier(&coreoidc.Config{ClientID: config.ClientID}),
		httpClient: httpClient,
	}, nil
}

// AuthorizationURL returns the provider URL for a state-bound flow with nonce and PKCE S256.
func (c *Client) AuthorizationURL(flow Flow) string {
	return c.oauth.AuthCodeURL(
		flow.State,
		oauth2.SetAuthURLParam("nonce", flow.Nonce),
		oauth2.S256ChallengeOption(flow.CodeVerifier),
	)
}

// Exchange exchanges a code and verifies the returned ID token.
func (c *Client) Exchange(ctx context.Context, code string, flow Flow) (Identity, error) {
	ctx = coreoidc.ClientContext(ctx, c.httpClient)
	token, err := c.oauth.Exchange(ctx, code, oauth2.VerifierOption(flow.CodeVerifier))
	if err != nil {
		return Identity{}, fmt.Errorf("exchanging Telegram authorization code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return Identity{}, errors.New("Telegram token response did not contain an ID token")
	}

	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Identity{}, fmt.Errorf("verifying Telegram ID token: %w", err)
	}

	claims := struct {
		Subject       string `json:"sub"`
		Nonce         string `json:"nonce"`
		Username      string `json:"preferred_username"`
		Name          string `json:"name"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}{}
	if err = idToken.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("decoding Telegram ID token claims: %w", err)
	}

	if claims.Nonce == "" || claims.Nonce != flow.Nonce {
		return Identity{}, errors.New("Telegram ID token nonce did not match the authentication flow")
	}

	if claims.Subject == "" {
		return Identity{}, errors.New("Telegram ID token subject is required")
	}

	return Identity{ProviderUserID: claims.Subject, Username: claims.Username, Name: claims.Name, Email: claims.Email, EmailVerified: claims.EmailVerified}, nil
}
