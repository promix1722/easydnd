// Package auth holds the passkey sign-in endpoints.
//
// Convention for this tree: one exported handler per file, named after the
// action, with its request and response types beside it.
//
// The handlers are deliberately thin. Every ceremony decision -- what the
// challenge is, how long it lives, whose credential it is -- belongs to the
// usecase; this package moves bytes between HTTP and that service and decides
// nothing except which cookie to set.
package auth

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// maxCeremonyBody caps a finish request. An attestation object is a few
// kilobytes; gin imposes no limit of its own, and an unbounded read on an
// unauthenticated endpoint is an invitation.
const maxCeremonyBody = 64 << 10

// Service is the sign-in usecase this handler drives.
type Service interface {
	BeginRegistration(ctx context.Context) (options []byte, ceremony string, err error)
	FinishRegistration(ctx context.Context, ceremony string, responseBody []byte) (user.User, string, error)
	BeginLogin(ctx context.Context) (options []byte, ceremony string, err error)
	FinishLogin(ctx context.Context, ceremony string, responseBody []byte) (user.User, string, error)
	SignInAnonymously(ctx context.Context) (user.User, string, error)
	Session(ctx context.Context, token string) (user.User, error)
	SessionTTL() time.Duration
	GuestSessionTTL() time.Duration
	CeremonyTTL() time.Duration

	Providers() []user.Provider
	StartSSO(ctx context.Context, provider user.Provider, returnTo string, linkTo *user.ID) (redirect, flight string, err error)
	FinishSSO(ctx context.Context, provider user.Provider, flight, state, code string, sessionUserID user.ID) (account user.User, token, returnTo string, err error)
	Unlink(ctx context.Context, id user.ID, provider user.Provider, subject string) (user.User, error)
}

// Handler serves the sign-in endpoints.
type Handler struct {
	svc     Service
	cookies helpers.CookieOptions
}

// New builds the handler.
func New(svc Service, cookies helpers.CookieOptions) *Handler {
	return &Handler{svc: svc, cookies: cookies}
}

// User is the wire form of an account. It carries the display name and the
// inventory of ways in, and nothing that would be worth stealing.
type User struct {
	ID          string       `json:"id"`
	DisplayName string       `json:"display_name"`
	CreatedAt   string       `json:"created_at"`
	Credentials []Credential `json:"credentials"`
	Identities  []Identity   `json:"identities"`
	// Anonymous marks a guest session -- no account, no passkeys, no linked
	// providers, and nothing that survives the token. The client needs it to
	// stop offering account management to somebody who has no account.
	Anonymous bool `json:"anonymous"`
}

// Identity is the wire form of one linked external account.
//
// The email is here because it is the only thing that tells two Google
// accounts apart on screen; the subject is here because unlinking has to name
// one, and it is already the provider's own public identifier for the person
// rather than anything we minted.
type Identity struct {
	Provider    string `json:"provider"`
	Subject     string `json:"subject"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	CreatedAt   string `json:"created_at"`
	LastUsedAt  string `json:"last_used_at"`
}

// Credential is the wire form of one registered passkey.
//
// BackedUp is here because the UI needs it: a passkey that does not sync to a
// password manager is a single point of failure for an account that has no
// recovery path, and the only honest thing to do is say so on screen.
type Credential struct {
	ID         string `json:"id"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at"`
	BackedUp   bool   `json:"backed_up"`
}

// SessionResponse is the body of every endpoint that establishes or reports a
// session.
type SessionResponse struct {
	User User `json:"user"`
}

// options writes a ceremony's browser options through unchanged.
//
// c.Data rather than c.JSON: the usecase already produced the exact JSON the
// WebAuthn API expects, and decoding it here only to re-encode it would risk
// changing it.
func (h *Handler) options(c *gin.Context, payload []byte, ceremony string) {
	h.cookies.SetCeremony(c, ceremony, h.svc.CeremonyTTL())
	c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
}

// establish sets the session cookie and answers with the account.
//
// ttl is passed rather than read from the service, because a guest session and
// an account session have different lifetimes and the cookie must expire with
// the token inside it -- a cookie that outlives its token is a browser that
// keeps presenting something the server will only reject.
func (h *Handler) establish(c *gin.Context, account user.User, token string, ttl time.Duration) {
	// The ceremony is spent the moment it resolves; clearing it here stops a
	// sealed challenge from being presented a second time.
	h.cookies.ClearCeremony(c)
	h.cookies.SetSession(c, token, ttl)
	c.JSON(http.StatusOK, SessionResponse{User: toWire(account)})
}

// ceremonyBody reads a finish request's raw WebAuthn response.
//
// The body is the browser's own PublicKeyCredential JSON, passed to the
// ceremony verbatim. Binding it to a struct here would mean re-serialising it,
// and the signature covers the exact bytes.
func ceremonyBody(c *gin.Context) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxCeremonyBody+1))
	if err != nil {
		return nil, types.NewValidationError("could not read the request body")
	}
	if len(body) > maxCeremonyBody {
		return nil, types.NewValidationError("request body is too large").Because("request.tooLarge")
	}
	if len(body) == 0 {
		return nil, types.NewValidationError("request body is empty").Because("request.empty")
	}
	return body, nil
}

func toWire(u user.User) User {
	credentials := make([]Credential, 0, len(u.Credentials))
	for _, c := range u.Credentials {
		credentials = append(credentials, Credential{
			// The credential id is opaque to the client and only ever used to
			// name one in a list, so a URL-safe rendering is all it needs.
			ID:         base64.RawURLEncoding.EncodeToString(c.ID),
			CreatedAt:  c.CreatedAt.UTC().Format(time.RFC3339),
			LastUsedAt: c.LastUsedAt.UTC().Format(time.RFC3339),
			BackedUp:   c.BackupState,
		})
	}
	identities := make([]Identity, 0, len(u.Identities))
	for _, i := range u.Identities {
		identities = append(identities, Identity{
			Provider:    string(i.Provider),
			Subject:     i.Subject,
			Email:       i.Email,
			DisplayName: i.DisplayName,
			CreatedAt:   i.CreatedAt.UTC().Format(time.RFC3339),
			LastUsedAt:  i.LastUsedAt.UTC().Format(time.RFC3339),
		})
	}
	return User{
		ID:          string(u.ID),
		DisplayName: u.DisplayName,
		CreatedAt:   u.CreatedAt.UTC().Format(time.RFC3339),
		Credentials: credentials,
		Identities:  identities,
		Anonymous:   u.Anonymous,
	}
}
