package token

import (
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
)

// A copy link and a group invite share a key and two claim slots. The kind
// claim is all that keeps a link to one group from being redeemed as a copy
// of a character, and the other way about.
func TestACopyLinkIsNotAnInvite(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now().Truncate(time.Second)

	link, err := s.SignCopyLink(character.CopyLink{
		Character: "chr_000001", From: "alice", IssuedAt: now, ExpiresAt: now.Add(character.CopyLinkTTL),
	})
	if err != nil {
		t.Fatalf("SignCopyLink: %v", err)
	}
	got, err := s.VerifyCopyLink(link, now)
	if err != nil {
		t.Fatalf("VerifyCopyLink: %v", err)
	}
	if got.Character != "chr_000001" || got.From != "alice" {
		t.Errorf("round trip = %+v, want chr_000001 from alice", got)
	}
	if _, err := s.VerifyCopyLink(link, now.Add(character.CopyLinkTTL+time.Second)); !types.IsUnauthenticated(err) {
		t.Errorf("an expired copy link verified: %v", err)
	}

	invite, err := s.SignInvite(testInvite(now))
	if err != nil {
		t.Fatalf("SignInvite: %v", err)
	}
	if _, err := s.VerifyCopyLink(invite, now); !types.IsUnauthenticated(err) {
		t.Errorf("an invite verified as a copy link: %v", err)
	}
	if _, err := s.VerifyInvite(link, now); !types.IsUnauthenticated(err) {
		t.Errorf("a copy link verified as an invite: %v", err)
	}
}
