// Package oidc implements the auth.Federation port over OpenID Connect.
//
// It is the only package that imports golang.org/x/oauth2 and go-oidc, both of
// which reach net/http -- which depguard and `make lint/layers` forbid in the
// domain and usecase layers. Everything crossing back inward is a plain string
// or a domain type, so the application layer never sees a protocol type. This
// is the same arrangement internal/adapter/webauthn has, for the same reason.
package oidc

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	domain "github.com/promix1722/easydnd/internal/domain/auth"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// GoogleIssuer is Google's OIDC issuer URL. Discovery reads its
// .well-known/openid-configuration to find the authorization, token and JWKS
// endpoints, so this constant is the only Google URL the code carries.
const GoogleIssuer = "https://accounts.google.com"

// Scopes we ask Google for, and nothing more: who this is, and what to call
// them. The application has no use for a Google API on the person's behalf, so
// it never asks for a token that could reach one.
var googleScopes = []string{oidc.ScopeOpenID, "email", "profile"}

// Config carries what a provider needs to identify itself to the issuer.
type Config struct {
	ClientID     string
	ClientSecret string
	// RedirectURL must match a URI registered with the provider byte for
	// byte. It is configuration rather than something derived from the
	// request, because deriving it from a Host header would let a caller
	// choose where the code is delivered.
	RedirectURL string

	// Issuer overrides where discovery points. It exists so the tests can
	// stand up an issuer of their own rather than reaching the internet, and
	// defaults to Google everywhere else -- nothing outside this package's
	// tests sets it.
	Issuer string
}

func (c Config) issuer() string {
	if c.Issuer == "" {
		return GoogleIssuer
	}
	return c.Issuer
}

// discoveryTimeout bounds the call to the issuer's well-known document.
//
// go-oidc discovers through http.DefaultClient, which has no timeout of its
// own, and discovery runs under a lock -- so a peer that accepts the
// connection and then says nothing would otherwise hang every sign-in in the
// process, permanently and with no client cancellation able to unwind it.
const discoveryTimeout = 10 * time.Second

// Google is the auth.Federation implementation for Google sign-in.
type Google struct {
	cfg Config

	// Discovery is deferred rather than done in the constructor. go-oidc's
	// NewProvider performs a network call, and doing that at startup would
	// make the process refuse to boot whenever accounts.google.com is
	// unreachable -- which deploy.sh's health gate would read as a bad
	// release and roll back. A provider that is briefly unreachable should
	// fail the sign-ins attempted during that window and nothing else.
	//
	// A mutex rather than a sync.Once, because a failed discovery has to be
	// retryable and a Once cannot be reset: re-zeroing one while another
	// goroutine is inside Do races on its internal mutex, and would let a
	// second failed attempt write a nil provider out from under a caller
	// already using a good one. Holding the lock across the network call also
	// means a hundred simultaneous first sign-ins perform one discovery
	// rather than a hundred, which is why the timeout above is not optional.
	mu       sync.Mutex
	provider *oidc.Provider
}

// NewGoogle builds the adapter. It performs no I/O: see the note on discovery.
func NewGoogle(cfg Config) (*Google, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("client id and client secret are both required")
	}
	if cfg.RedirectURL == "" {
		return nil, errors.New("redirect url is required")
	}
	return &Google{cfg: cfg}, nil
}

// AuthCodeURL builds the URL to send the browser to.
//
// A failed discovery cannot be reported here, because the port returns no
// error -- so it yields an empty string and the usecase treats that as
// "provider unavailable". Discovery is retried on the next attempt.
func (g *Google) AuthCodeURL(state, nonce, verifier string) string {
	// The port takes no context -- building a URL is not I/O from the
	// application's point of view -- so the bound is applied here.
	ctx, cancel := context.WithTimeout(context.Background(), discoveryTimeout)
	defer cancel()

	cfg, _, err := g.oauthConfig(ctx)
	if err != nil {
		return ""
	}
	return cfg.AuthCodeURL(state,
		oidc.Nonce(nonce),
		// Takes the verifier and derives the S256 challenge itself; it is the
		// counterpart of the VerifierOption in Exchange, and pairing the two
		// is what keeps them in agreement.
		oauth2.S256ChallengeOption(verifier),
		// Google returns a refresh token only when asked, and we never ask:
		// there is nothing to call on the person's behalf once they are
		// signed in, so holding one would be a credential with no use and a
		// real cost if leaked.
		oauth2.SetAuthURLParam("access_type", "online"),
	)
}

// Exchange trades an authorization code for a verified identity.
func (g *Google) Exchange(ctx context.Context, code, nonce, verifier string) (user.Identity, error) {
	cfg, provider, err := g.oauthConfig(ctx)
	if err != nil {
		return user.Identity{}, err
	}

	token, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		// The provider's message can name our client id and redirect uri, so
		// it goes to the caller as an opaque failure and to the log as
		// itself. Nothing here is the visitor's fault or under their control.
		return user.Identity{}, types.WrapServerError(err, "exchange authorization code with google")
	}

	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return user.Identity{}, types.WrapServerError(
			errors.New("no id_token in the token response"), "google sign-in")
	}

	// Verify checks the signature against the issuer's JWKS, the issuer
	// itself, the audience against our client id, and expiry. The token
	// arrived over TLS directly from the token endpoint, which makes the
	// signature check belt-and-braces rather than the only defence -- but the
	// audience check is not, and it is what stops an ID token minted for a
	// different application from signing someone in here.
	idToken, err := provider.Verifier(&oidc.Config{ClientID: g.cfg.ClientID}).Verify(ctx, raw)
	if err != nil {
		return user.Identity{}, types.NewUnauthenticatedError("google did not prove who this is")
	}
	// The nonce binds this token to the authorization request we started.
	// Without it a token obtained in some other flow could be replayed here.
	if idToken.Nonce != nonce {
		return user.Identity{}, types.NewUnauthenticatedError("google sign-in did not match the request that started it")
	}

	var claims struct {
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return user.Identity{}, types.WrapServerError(err, "read google identity claims")
	}
	if claims.Subject == "" {
		return user.Identity{}, types.WrapServerError(
			errors.New("id token carries no subject"), "google sign-in")
	}

	return user.Identity{
		Provider:      user.ProviderGoogle,
		Subject:       claims.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		DisplayName:   claims.Name,
	}, nil
}

// oauthConfig discovers the issuer's endpoints once and builds the exchange
// configuration over them. It returns the provider alongside, so callers use
// the one they were given rather than re-reading the field.
func (g *Google) oauthConfig(ctx context.Context) (*oauth2.Config, *oidc.Provider, error) {
	provider, err := g.discover(ctx)
	if err != nil {
		return nil, nil, err
	}
	return &oauth2.Config{
		ClientID:     g.cfg.ClientID,
		ClientSecret: g.cfg.ClientSecret,
		RedirectURL:  g.cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       googleScopes,
	}, provider, nil
}

// discover resolves the issuer's endpoints, at most once while it succeeds.
//
// A failure is not cached: an issuer unreachable for one request must not be
// unreachable for the life of the process. Note that go-oidc does not retain
// ctx -- its later JWKS fetches build their own -- so handing it a deadline
// here bounds discovery without breaking token verification afterwards.
func (g *Google) discover(ctx context.Context) (*oidc.Provider, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.provider != nil {
		return g.provider, nil
	}
	provider, err := oidc.NewProvider(ctx, g.cfg.issuer())
	if err != nil {
		return nil, types.WrapServerError(err, "reach the google identity service")
	}
	g.provider = provider
	return provider, nil
}

// Compile-time proof that this adapter satisfies the port.
var _ domain.Federation = (*Google)(nil)
