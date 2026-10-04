package memory

import (
	"context"
	"sync"

	"github.com/promix1722/easydnd/internal/domain/imageasset"
	"github.com/promix1722/easydnd/internal/types"
)

// ImageRepository is the in-process image store, used where no database is
// configured. It is real storage, not a stub: bytes survive until the process
// exits, which is exactly the contract the seed importer relies on during
// DB-less development.
type ImageRepository struct {
	mu     sync.Mutex
	images map[string]imageasset.Image
}

func NewImageRepository() *ImageRepository {
	return &ImageRepository{images: map[string]imageasset.Image{}}
}

func (r *ImageRepository) Get(_ context.Context, slug string) (imageasset.Image, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	img, ok := r.images[slug]
	if !ok {
		return img, types.NewNotFoundError("image not found")
	}
	return img.Clone(), nil
}

func (r *ImageRepository) Upsert(_ context.Context, img imageasset.Image) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Same rule as Postgres: an identical revision is already stored, so the
	// write is skipped rather than repeated.
	if stored, ok := r.images[img.Slug]; ok && stored.Revision == img.Revision {
		return nil
	}
	r.images[img.Slug] = img.Clone()
	return nil
}
