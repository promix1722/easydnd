package character_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/adapter/token"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

const recipient domain.OwnerID = "somebody-else"

func newLinkedService(t *testing.T) *charuc.Service {
	t.Helper()
	s := newService(t)
	s.SetCopyLinks(token.NewSigner([]byte("0123456789abcdef0123456789abcdef"), time.Hour))
	return s
}

func TestACopyLinkGivesItsHolderTheirOwnCopy(t *testing.T) {
	t.Parallel()
	s := newLinkedService(t)
	ctx := context.Background()
	source := mustCreate(t, s)

	link, _, err := s.CreateCopyLink(ctx, testOwner, source.ID)
	if err != nil {
		t.Fatalf("CreateCopyLink() error = %v", err)
	}

	preview, err := s.PreviewCopyLink(ctx, link, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("PreviewCopyLink() error = %v", err)
	}
	if want := opening().Name; preview.Name != want {
		t.Errorf("preview name = %q, want %q", preview.Name, want)
	}

	copied, err := s.AcceptCopyLink(ctx, recipient, link, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("AcceptCopyLink() error = %v", err)
	}
	if copied.Owner != recipient || copied.ID == source.ID {
		t.Fatalf("copy = %q owned by %q, want a new character owned by %q", copied.ID, copied.Owner, recipient)
	}
	def, err := s.DefaultFolder(ctx, recipient)
	if err != nil {
		t.Fatalf("DefaultFolder() error = %v", err)
	}
	if copied.Folder != def.ID {
		t.Errorf("copy folder = %q, want the recipient's default %q", copied.Folder, def.ID)
	}
	// No rename: the log is the source's, event for event.
	if copied.Log.Len() != source.Log.Len() {
		t.Errorf("copy log length = %d, want %d", copied.Log.Len(), source.Log.Len())
	}

	// The sender still has the original and has not gained the copy.
	mine, err := s.List(ctx, testOwner, "", rules.DefaultLocale)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(mine) != 1 || mine[0].ID != source.ID {
		t.Errorf("sender's characters = %+v, want only the original", mine)
	}
}

func TestOnlyTheOwnerMayMintACopyLink(t *testing.T) {
	t.Parallel()
	s := newLinkedService(t)
	source := mustCreate(t, s)

	if _, _, err := s.CreateCopyLink(context.Background(), recipient, source.ID); !types.IsNotFound(err) {
		t.Errorf("CreateCopyLink() by a stranger error = %v, want a NotFoundError", err)
	}
}

func TestACopyLinkDiesWithItsCharacter(t *testing.T) {
	t.Parallel()
	s := newLinkedService(t)
	ctx := context.Background()
	source := mustCreate(t, s)

	link, _, err := s.CreateCopyLink(ctx, testOwner, source.ID)
	if err != nil {
		t.Fatalf("CreateCopyLink() error = %v", err)
	}
	if err := s.Delete(ctx, testOwner, source.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.AcceptCopyLink(ctx, recipient, link, rules.DefaultLocale); !types.IsNotFound(err) {
		t.Errorf("AcceptCopyLink() after deletion error = %v, want a NotFoundError", err)
	}
}

// A 401 here would sign out whoever clicked a stale link.
func TestABadCopyLinkIsABadArgumentNotALostSession(t *testing.T) {
	t.Parallel()
	s := newLinkedService(t)

	_, err := s.AcceptCopyLink(context.Background(), recipient, "not-a-token", rules.DefaultLocale)
	var invalid *types.ValidationError
	if !errors.As(err, &invalid) {
		t.Errorf("AcceptCopyLink() with a bad token error = %v, want a ValidationError", err)
	}
}
