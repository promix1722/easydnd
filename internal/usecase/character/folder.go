package character

import (
	"context"
	"strings"
	"unicode/utf8"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// maxFolderNameLen bounds a folder's name.
//
// The bound is generous and exists only to stop a listing being unrenderable;
// what a player calls their folder is not something this service has an opinion
// about.
const maxFolderNameLen = 120

// Folders returns owner's folders, default first.
//
// It materialises the default before listing, which is what makes "every
// account always has a folder" true without a migration that walks every
// existing account: the first read is the creation.
func (s *Service) Folders(ctx context.Context, owner domain.OwnerID) ([]domain.Folder, error) {
	if _, err := s.folders.EnsureDefault(ctx, owner); err != nil {
		return nil, err
	}
	return s.folders.List(ctx, owner)
}

// DefaultFolder returns owner's default folder, creating it on first use.
func (s *Service) DefaultFolder(ctx context.Context, owner domain.OwnerID) (domain.Folder, error) {
	return s.folders.EnsureDefault(ctx, owner)
}

// CreateFolder adds a folder for owner.
func (s *Service) CreateFolder(
	ctx context.Context, owner domain.OwnerID, name string,
) (domain.Folder, error) {
	name, err := validateFolderName(name)
	if err != nil {
		return domain.Folder{}, err
	}
	return s.folders.Create(ctx, owner, name)
}

// RenameFolder renames one of owner's folders, the default one included.
func (s *Service) RenameFolder(
	ctx context.Context, owner domain.OwnerID, id domain.FolderID, name string,
) (domain.Folder, error) {
	if _, err := s.ownedFolder(ctx, owner, id); err != nil {
		return domain.Folder{}, err
	}
	name, err := validateFolderName(name)
	if err != nil {
		return domain.Folder{}, err
	}
	if err := s.folders.Rename(ctx, id, name); err != nil {
		return domain.Folder{}, err
	}
	return s.folders.Get(ctx, id)
}

// ReorderFolders sets the order owner's folders are listed in.
//
// The default folder is not named and cannot be: it leads the listing, and a
// caller that includes it is asking for something the model does not offer.
// That refusal is a 400 rather than a 404, for the same reason deleting it is:
// the folder exists, the caller owns it, and the honest answer is that this
// particular folder does not move.
//
// Every other id goes through ownedFolder first, so a folder belonging to
// somebody else is a 404 here exactly as it is on a move or a rename -- the
// store would refuse it too, but as a set mismatch, and "one of these is not
// yours" is a worse answer than the one the choke point already gives.
func (s *Service) ReorderFolders(
	ctx context.Context, owner domain.OwnerID, ids []domain.FolderID,
) error {
	for _, id := range ids {
		folder, err := s.ownedFolder(ctx, owner, id)
		if err != nil {
			return err
		}
		if folder.Default {
			return types.NewValidationError(
				"%q is the default folder and is always listed first", folder.Name)
		}
	}
	// Materialised before reordering for the same reason Folders does it:
	// an account that has never listed its folders has no default yet, and
	// the store counts one when it decides whether the set is complete.
	if _, err := s.folders.EnsureDefault(ctx, owner); err != nil {
		return err
	}
	return s.folders.Reorder(ctx, owner, ids)
}

// DeleteFolder removes a folder and every character filed in it.
//
// The cascade is deliberate and it is destructive: a deleted folder takes its
// characters with it, and nothing here can give them back. Callers that put a
// button on this owe the player a confirmation that says how many characters
// are about to go.
//
// The characters go first and the folder last. There is no transaction across
// two stores, so one of the two orders has to be chosen for what it leaves
// behind when the process dies half way: this one leaves a folder holding fewer
// characters than it did, which is a state the application already understands.
// The other leaves characters filed in a folder that no longer exists, which
// nothing can list and nobody can reach.
func (s *Service) DeleteFolder(ctx context.Context, owner domain.OwnerID, id domain.FolderID) error {
	folder, err := s.ownedFolder(ctx, owner, id)
	if err != nil {
		return err
	}
	// Checked here as well as in the store, so the refusal does not depend
	// on which adapter is wired in.
	if folder.Default {
		return types.NewValidationError(
			"%q is the default folder and cannot be deleted", folder.Name)
	}

	characters, err := s.repo.List(ctx, owner)
	if err != nil {
		return err
	}
	for _, c := range characters {
		if c.Folder != id {
			continue
		}
		if err := s.repo.Delete(ctx, c.ID); err != nil {
			return err
		}
	}
	return s.folders.Delete(ctx, id)
}

// MoveCharacter files a character in another folder.
func (s *Service) MoveCharacter(
	ctx context.Context, owner domain.OwnerID, id domain.ID, folder domain.FolderID,
) error {
	if _, err := s.owned(ctx, owner, id); err != nil {
		return err
	}
	// Both ends are checked. Without the second, a caller could file their
	// own character into a folder belonging to somebody else and make it
	// vanish from their own listing.
	folder, err := s.resolveFolder(ctx, owner, folder)
	if err != nil {
		return err
	}
	return s.repo.SetFolder(ctx, id, folder)
}

// CopyCharacter duplicates a character, log and all.
//
// A zero target folder means "beside the original", which is what a Copy button
// on a row is asking for.
//
// The copy is built the way an import is -- create, then append the whole log
// at sequence zero -- and its new name arrives as one more appended event
// rather than as an edit of the init event it came with. That is not
// fastidiousness: the log's invariant is append, or drop a suffix, never edit
// the middle, and a copy that rewrote its own history would be the one record
// in the system that broke it.
func (s *Service) CopyCharacter(
	ctx context.Context,
	owner domain.OwnerID,
	id domain.ID,
	target domain.FolderID,
	locale rules.Locale,
) (domain.Character, error) {
	source, cat, err := s.load(ctx, owner, id, locale)
	if err != nil {
		return domain.Character{}, err
	}
	if target.IsZero() {
		target = source.Folder
	}
	target, err = s.resolveFolder(ctx, owner, target)
	if err != nil {
		return domain.Character{}, err
	}

	created, err := s.repo.Create(ctx, owner, target)
	if err != nil {
		return domain.Character{}, err
	}

	// Seq is zeroed rather than carried across: numbering is Log.Append's
	// job, and a caller that hands it numbers is asserting something it has
	// no way to know is still true.
	events := make([]domain.Event, 0, source.Log.Len()+1)
	for _, e := range source.Log.Events {
		e.Seq = 0
		events = append(events, e)
	}
	if name := domain.Summarize(id, owner, source.Folder, source.Log, cat).Name; name != "" {
		events = append(events, domain.Event{
			Type: domain.EventChange,
			Changes: []domain.Change{{
				Path:  "identity.name",
				Op:    domain.OpSet,
				Value: domain.StringValue(name + " (copy)"),
			}},
		})
	}

	if err := s.repo.Append(ctx, created.ID, 0, events...); err != nil {
		return domain.Character{}, err
	}
	return s.repo.Get(ctx, created.ID)
}

// resolveFolder turns a caller's folder into one owner definitely has: the
// zero value becomes their default, and anything else must be theirs.
func (s *Service) resolveFolder(
	ctx context.Context, owner domain.OwnerID, folder domain.FolderID,
) (domain.FolderID, error) {
	if folder.IsZero() {
		def, err := s.folders.EnsureDefault(ctx, owner)
		if err != nil {
			return "", err
		}
		return def.ID, nil
	}
	if _, err := s.ownedFolder(ctx, owner, folder); err != nil {
		return "", err
	}
	return folder, nil
}

// ownedFolder fetches a folder and refuses it to anyone but its owner.
//
// It is Service.owned for the other aggregate, down to the refusal being a
// NotFoundError: a 403 on somebody else's folder id confirms the id exists,
// and folders are numbered from one just as characters are. The reason to have
// two of these rather than one generic helper is that there is nothing generic
// to share -- two lines of it are the two stores.
func (s *Service) ownedFolder(
	ctx context.Context, owner domain.OwnerID, id domain.FolderID,
) (domain.Folder, error) {
	folder, err := s.folders.Get(ctx, id)
	if err != nil {
		return domain.Folder{}, err
	}
	if folder.Owner != owner {
		return domain.Folder{}, types.NewNotFoundError("folder %q", id)
	}
	return folder, nil
}

// validateFolderName trims a name and rejects an empty or overlong one.
func validateFolderName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", types.NewFieldValidationError("the folder could not be saved",
			types.FieldError{
				Field: "name", Rule: "required", Message: "a folder needs a name",
			})
	}
	if utf8.RuneCountInString(name) > maxFolderNameLen {
		return "", types.NewFieldValidationError("the folder could not be saved",
			types.FieldError{
				Field: "name", Rule: "length",
				Message: "a folder's name must be 120 characters or fewer",
			})
	}
	return name, nil
}
