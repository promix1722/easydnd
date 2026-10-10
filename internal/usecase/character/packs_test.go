package character_test

import (
	"context"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

func TestRevisionTokenRejectsSameLengthRewrite(t *testing.T) {
	t.Parallel()
	s := newService(t)
	c := mustCreate(t, s)
	ctx := context.Background()
	event := c.Log.Events[0]
	event.Changes[0].Value = domain.StringValue("First edit")
	result, err := s.Revise(charuc.WithRevision(ctx, c.Revision), testOwner, c.ID, rules.LocaleEN, c.Log.LastSeq(), 1, &event, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Seq != c.Log.LastSeq() || result.Revision <= c.Revision {
		t.Fatal("invalid revision/sequence semantics")
	}
	event.Changes[0].Value = domain.StringValue("Stale edit")
	if _, err = s.Revise(charuc.WithRevision(ctx, c.Revision), testOwner, c.ID, rules.LocaleEN, c.Log.LastSeq(), 1, &event, true); err == nil {
		t.Fatal("stale rewrite accepted")
	}
}
