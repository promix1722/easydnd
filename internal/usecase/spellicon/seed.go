package spellicon

import (
	"context"
	"errors"
	"path"
	"regexp"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/imageasset"
	"github.com/promix1722/easydnd/internal/types"
)

// SeedPack is the disk side of seeded artwork: the directories of WebP files
// the repository is seeded from. A seed-pack slug is the flat file name minus
// its .webp suffix, qualified by the directory that owns it -- "acid-arrow"
// in the SRD base directory, "dnd-2014/acid-arrow" for the local file in the
// directory configured for that pack.
//
// internal/adapter/imagegen/file implements it. Read resolves a slug in the
// directory that owns its namespace first and then against the flat legacy
// name, and reports the canonical slug it resolved, so an image imported
// here lands under the same key List advertises.
type SeedPack interface {
	Existing(ctx context.Context, slug string) (revision string, exists bool, err error)
	Save(ctx context.Context, slug string, png []byte) (revision string, err error)
	Read(ctx context.Context, slug string) (imageasset.Image, error)
	List(ctx context.Context) ([]string, error)
}

// SeedStore is the application's piece between the seed pack on disk and the
// image repository: it is the Store the generation queue sees, the Seed the
// startup path runs, and the source the image endpoint serves from.
//
// The pack and the repository keep the same artwork, but the pack is what
// makes generation idempotent -- it survives a restart even when nothing else
// does, so a seed that was paid for once is never paid for twice. That is why
// every read below can only ever go to the repository, while every write goes
// to the pack first: the repository is seeded from the pack, not the other
// way round.
type SeedStore struct {
	pack SeedPack
	repo imageasset.Repository
}

// NewSeedStore wires the pack and the repository into the coordinator.
func NewSeedStore(pack SeedPack, repo imageasset.Repository) *SeedStore {
	return &SeedStore{pack: pack, repo: repo}
}

// Existing reports whether finished artwork already answers slug. When the
// pack holds artwork the repository has not seen -- a seed left behind by a
// failed import, or a file restored from an older tree -- the hit is imported
// before it is reported, so a "exists" answer always means the image endpoint
// can serve it.
func (s *SeedStore) Existing(ctx context.Context, slug string) (revision string, exists bool, err error) {
	revision, exists, err = s.pack.Existing(ctx, slug)
	if err != nil || !exists {
		return revision, exists, err
	}
	if err := s.importSeed(ctx, slug); err != nil {
		return "", false, err
	}
	return revision, true, nil
}

// Save writes generated artwork to the pack first and then imports it into
// the repository. An import failure does not lose the artwork: the pack still
// holds it, and Seed on startup or Existing on the next request retires the
// retry without paying for another generation.
func (s *SeedStore) Save(ctx context.Context, slug string, png []byte) (revision string, err error) {
	revision, err = s.pack.Save(ctx, slug, png)
	if err != nil {
		return "", err
	}
	if err := s.importSeed(ctx, slug); err != nil {
		return "", err
	}
	return revision, nil
}

// Seed imports every file the pack lists into the repository.
//
// It runs at startup in every environment, so it has to be cheap once the
// repository is already seeded: the repository's upsert leaves a row whose
// stored revision already matches untouched, and a restart re-reads the pack
// without rewriting what it agrees with. Any import failure is reported --
// a missing pack directory means the artwork simply is not there, and
// starting up to serve 404s would hide that.
func (s *SeedStore) Seed(ctx context.Context) error {
	slugs, err := s.pack.List(ctx)
	if err != nil {
		return err
	}
	var failed error
	for _, slug := range slugs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.importSeed(ctx, slug); err != nil {
			// One bad file must not stop the rest of the pack: every import
			// that succeeds is one the next startup does not owe.
			failed = errors.Join(failed, err)
		}
	}
	return failed
}

// Image serves artwork from the repository -- never from the pack at request
// time, which would make a half-seeded database silently reachable and a
// restart a way to resurrect deleted rows.
//
// A namespaced slug that has no artwork of its own falls back to the flat
// legacy icon of its last segment, the same resolution the pack applies: the
// seed tree shipped flat files, and spells name them that way.
func (s *SeedStore) Image(ctx context.Context, slug string) (imageasset.Image, error) {
	if !validSeedSlug(slug) {
		return imageasset.Image{}, types.NewNotFoundError("spell icon not found")
	}
	img, err := s.repo.Get(ctx, slug)
	if err != nil {
		// The fallback keys on NotFound only: a repository error says nothing
		// about whether the flat name would hit, and serving the legacy icon
		// for a slug that genuinely has one would hide the failure it should
		// be answering for.
		if base := path.Base(slug); strings.Contains(slug, "/") && types.IsNotFound(err) && base != slug {
			return s.repo.Get(ctx, base)
		}
		return imageasset.Image{}, err
	}
	return img, nil
}

// importSeed copies one file of the pack into the repository. Read returns
// the canonical slug it resolved, so the upsert always keys the image the way
// List names it.
func (s *SeedStore) importSeed(ctx context.Context, slug string) error {
	img, err := s.pack.Read(ctx, slug)
	if err != nil {
		return err
	}
	return s.repo.Upsert(ctx, img)
}

// seedSegment is one path segment of a seed slug: the pack file's own rule,
// kept here because a malformed slug should be refused before it ever reaches
// the repository.
var seedSegment = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,119}$`)

// validSeedSlug reports whether slug is shaped like a seed key: segments of
// seed characters joined by slashes, nothing else. It is the serving-side
// guard -- the repository stores under these keys only, so a slug that fails
// here can only be a 404 anyway.
func validSeedSlug(slug string) bool {
	if slug == "" {
		return false
	}
	for _, segment := range strings.Split(slug, "/") {
		if !seedSegment.MatchString(segment) {
			return false
		}
	}
	return true
}
