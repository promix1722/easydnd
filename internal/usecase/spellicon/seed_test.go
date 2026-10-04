package spellicon_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/imageasset"
	"github.com/promix1722/easydnd/internal/types"
	uc "github.com/promix1722/easydnd/internal/usecase/spellicon"
)

// stubPack fakes the file adapter's contract: canonical keys, exact-then-
// legacy-basename resolution, and a SHA256-of-bytes revision. The bytes are
// raw test payloads -- SeedStore passes them through untouched, so there is
// nothing to decode.
type stubPack struct {
	files        map[string][]byte
	listErr      error
	existingErr  error
	saveErr      error
	readErr      error
	existingKeys []string
	saveCalls    int
}

func (p *stubPack) resolve(slug string) (string, bool) {
	if _, ok := p.files[slug]; ok {
		return slug, true
	}
	if base := path.Base(slug); strings.Contains(slug, "/") && base != slug {
		if _, ok := p.files[base]; ok {
			return base, true
		}
	}
	return "", false
}

func revisionOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (p *stubPack) Existing(_ context.Context, slug string) (string, bool, error) {
	if p.existingErr != nil {
		return "", false, p.existingErr
	}
	p.existingKeys = append(p.existingKeys, slug)
	key, ok := p.resolve(slug)
	if !ok {
		return "", false, nil
	}
	return revisionOf(p.files[key]), true, nil
}

func (p *stubPack) Save(_ context.Context, slug string, png []byte) (string, error) {
	p.saveCalls++
	if p.saveErr != nil {
		return "", p.saveErr
	}
	webp := append([]byte("webp:"), png...)
	p.files[slug] = webp
	return revisionOf(webp), nil
}

func (p *stubPack) Read(_ context.Context, slug string) (imageasset.Image, error) {
	if p.readErr != nil {
		return imageasset.Image{}, p.readErr
	}
	key, ok := p.resolve(slug)
	if !ok {
		return imageasset.Image{}, types.NewNotFoundError("no seed file %q", slug)
	}
	data := p.files[key]
	return imageasset.Image{
		Slug:        key,
		Revision:    revisionOf(data),
		ContentType: "image/webp",
		Data:        slices.Clone(data),
	}, nil
}

func (p *stubPack) List(context.Context) ([]string, error) {
	if p.listErr != nil {
		return nil, p.listErr
	}
	keys := make([]string, 0, len(p.files))
	for key := range p.files {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys, nil
}

// stubRepo fakes the durable repository, including its idempotent upsert: a
// stored row whose revision matches is not rewritten, and that is what keeps
// repeat seeds cheap.
type stubRepo struct {
	images        map[string]imageasset.Image
	getErr        error
	failUpserts   int
	getCalls      []string
	upsertCalls   int
	upsertedBytes int
}

func (r *stubRepo) Get(_ context.Context, slug string) (imageasset.Image, error) {
	r.getCalls = append(r.getCalls, slug)
	if r.getErr != nil {
		return imageasset.Image{}, r.getErr
	}
	img, ok := r.images[slug]
	if !ok {
		return imageasset.Image{}, types.NewNotFoundError("no image %q", slug)
	}
	return img.Clone(), nil
}

func (r *stubRepo) Upsert(_ context.Context, img imageasset.Image) error {
	if r.failUpserts > 0 {
		r.failUpserts--
		return errors.New("repository unavailable")
	}
	if stored, ok := r.images[img.Slug]; ok && stored.Revision == img.Revision {
		return nil
	}
	r.upsertCalls++
	r.upsertedBytes += len(img.Data)
	r.images[img.Slug] = img.Clone()
	return nil
}

func seeded(t *testing.T) (context.Context, *uc.SeedStore, *stubPack, *stubRepo) {
	t.Helper()
	pack := &stubPack{files: map[string][]byte{
		"acid-splash":  []byte("legacy-acid"),
		"pack-x/charm": []byte("namespaced-charm"),
	}}
	repo := &stubRepo{images: map[string]imageasset.Image{}}
	return context.Background(), uc.NewSeedStore(pack, repo), pack, repo
}

func TestSeedImportsPackAndIsIdempotent(t *testing.T) {
	ctx, seeds, _, repo := seeded(t)

	if err := seeds.Seed(ctx); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	for slug, data := range map[string]string{
		"acid-splash":  "legacy-acid",
		"pack-x/charm": "namespaced-charm",
	} {
		img, err := repo.Get(ctx, slug)
		if err != nil {
			t.Fatalf("repository has no %q: %v", slug, err)
		}
		if string(img.Data) != data {
			t.Errorf("%q data = %q, want %q", slug, img.Data, data)
		}
	}
	first := repo.upsertedBytes
	if first == 0 {
		t.Fatal("first seed wrote nothing")
	}
	if err := seeds.Seed(ctx); err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	if repo.upsertedBytes != first {
		t.Errorf("second seed rewrote %d bytes, want none (revisions already match)", repo.upsertedBytes-first)
	}
}

func TestSeedReplacesChangedRevisions(t *testing.T) {
	ctx, seeds, pack, repo := seeded(t)
	if err := seeds.Seed(ctx); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	pack.files["acid-splash"] = []byte("regenerated-acid")

	if err := seeds.Seed(ctx); err != nil {
		t.Fatalf("re-Seed: %v", err)
	}
	img, err := repo.Get(ctx, "acid-splash")
	if err != nil {
		t.Fatalf("repository Get: %v", err)
	}
	if string(img.Data) != "regenerated-acid" {
		t.Errorf("stored data = %q, want the new revision's bytes", img.Data)
	}
}

func TestSeedFailsWhenPackCannotList(t *testing.T) {
	ctx, seeds, pack, _ := seeded(t)
	pack.listErr = errors.New("open data/spell-icons: no such directory")

	if err := seeds.Seed(ctx); err == nil {
		t.Fatal("Seed with a missing pack directory succeeded; a silently empty repository would serve 404s")
	}
}

func TestSeedCollectsFailuresWithoutLosingTheRest(t *testing.T) {
	ctx, seeds, pack, repo := seeded(t)
	pack.readErr = errors.New("disk read failed")

	err := seeds.Seed(ctx)
	if err == nil {
		t.Fatal("Seed swallowed a pack read failure")
	}
	if !errors.Is(err, pack.readErr) {
		t.Errorf("Seed error %v does not wrap the read failure", err)
	}

	// A repository that refuses the first write still receives the second --
	// the pack seeds independently per file.
	pack.readErr = nil
	repo.failUpserts = 1
	if err := seeds.Seed(ctx); err == nil {
		t.Fatal("Seed swallowed a repository failure")
	}
	if len(repo.images) != 1 {
		t.Errorf("repository holds %d images, want the one whose import succeeded", len(repo.images))
	}
}

func TestExistingMissLeavesRepositoryAlone(t *testing.T) {
	ctx, seeds, _, repo := seeded(t)

	revision, exists, err := seeds.Existing(ctx, "never-drawn")
	if err != nil || exists {
		t.Fatalf("Existing = (%q, %v, %v), want a clean miss", revision, exists, err)
	}
	if len(repo.getCalls) != 0 || repo.upsertCalls != 0 {
		t.Errorf("repository touched on a miss: %v gets, %d upserts", repo.getCalls, repo.upsertCalls)
	}
}

func TestExistingImportsAPackHitBeforeReportingIt(t *testing.T) {
	ctx, seeds, _, repo := seeded(t)

	revision, exists, err := seeds.Existing(ctx, "acid-splash")
	if err != nil || !exists {
		t.Fatalf("Existing = (%q, %v, %v), want a hit", revision, exists, err)
	}
	img, err := repo.Get(ctx, "acid-splash")
	if err != nil {
		t.Fatalf("Existing reported artwork the repository cannot serve: %v", err)
	}
	if img.Revision != revision {
		t.Errorf("served revision %q != reported %q", img.Revision, revision)
	}

	// A namespaced request resolved by the legacy fallback lands under the
	// canonical key -- the same one Seed imports.
	revision, exists, err = seeds.Existing(ctx, "other-pack/acid-splash")
	if err != nil || !exists {
		t.Fatalf("Existing fallback = (%q, %v, %v), want the legacy hit", revision, exists, err)
	}
	if _, err := repo.Get(ctx, "other-pack/acid-splash"); !types.IsNotFound(err) {
		t.Errorf("fallback wrote a duplicate namespaced row: %v", err)
	}
}

func TestExistingSurfacesImportFailureInsteadOfClaimingSkip(t *testing.T) {
	ctx, seeds, _, repo := seeded(t)
	repo.failUpserts = 9

	if _, _, err := seeds.Existing(ctx, "acid-splash"); err == nil {
		t.Fatal("Existing reported success while the import failed; the icon would have been marked skipped yet unservable")
	}

	repo.failUpserts = 0
	if _, exists, err := seeds.Existing(ctx, "acid-splash"); err != nil || !exists {
		t.Fatalf("retry after the repository recovered: (%v, %v)", exists, err)
	}
	if _, err := repo.Get(ctx, "acid-splash"); err != nil {
		t.Fatalf("repository still missing the icon after retry: %v", err)
	}
}

func TestSavePublishesToPackAndRepository(t *testing.T) {
	ctx, seeds, pack, repo := seeded(t)

	revision, err := seeds.Save(ctx, "pack-x/new-spell", []byte("png-bytes"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, ok := pack.files["pack-x/new-spell"]; !ok {
		t.Fatal("Save did not keep the generated seed on disk")
	}
	img, err := repo.Get(ctx, "pack-x/new-spell")
	if err != nil {
		t.Fatalf("Save did not import to the repository: %v", err)
	}
	if img.Revision != revision {
		t.Errorf("served revision %q != Save's %q", img.Revision, revision)
	}
}

func TestSaveRepositoryFailureKeepsTheSeedOnDisk(t *testing.T) {
	ctx, seeds, pack, repo := seeded(t)
	repo.failUpserts = 9

	if _, err := seeds.Save(ctx, "new-spell", []byte("png-bytes")); err == nil {
		t.Fatal("Save reported success while the import failed")
	}
	if _, ok := pack.files["new-spell"]; !ok {
		t.Fatal("the paid-for seed was rolled back with the failed import -- it should stay on disk for retry")
	}

	// The retry pays nothing: the pack already holds the artwork, so Seed
	// repairs the repository outright.
	repo.failUpserts = 0
	if err := seeds.Seed(ctx); err != nil {
		t.Fatalf("Seed retry: %v", err)
	}
	if _, err := repo.Get(ctx, "new-spell"); err != nil {
		t.Fatalf("repository still missing the icon after Seed: %v", err)
	}
	if pack.saveCalls != 1 {
		t.Errorf("pack.Save ran %d times, want once -- retrying never regenerates", pack.saveCalls)
	}
}

func TestSavePackFailureWritesNothing(t *testing.T) {
	ctx, seeds, _, repo := seeded(t)
	pack := &stubPack{files: map[string][]byte{}, saveErr: errors.New("webp encode failed")}
	seeds = uc.NewSeedStore(pack, repo)

	if _, err := seeds.Save(ctx, "new-spell", []byte("png")); err == nil {
		t.Fatal("Save swallowed a pack failure")
	}
	if len(repo.images) != 0 {
		t.Errorf("repository was written although the pack save failed")
	}
}

func TestImageServesExactSlugOnlyFromRepository(t *testing.T) {
	ctx, seeds, pack, _ := seeded(t)
	if err := seeds.Seed(ctx); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	img, err := seeds.Image(ctx, "pack-x/charm")
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	if string(img.Data) != "namespaced-charm" {
		t.Errorf("data = %q, want namespaced-charm", img.Data)
	}

	// The pack holds artwork the repository never imported: serving it
	// anyway would make a partial seed indistinguishable from a complete one.
	pack.files["unsown"] = []byte("unsown-bytes")
	if _, err := seeds.Image(ctx, "unsown"); !types.IsNotFound(err) {
		t.Errorf("Image fell back to the pack at serve time: %v", err)
	}
}

func TestImageFallsBackToLegacyBasename(t *testing.T) {
	ctx, seeds, _, repo := seeded(t)
	if err := seeds.Seed(ctx); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	img, err := seeds.Image(ctx, "another-pack/acid-splash")
	if err != nil {
		t.Fatalf("Image fallback: %v", err)
	}
	if img.Slug != "acid-splash" {
		t.Errorf("Slug = %q, want the canonical acid-splash", img.Slug)
	}
	if !slices.Equal(repo.getCalls, []string{"another-pack/acid-splash", "acid-splash"}) {
		t.Errorf("repository lookups %v, want exact slug then basename", repo.getCalls)
	}
}

func TestImagePrefersNamespacedOverLegacy(t *testing.T) {
	ctx, seeds, pack, _ := seeded(t)
	pack.files["dnd-2014/acid-splash"] = []byte("namespaced-acid")
	if err := seeds.Seed(ctx); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	img, err := seeds.Image(ctx, "dnd-2014/acid-splash")
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	if string(img.Data) != "namespaced-acid" {
		t.Errorf("data = %q, want the namespaced icon, not the legacy fallback", img.Data)
	}
}

func TestImageMissAndMalformedSlugs(t *testing.T) {
	ctx, seeds, _, repo := seeded(t)

	for _, slug := range []string{
		"never-seeded",
		"pack/never-seeded",
		"..",
		"../etc/passwd",
		"a//b",
		"/absolute",
		"UPPER CASE",
		"acid-splash.webp",
		"",
	} {
		if _, err := seeds.Image(ctx, slug); !types.IsNotFound(err) {
			t.Errorf("Image(%q) = %v, want not found", slug, err)
		}
	}
	for _, got := range repo.getCalls {
		if strings.Contains(got, "..") || strings.Contains(got, ".webp") || got == "" {
			t.Errorf("malformed slug %q reached the repository", got)
		}
	}
}

func TestImagePropagatesRepositoryFailure(t *testing.T) {
	ctx, seeds, _, repo := seeded(t)
	repo.getErr = errors.New("connection refused")

	if _, err := seeds.Image(ctx, "pack-x/charm"); err == nil || types.IsNotFound(err) {
		t.Errorf("Image = %v, want the repository failure, not a fallback or 404", err)
	}
	if len(repo.getCalls) != 1 {
		t.Errorf("repository probed %d times, want one: a real error must not trigger the legacy fallback", len(repo.getCalls))
	}
}

func TestImageNeedsNoFilesystemWhenRepositoryIsSeeded(t *testing.T) {
	ctx := context.Background()
	repo := &stubRepo{images: map[string]imageasset.Image{
		"acid-splash": {Slug: "acid-splash", Revision: "rev1", ContentType: "image/webp", Data: []byte("art")},
	}}
	seeds := uc.NewSeedStore(&stubPack{files: map[string][]byte{}}, repo)

	img, err := seeds.Image(ctx, "acid-splash")
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	if string(img.Data) != "art" {
		t.Errorf("data = %q, want art", img.Data)
	}
}

// compile-time check that the coordinator still implements the queue's Store.
var _ uc.Store = (*uc.SeedStore)(nil)
