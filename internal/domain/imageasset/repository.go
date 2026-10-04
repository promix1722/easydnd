package imageasset

import "context"

// Repository is the persistence port for seeded artwork.
//
// It is deliberately two methods. There is no List because the pack on disk
// is the catalogue of what should exist; the repository only answers for a
// slug it is asked about. There is no Delete because seeds are retired by
// removing them from the pack, not from the database -- a row nothing
// requests costs nothing.
//
// Implementations live under internal/adapter/repository; internal/app picks
// the concrete one, and that assignment is what proves conformance.
type Repository interface {
	// Get returns the stored image, or a *types.NotFoundError if the slug was
	// never seeded.
	Get(ctx context.Context, slug string) (Image, error)

	// Upsert stores img under img.Slug. A stored row whose revision already
	// equals img.Revision is left untouched -- importing an unchanged seed is
	// a no-op, which is what makes a startup re-import cheap enough to run
	// unconditionally. A different revision replaces the stored bytes.
	Upsert(ctx context.Context, img Image) error
}
