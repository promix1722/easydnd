package character_test

import (
	"context"
	"errors"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// The promise the whole feature rests on: an account that has never touched a
// folder still has one, and asking is what makes it.
func TestFoldersMaterialisesTheDefault(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	folders, err := s.Folders(ctx, testOwner)
	if err != nil {
		t.Fatalf("Folders() error = %v", err)
	}
	if len(folders) != 1 {
		t.Fatalf("Folders() length = %d, want 1", len(folders))
	}
	if !folders[0].Default {
		t.Error("Folders() first entry is not the default")
	}

	// Asking twice does not make a second one.
	again, err := s.Folders(ctx, testOwner)
	if err != nil {
		t.Fatalf("Folders() error = %v", err)
	}
	if len(again) != 1 || again[0].ID != folders[0].ID {
		t.Errorf("Folders() second call = %+v, want the same single folder", again)
	}
}

func TestCreateFilesIntoTheDefaultWhenNoFolderIsNamed(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	c := mustCreate(t, s)
	if c.Folder.IsZero() {
		t.Fatal("Create() left the character in no folder")
	}

	def, err := s.DefaultFolder(ctx, testOwner)
	if err != nil {
		t.Fatalf("DefaultFolder() error = %v", err)
	}
	if c.Folder != def.ID {
		t.Errorf("Create() folder = %q, want the default %q", c.Folder, def.ID)
	}
}

func TestCreateRefusesAFolderTheOwnerDoesNotHave(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	theirs, err := s.CreateFolder(ctx, "somebody-else", "Theirs")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	// 404 rather than 403: a 403 on somebody else's folder id confirms the
	// id exists, and folders are numbered from one just as characters are.
	if _, err := s.Create(ctx, testOwner, theirs.ID, opening()); !types.IsNotFound(err) {
		t.Errorf("Create() into another owner's folder error = %v, want a NotFoundError", err)
	}
}

func TestCreateFolderRejectsAnEmptyName(t *testing.T) {
	s := newService(t)

	_, err := s.CreateFolder(context.Background(), testOwner, "   ")
	var fields *types.FieldValidationError
	if !errors.As(err, &fields) {
		t.Fatalf("CreateFolder(\"   \") error = %v, want a FieldValidationError", err)
	}
	if len(fields.Fields) != 1 || fields.Fields[0].Field != "name" {
		t.Errorf("CreateFolder() fields = %+v, want one error on name", fields.Fields)
	}
}

func TestCreateFolderTrimsTheName(t *testing.T) {
	s := newService(t)

	created, err := s.CreateFolder(context.Background(), testOwner, "  Campaign  ")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	if created.Name != "Campaign" {
		t.Errorf("CreateFolder() name = %q, want Campaign", created.Name)
	}
}

func TestRenameFolderIsRefusedToAnotherOwner(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	theirs, err := s.CreateFolder(ctx, "somebody-else", "Theirs")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	if _, err := s.RenameFolder(ctx, testOwner, theirs.ID, "Mine now"); !types.IsNotFound(err) {
		t.Errorf("RenameFolder() error = %v, want a NotFoundError", err)
	}
}

func TestMoveCharacterFilesItElsewhere(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	c := mustCreate(t, s)
	campaign, err := s.CreateFolder(ctx, testOwner, "Campaign")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	if err := s.MoveCharacter(ctx, testOwner, c.ID, campaign.ID); err != nil {
		t.Fatalf("MoveCharacter() error = %v", err)
	}

	listed, err := s.List(ctx, testOwner, campaign.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != c.ID {
		t.Fatalf("List(campaign) = %+v, want just %q", listed, c.ID)
	}
	if listed[0].Folder != campaign.ID {
		t.Errorf("summary folder = %q, want %q", listed[0].Folder, campaign.ID)
	}

	// And it is gone from where it was.
	def, err := s.DefaultFolder(ctx, testOwner)
	if err != nil {
		t.Fatalf("DefaultFolder() error = %v", err)
	}
	remaining, err := s.List(ctx, testOwner, def.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("List(default) = %+v, want empty", remaining)
	}
}

// The check that matters: without it a player could file their own character
// into somebody else's folder, where it would vanish from their own listing.
func TestMoveCharacterRefusesAnotherOwnersFolder(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	c := mustCreate(t, s)
	theirs, err := s.CreateFolder(ctx, "somebody-else", "Theirs")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	if err := s.MoveCharacter(ctx, testOwner, c.ID, theirs.ID); !types.IsNotFound(err) {
		t.Fatalf("MoveCharacter() error = %v, want a NotFoundError", err)
	}

	got, err := s.Get(ctx, testOwner, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Folder != c.Folder {
		t.Errorf("a refused move still changed the folder to %q", got.Folder)
	}
}

func TestMoveCharacterIsRefusedToAnotherOwner(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	c := mustCreate(t, s)
	if err := s.MoveCharacter(ctx, "somebody-else", c.ID, ""); !types.IsNotFound(err) {
		t.Errorf("MoveCharacter() by another owner error = %v, want a NotFoundError", err)
	}
}

func TestCopyDuplicatesTheLogAndSuffixesTheName(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	source := mustCreate(t, s)

	copied, err := s.CopyCharacter(ctx, testOwner, source.ID, "", rules.DefaultLocale)
	if err != nil {
		t.Fatalf("CopyCharacter() error = %v", err)
	}
	if copied.ID == source.ID {
		t.Fatal("CopyCharacter() returned the original")
	}
	if copied.Folder != source.Folder {
		t.Errorf("copy folder = %q, want the source's %q", copied.Folder, source.Folder)
	}

	// The source's events, then one more that renames the copy. The rename
	// is an append: the copy must not have rewritten the init event it was
	// duplicated from.
	if copied.Log.Len() != source.Log.Len()+1 {
		t.Fatalf("copy log length = %d, want %d", copied.Log.Len(), source.Log.Len()+1)
	}
	if copied.Log.Events[0].Type != domain.EventInit {
		t.Errorf("copy's first event = %q, want init", copied.Log.Events[0].Type)
	}
	if err := copied.Log.Validate(); err != nil {
		t.Errorf("copy's log is malformed: %v", err)
	}

	summaries, err := s.List(ctx, testOwner, "", rules.DefaultLocale)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	names := map[domain.ID]string{}
	for _, sum := range summaries {
		names[sum.ID] = sum.Name
	}
	if want := opening().Name; names[source.ID] != want {
		t.Errorf("source name = %q, want %q -- copying renamed the original", names[source.ID], want)
	}
	if want := opening().Name + " (copy)"; names[copied.ID] != want {
		t.Errorf("copy name = %q, want %q", names[copied.ID], want)
	}
}

func TestCopyCanLandInAnotherFolder(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	source := mustCreate(t, s)
	campaign, err := s.CreateFolder(ctx, testOwner, "Campaign")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	copied, err := s.CopyCharacter(ctx, testOwner, source.ID, campaign.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("CopyCharacter() error = %v", err)
	}
	if copied.Folder != campaign.ID {
		t.Errorf("copy folder = %q, want %q", copied.Folder, campaign.ID)
	}
}

func TestCopyIsRefusedToAnotherOwner(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	source := mustCreate(t, s)
	_, err := s.CopyCharacter(ctx, "somebody-else", source.ID, "", rules.DefaultLocale)
	if !types.IsNotFound(err) {
		t.Errorf("CopyCharacter() by another owner error = %v, want a NotFoundError", err)
	}
}

// The destructive one. Deleting a folder takes the characters filed in it, and
// this is the test that says so out loud.
func TestDeleteFolderTakesItsCharactersWithIt(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	kept := mustCreate(t, s)
	campaign, err := s.CreateFolder(ctx, testOwner, "Campaign")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	doomed, err := s.Create(ctx, testOwner, campaign.ID, opening())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := s.DeleteFolder(ctx, testOwner, campaign.ID); err != nil {
		t.Fatalf("DeleteFolder() error = %v", err)
	}

	if _, err := s.Get(ctx, testOwner, doomed.ID); !types.IsNotFound(err) {
		t.Errorf("Get() on a character in a deleted folder error = %v, want a NotFoundError", err)
	}
	// And only those: a cascade that reached past the folder would be a bug
	// nobody could undo.
	if _, err := s.Get(ctx, testOwner, kept.ID); err != nil {
		t.Errorf("Get() on a character in another folder error = %v, want it kept", err)
	}

	folders, err := s.Folders(ctx, testOwner)
	if err != nil {
		t.Fatalf("Folders() error = %v", err)
	}
	if len(folders) != 1 {
		t.Errorf("Folders() length = %d, want 1", len(folders))
	}
}

func TestDeleteFolderRefusesTheDefault(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	c := mustCreate(t, s)
	def, err := s.DefaultFolder(ctx, testOwner)
	if err != nil {
		t.Fatalf("DefaultFolder() error = %v", err)
	}

	err = s.DeleteFolder(ctx, testOwner, def.ID)
	var validation *types.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("DeleteFolder(default) error = %v, want a ValidationError", err)
	}
	// The refusal must be total: no half-done cascade that ate the
	// characters and then declined to remove the folder.
	if _, err := s.Get(ctx, testOwner, c.ID); err != nil {
		t.Errorf("Get() after a refused DeleteFolder error = %v, want the character kept", err)
	}
}

func TestDeleteFolderIsRefusedToAnotherOwner(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	theirs, err := s.CreateFolder(ctx, "somebody-else", "Theirs")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	if err := s.DeleteFolder(ctx, testOwner, theirs.ID); !types.IsNotFound(err) {
		t.Errorf("DeleteFolder() error = %v, want a NotFoundError", err)
	}
}

func TestListRefusesAFolderTheCallerDoesNotOwn(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	theirs, err := s.CreateFolder(ctx, "somebody-else", "Theirs")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	// Not an empty list: that would read as "their folder is empty" and
	// confirm the id exists.
	if _, err := s.List(ctx, testOwner, theirs.ID, rules.DefaultLocale); !types.IsNotFound(err) {
		t.Errorf("List() with another owner's folder error = %v, want a NotFoundError", err)
	}
}

func TestReorderFoldersSetsTheListing(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	first, err := s.CreateFolder(ctx, testOwner, "Tuesday game")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	second, err := s.CreateFolder(ctx, testOwner, "Retired")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	if err := s.ReorderFolders(ctx, testOwner, []domain.FolderID{second.ID, first.ID}); err != nil {
		t.Fatalf("ReorderFolders() error = %v", err)
	}

	folders, err := s.Folders(ctx, testOwner)
	if err != nil {
		t.Fatalf("Folders() error = %v", err)
	}
	if len(folders) != 3 {
		t.Fatalf("Folders() length = %d, want 3", len(folders))
	}
	if !folders[0].Default {
		t.Error("Folders() first entry is not the default")
	}
	if folders[1].ID != second.ID || folders[2].ID != first.ID {
		t.Errorf("Folders() order = %q,%q, want %q,%q",
			folders[1].ID, folders[2].ID, second.ID, first.ID)
	}
}

// The default folder leads the listing, so there is no position for it to
// take. Naming it is a 400 rather than a 404 for the same reason deleting it
// is: it exists, the caller owns it, and this particular folder does not move.
func TestReorderFoldersRefusesTheDefault(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	def, err := s.DefaultFolder(ctx, testOwner)
	if err != nil {
		t.Fatalf("DefaultFolder() error = %v", err)
	}
	other, err := s.CreateFolder(ctx, testOwner, "Tuesday game")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	err = s.ReorderFolders(ctx, testOwner, []domain.FolderID{def.ID, other.ID})
	var validation *types.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("ReorderFolders() error = %v, want a ValidationError", err)
	}
}

// The same 404 a move or a rename gives, and from the same choke point: a
// folder id the caller does not own is one that, as far as they can tell,
// does not exist.
func TestReorderFoldersIsRefusedAnotherOwnersFolder(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	mine, err := s.CreateFolder(ctx, testOwner, "Mine")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	theirs, err := s.CreateFolder(ctx, "somebody-else", "Theirs")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	err = s.ReorderFolders(ctx, testOwner, []domain.FolderID{mine.ID, theirs.ID})
	if !types.IsNotFound(err) {
		t.Errorf("ReorderFolders() error = %v, want a NotFoundError", err)
	}
}

// An account whose only folder is the default has nothing to order, and an
// empty body is the honest way to say so rather than an error.
func TestReorderFoldersAcceptsNothingToOrder(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	if _, err := s.DefaultFolder(ctx, testOwner); err != nil {
		t.Fatalf("DefaultFolder() error = %v", err)
	}
	if err := s.ReorderFolders(ctx, testOwner, nil); err != nil {
		t.Errorf("ReorderFolders() error = %v, want nil", err)
	}
}
