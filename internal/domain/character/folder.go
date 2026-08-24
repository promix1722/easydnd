package character

import (
	"context"
)

// FolderID identifies a folder.
type FolderID string

// String returns the identifier's text.
func (id FolderID) String() string { return string(id) }

// IsZero reports whether the identifier is unset.
func (id FolderID) IsZero() bool { return id == "" }

// Folder is a named place one account files its characters.
//
// It is filing and nothing more. A folder is not a party, a campaign or a
// shared space: it has exactly one owner, it grants nobody any access they did
// not already have, and no rule in the game reads it. The word *group* is
// deliberately absent from this file and from every route and screen this
// feature touches, because it is reserved for a group of players -- a different
// idea with a different owner model, which this one must not be mistaken for.
//
// A folder lives here rather than in a package of its own because it has no
// meaning apart from the characters in it, and because it shares OwnerID with
// them. A separate domain package would duplicate that type to say the same
// thing.
type Folder struct {
	ID    FolderID
	Owner OwnerID
	Name  string

	// Default marks the one folder an account always has and can never
	// delete. It is what makes "a character is always somewhere" true
	// without a nullable folder on every character.
	Default bool
}

// DefaultFolderName is what an account's default folder is called until its
// owner renames it.
//
// Renaming it is allowed: what protects the folder is the Default flag, not its
// name, and an account that would rather call it "Active" is not doing anything
// the model has to prevent.
const DefaultFolderName = "Default"

// FolderRepository is the persistence port for folders. Implementations live
// under internal/adapter/repository; internal/app picks the concrete one, and
// that assignment is what proves conformance at compile time.
type FolderRepository interface {
	// EnsureDefault returns owner's default folder, creating it if this is
	// the first time anyone has asked.
	//
	// It is a repository method rather than a get-or-create in the
	// application layer, and that is the whole point of it: two concurrent
	// requests from a new account would both read "no default" and both
	// create one. The store holds the lock, so the store holds the
	// invariant.
	EnsureDefault(ctx context.Context, owner OwnerID) (Folder, error)

	// Create stores a new folder for owner and returns it with its assigned
	// ID. The folder is never the default one -- only EnsureDefault makes
	// that.
	Create(ctx context.Context, owner OwnerID, name string) (Folder, error)

	// Get returns the folder with the given ID. Implementations report a
	// *types.NotFoundError when it does not exist.
	Get(ctx context.Context, id FolderID) (Folder, error)

	// List returns every folder owned by owner, the default one first and
	// the rest in a stable order.
	List(ctx context.Context, owner OwnerID) ([]Folder, error)

	// Rename changes a folder's name. Implementations report a
	// *types.NotFoundError when it does not exist.
	Rename(ctx context.Context, id FolderID, name string) error

	// Delete removes a folder, and only a folder: the characters filed in it
	// are the application layer's to deal with, because a repository that
	// reached into another aggregate's store would be two repositories
	// wearing one name.
	//
	// Implementations report a *types.NotFoundError when it does not exist
	// and a *types.ValidationError for the default folder, which is the one
	// an account is guaranteed to have.
	Delete(ctx context.Context, id FolderID) error
}
