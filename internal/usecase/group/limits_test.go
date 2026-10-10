package group_test

import (
	"context"
	"errors"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/types"
)

func wantLimit(t *testing.T, err error, what string) {
	t.Helper()
	var refused *types.ValidationError
	if !errors.As(err, &refused) || refused.Reason != "limit."+what {
		t.Fatalf("error = %v, want limit.%s", err, what)
	}
}

func TestGroupLimits(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	l := types.DefaultLimits
	l.Groups, l.GroupMembers = 1, 2
	f.svc.SetLimits(l)

	g, err := f.svc.Create(ctx, account("alice"), "The Table")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	_, err = f.svc.Create(ctx, account("alice"), "Another")
	wantLimit(t, err, "groups")

	invitation, err := f.svc.Invite(ctx, "alice", g.ID, domain.RolePlayer)
	if err != nil {
		t.Fatalf("Invite() error = %v", err)
	}
	if _, err := f.svc.Accept(ctx, account("bob"), invitation.Token); err != nil {
		t.Fatalf("Accept(bob) error = %v", err)
	}
	_, err = f.svc.Accept(ctx, account("carol"), invitation.Token)
	wantLimit(t, err, "groupMembers")
	// Somebody already seated is not refused by a full group.
	if _, err := f.svc.Accept(ctx, account("bob"), invitation.Token); err != nil {
		t.Fatalf("Accept(bob) again error = %v", err)
	}
	// Being a member of a group does not use up the groups one may own.
	if _, err := f.svc.Create(ctx, account("bob"), "Bob's"); err != nil {
		t.Fatalf("Create(bob) error = %v", err)
	}
}
