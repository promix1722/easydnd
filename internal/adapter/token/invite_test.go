package token

import (
	"testing"
	"time"

	domain "github.com/promix1722/easydnd/internal/domain/auth"
	group "github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/types"
)

func testInvite(now time.Time) group.Invite {
	return group.Invite{
		Group:     "grp_abc",
		Role:      group.RolePlayer,
		InvitedBy: "alice",
		IssuedAt:  now,
		ExpiresAt: now.Add(group.InviteTTL),
	}
}

func TestSignInviteRoundTrip(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now().Truncate(time.Second)

	token, err := s.SignInvite(testInvite(now))
	if err != nil {
		t.Fatalf("SignInvite: %v", err)
	}

	got, err := s.VerifyInvite(token, now)
	if err != nil {
		t.Fatalf("VerifyInvite: %v", err)
	}
	if got.Group != "grp_abc" {
		t.Errorf("group = %q, want %q", got.Group, "grp_abc")
	}
	if got.Role != group.RolePlayer {
		t.Errorf("role = %q, want %q", got.Role, group.RolePlayer)
	}
	if got.InvitedBy != "alice" {
		t.Errorf("inviter = %q, want %q", got.InvitedBy, "alice")
	}
	if !got.ExpiresAt.Equal(now.Add(group.InviteTTL)) {
		t.Errorf("expires at = %v, want %v", got.ExpiresAt, now.Add(group.InviteTTL))
	}
}

// An invite lasts a day, and after that it is refused. This is the only thing
// bounding a link that is reusable and cannot be withdrawn.
func TestVerifyInviteRejectsExpired(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now().Truncate(time.Second)

	token, err := s.SignInvite(testInvite(now))
	if err != nil {
		t.Fatalf("SignInvite: %v", err)
	}

	_, err = s.VerifyInvite(token, now.Add(group.InviteTTL+time.Second))
	if !types.IsUnauthenticated(err) {
		t.Fatalf("VerifyInvite = %v, want an unauthenticated error", err)
	}
}

func TestVerifyInviteRejectsForeignKey(t *testing.T) {
	mine := NewSigner(testSecret, time.Hour)
	theirs := NewSigner([]byte("fedcba9876543210fedcba9876543210"), time.Hour)
	now := time.Now().Truncate(time.Second)

	token, err := theirs.SignInvite(testInvite(now))
	if err != nil {
		t.Fatalf("SignInvite: %v", err)
	}
	if _, err := mine.VerifyInvite(token, now); !types.IsUnauthenticated(err) {
		t.Fatalf("VerifyInvite = %v, want an unauthenticated error", err)
	}
}

// The invariant the shared signing key rests on.
//
// One key signs session cookies, ceremony cookies and invite links. Without
// the kind claim, the session cookie every signed-in visitor already holds
// would verify perfectly well as an invitation to any group whose id they
// could guess -- and an invite link, which is meant to be forwarded to
// strangers, would verify as somebody's session.
func TestAnInviteIsNotASessionOrACeremony(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now().Truncate(time.Second)

	invite, err := s.SignInvite(testInvite(now))
	if err != nil {
		t.Fatalf("SignInvite: %v", err)
	}
	session, err := s.SignSession(domain.Session{
		UserID: "alice", IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("SignSession: %v", err)
	}
	ceremony, err := s.Seal([]byte("state"), time.Minute, now)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if _, err := s.VerifySession(invite, now); !types.IsUnauthenticated(err) {
		t.Errorf("an invite verified as a session: %v", err)
	}
	if _, err := s.Open(invite, now); !types.IsUnauthenticated(err) {
		t.Errorf("an invite opened as a ceremony: %v", err)
	}
	if _, err := s.VerifyInvite(session, now); !types.IsUnauthenticated(err) {
		t.Errorf("a session verified as an invite: %v", err)
	}
	if _, err := s.VerifyInvite(ceremony, now); !types.IsUnauthenticated(err) {
		t.Errorf("a ceremony verified as an invite: %v", err)
	}
}

// An invite may never seat its holder as the owner: a link that did would be a
// way to give somebody else's table away. Refused on the way in and, because
// this is the value that decides a permission, again on the way out.
func TestAnInviteCannotGrantOwnership(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now().Truncate(time.Second)

	invite := testInvite(now)
	invite.Role = group.RoleOwner
	if _, err := s.SignInvite(invite); err == nil {
		t.Fatal("SignInvite minted an ownership invite")
	}

	invite.Role = "sovereign"
	if _, err := s.SignInvite(invite); err == nil {
		t.Fatal("SignInvite minted an invite for a role that does not exist")
	}
}

func TestSignInviteRejectsAnEmptyGroup(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	invite := testInvite(time.Now())
	invite.Group = ""
	if _, err := s.SignInvite(invite); err == nil {
		t.Fatal("SignInvite minted an invite to no group")
	}
}

func TestVerifyInviteRejectsEmpty(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	if _, err := s.VerifyInvite("", time.Now()); !types.IsUnauthenticated(err) {
		t.Fatalf("VerifyInvite = %v, want an unauthenticated error", err)
	}
}
