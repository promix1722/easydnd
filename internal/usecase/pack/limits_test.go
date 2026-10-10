package pack_test

import (
	"errors"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

func TestPackLimit(t *testing.T) {
	ctx, s, _, _, _ := fixture(t)
	l := types.DefaultLimits
	l.Packs = 1
	s.SetLimits(l)

	r := create(t, ctx, s, "alice")
	alice := user.User{ID: "alice", Anonymous: true, DisplayName: "alice"}
	_, err := s.Create(ctx, alice, "Second", nil, nil)
	var refused *types.ValidationError
	if !errors.As(err, &refused) || refused.Reason != "limit.packs" {
		t.Fatalf("error = %v, want limit.packs", err)
	}
	create(t, ctx, s, "bob")

	// An archived pack gives its place back: it cannot be deleted.
	if _, err := s.Archive(ctx, "alice", r.ID, r.Revision); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if _, err := s.Create(ctx, alice, "Second", nil, nil); err != nil {
		t.Fatalf("Create() after archiving: %v", err)
	}
}
