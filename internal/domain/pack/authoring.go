package pack

import (
	"context"

	"github.com/promix1722/easydnd/internal/domain/user"
)

// Documents retain the portable JSON bytes. Authoring metadata never enters a release.
type Document struct {
	Release Release
	Data    []byte
}
type Record struct {
	ID, Title string
	Owner     user.ID
	Revision  int
	Archived  bool
	// Restricted marks a disk pack from data.private_pack_files: unowned like
	// every installed pack, but not everybody's.
	Restricted bool
	Draft      []byte
	Releases   []Document
}
type Share struct {
	Group, Pack string
	Contributor user.ID
	Lock        Lock
}
type Repository interface {
	List(context.Context) ([]Record, error)
	Get(context.Context, string) (Record, error)
	Save(context.Context, Record, int) error
	Shares(context.Context, string) ([]Share, error)
	PutShare(context.Context, Share) error
	DeleteShare(context.Context, string, string) error

	// PutPrivate keeps a release an import compiled for one character, so
	// that the character can be loaded after the process that compiled it
	// is gone. It is never listed: a private release is reachable only
	// through the lock that pins it. Storing the same release twice is not
	// an error.
	PutPrivate(context.Context, Document) error
	// GetPrivate returns a stored private release, or a *types.NotFoundError.
	GetPrivate(context.Context, Release) (Document, error)
}

// Engine is the adapter boundary for the existing portable codec and compiler.
type Engine interface {
	Builtins() []Record
	Default() Lock
	Schema() []byte
	NewDraft(string) []byte
	ImportZIP([]byte) ([]byte, error)
	Fork([]byte, string, map[string]string) ([]byte, error)
	CheckDraft([]byte, string) error
	Resolve(context.Context, []Document, []Release) (Lock, error)
	Validate(context.Context, []Document, []byte) (Document, Lock, error)
}
