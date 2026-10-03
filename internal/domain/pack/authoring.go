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
	Draft     []byte
	Releases  []Document
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
}

// Engine is the adapter boundary for the existing portable codec and compiler.
type Engine interface {
	Builtins() []Record
	Default() Lock
	Schema() []byte
	NewDraft(string) []byte
	Fork([]byte, string, map[string]string) ([]byte, error)
	CheckDraft([]byte, string) error
	Resolve(context.Context, []Document, []Release) (Lock, error)
	Validate(context.Context, []Document, []byte) (Document, Lock, error)
}
