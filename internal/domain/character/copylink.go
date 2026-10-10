package character

import "time"

// CopyLinkTTL is how long a copy link works for. A constant for the reason
// group.InviteTTL is one.
const CopyLinkTTL = 24 * time.Hour

// CopyLink is the claim a copy link carries: a copy of this character, offered
// by this owner.
//
// Like group.Invite it is a value, not a record: nothing is stored when one is
// made, so the link is reusable -- everyone it is forwarded to may take a copy
// -- and it cannot be withdrawn short of deleting the character. What it
// hands over is a copy and never the character itself, which is why none of
// that needs a table: the original is not changed by any number of redemptions.
type CopyLink struct {
	Character ID

	// From is the owner who made the link. It is checked against the
	// character's owner on redemption, so a link is only ever as good as the
	// right its maker still has.
	From OwnerID

	IssuedAt  time.Time
	ExpiresAt time.Time
}

// CopyLinks mints and checks copy-link tokens. A port for the reason
// group.Inviter is one.
type CopyLinks interface {
	// SignCopyLink renders a copy link as a token.
	SignCopyLink(l CopyLink) (string, error)

	// VerifyCopyLink checks a token against now and returns what it claims.
	// Any failure is a *types.UnauthenticatedError that does not say which
	// check it was.
	VerifyCopyLink(token string, now time.Time) (CopyLink, error)
}
