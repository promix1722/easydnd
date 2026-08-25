package group

import (
	"time"

	"github.com/promix1722/easydnd/internal/domain/user"
)

// InviteTTL is how long an invite link works for.
//
// It is a constant rather than a configuration key on purpose. Configuration
// here is a single YAML file in which an unknown key is a fatal startup error,
// so adding one costs a two-release dance -- install the config, then the
// binary that reads it -- and that is a heavy price for a number nobody has
// asked to change. If it ever needs to vary, it becomes a key then.
const InviteTTL = 24 * time.Hour

// Invite is the claim an invite link carries: this group, at this rank,
// offered by this member.
//
// It is a value, not a record. Nothing is stored when an invite is made and
// nothing is consulted when one is accepted -- the token is the whole invite,
// and it is believed because it verifies and has not expired. The consequences
// are deliberate and worth stating where somebody will read them:
//
//   - the link is reusable, so it seats everyone it is forwarded to, and
//   - it cannot be withdrawn; the only bound on a leaked link is InviteTTL.
//
// The upgrade path, if that ever stops being acceptable, is a stored invite
// whose id rides in the token and is checked on accept.
type Invite struct {
	Group ID

	// Role is the rank the recipient is seated at. It is never RoleOwner: a
	// group gets its owner by being created and changes it by an explicit
	// transfer, so an invite that could mint one would be a way to take a
	// table over by sending somebody a link.
	Role Role

	// InvitedBy names the member who made the link, so the recipient can see
	// who is asking before they accept.
	InvitedBy user.ID

	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Inviter mints and checks invite tokens.
//
// It is a port for exactly the reason auth.Signer is: the implementation is a
// JWT, the JWT library reaches net/http, and both `make lint/layers` and
// depguard forbid that here. Nothing above the adapter may assume the token
// has any particular format -- it is an opaque string that goes in a link.
type Inviter interface {
	// SignInvite renders an invite as a token.
	SignInvite(i Invite) (string, error)

	// VerifyInvite checks a token against now and returns what it claims.
	// A token that is malformed, expired, signed by another key, or issued
	// for some other purpose yields a *types.UnauthenticatedError that does
	// not say which of those it was.
	VerifyInvite(token string, now time.Time) (Invite, error)
}
