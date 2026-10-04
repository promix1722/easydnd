package spellicon_test

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	file "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	packuc "github.com/promix1722/easydnd/internal/usecase/pack"
	spellicon "github.com/promix1722/easydnd/internal/usecase/spellicon"
)

// fixture mirrors the pack service's own test rig: the real SRD registry and
// the real pack resolver over memory stores, because authorization is exactly
// what the selector must not reimplement.
func fixture(t *testing.T) (*spellicon.Selector, *packuc.Service, *file.Authoring) {
	t.Helper()
	base, err := file.NewRegistry([]string{"../../../data/srd_5.1"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	repo := memory.NewPackRepository()
	users := memory.NewUserRepository()
	groups := memory.NewGroupRepository(users)
	engine := file.NewAuthoring(base, repo)
	packs := packuc.NewService(repo, engine, groups, users)
	return spellicon.NewSelector(packs, engine), packs, engine
}

func owner(name string) user.User {
	return user.User{ID: user.ID(name), DisplayName: name}
}

// publishExample imports and publishes the example pack, the same document
// the HTTP suite uses, so the private-spell assertions run on real content.
func publishExample(t *testing.T, ctx context.Context, s *packuc.Service, who user.User) domain.Record {
	t.Helper()
	raw, err := os.ReadFile("../../../data/packs/examples/tactician.json")
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Import(ctx, who, "Tactician", raw, false)
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.Publish(ctx, who.ID, r.ID, r.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func slugs(spells []spellicon.Spell) []string {
	out := []string{}
	for _, s := range spells {
		out = append(out, s.Slug)
	}
	return out
}

func hasSlug(spells []spellicon.Spell, slug string) bool {
	return slices.Contains(slugs(spells), slug)
}

// The aggregate scope is the SRD compendium the spells screen shows without a
// pack selection: every spell, unqualified slug, English name.
func TestSelectAggregateCatalogue(t *testing.T) {
	ctx := context.Background()
	sel, _, _ := fixture(t)

	spells, err := sel.Select(ctx, owner("anon:a").ID, spellicon.Selection{Locale: string(rules.LocaleEN)})
	if err != nil {
		t.Fatal(err)
	}
	if len(spells) < 300 {
		t.Fatalf("aggregate selection = %d spells, want the whole SRD", len(spells))
	}
	if !hasSlug(spells, "fireball") || !hasSlug(spells, "acid-splash") {
		t.Fatal("SRD spells missing from aggregate selection")
	}
	for _, s := range spells {
		if strings.Contains(s.Slug, "/") {
			t.Fatalf("SRD spell slug qualified: %q", s.Slug)
		}
	}
}

// A localized filter must match the name the visitor sees, while the Spell
// handed to the generator still carries the English name the prompt needs.
func TestSelectLocalizedFilterEnglishNames(t *testing.T) {
	ctx := context.Background()
	sel, _, _ := fixture(t)

	// "Брызги кислоты" is the Russian name of acid-splash; the English filter
	// text must not match it, and vice versa for the Russian catalogue.
	byRussian, err := sel.Select(ctx, owner("anon:a").ID, spellicon.Selection{
		Locale: string(rules.LocaleRU),
		Filter: catalog.SpellFilter{Name: "Брызги кислоты"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(byRussian) != 1 || byRussian[0].Slug != "acid-splash" {
		t.Fatalf("Russian name filter = %+v", byRussian)
	}
	if byRussian[0].Name != "Acid Splash" {
		t.Fatalf("prompt name = %q, want the English name", byRussian[0].Name)
	}
	if byRussian[0].School == "" {
		t.Fatal("school slug missing")
	}

	miss, err := sel.Select(ctx, owner("anon:a").ID, spellicon.Selection{
		Locale: string(rules.LocaleRU),
		Filter: catalog.SpellFilter{Name: "Acid Splash"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(miss) != 0 {
		t.Fatal("English text matched a Russian-only name")
	}
}

// The slug form is the per-row action: exactly that spell, regardless of what
// the filter says, as long as the scope can see it.
func TestSelectSlugIgnoresFilter(t *testing.T) {
	ctx := context.Background()
	sel, _, _ := fixture(t)

	spells, err := sel.Select(ctx, owner("anon:a").ID, spellicon.Selection{
		Slug:   "fireball",
		Filter: catalog.SpellFilter{Name: "definitely not fireball"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(spells) != 1 || spells[0].Slug != "fireball" || spells[0].Name != "Fireball" {
		t.Fatalf("slug selection = %+v", spells)
	}
}

func TestSelectUnknownSlugRejected(t *testing.T) {
	ctx := context.Background()
	sel, _, _ := fixture(t)

	if _, err := sel.Select(ctx, owner("anon:a").ID, spellicon.Selection{Slug: "not-a-spell"}); err == nil {
		t.Fatal("unknown slug selected")
	}
}

// Scoped selection resolves the exact release the screen pinned and contains
// the whole closure: the pack's own spells and the SRD spells it depends on.
func TestSelectScopedPackResolvesClosure(t *testing.T) {
	ctx := context.Background()
	sel, packs, _ := fixture(t)
	alice := owner("anon:alice")
	r := publishExample(t, ctx, packs, alice)
	release := r.Releases[0].Release

	spells, err := sel.Select(ctx, alice.ID, spellicon.Selection{
		Packs: []domain.Release{{ID: r.ID, Version: release.Version}},
	})
	if err != nil {
		t.Fatal(err)
	}
	found := []string{}
	for _, s := range spells {
		if strings.HasPrefix(s.Slug, r.ID+"/") {
			found = append(found, s.Slug)
		}
	}
	if len(found) == 0 {
		t.Fatal("own-pack spells missing from scoped selection")
	}
	if !hasSlug(spells, "fireball") {
		t.Fatal("dependency spells missing from scoped selection")
	}
	// The pack filter narrows to the pack's own rows even inside a resolved
	// closure, which is how the screen's pack filter reads.
	own, err := sel.Select(ctx, alice.ID, spellicon.Selection{
		Packs:  []domain.Release{{ID: r.ID, Version: release.Version}},
		Filter: catalog.SpellFilter{PackIDs: []string{r.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(own) != len(found) {
		t.Fatalf("pack filter kept %d of %d own spells", len(own), len(found))
	}
}

// A pack Alice has not published and has not been shared is simply not in her
// catalogue: the selector must refuse a scope that names it for someone else.
func TestSelectScopedPackIsAuthorized(t *testing.T) {
	ctx := context.Background()
	sel, packs, _ := fixture(t)
	alice := owner("anon:alice")
	bob := owner("anon:bob")
	r := publishExample(t, ctx, packs, alice)
	release := r.Releases[0].Release

	if _, err := sel.Select(ctx, bob.ID, spellicon.Selection{
		Packs: []domain.Release{{ID: r.ID, Version: release.Version}},
	}); err == nil {
		t.Fatal("another account resolved a private pack")
	}
}

// Browse is the spells screen's every-pack view: each accessible release at
// its chosen version, one row's own spells per pack. Versions pins a release
// the way ?versions= does on the catalogue route.
func TestSelectBrowseRespectsVersionsAndOwnership(t *testing.T) {
	ctx := context.Background()
	sel, packs, _ := fixture(t)
	alice := owner("anon:alice")
	bob := owner("anon:bob")
	r := publishExample(t, ctx, packs, alice)

	own := func(spells []spellicon.Spell) []string {
		var found []string
		for _, s := range spells {
			if strings.HasPrefix(s.Slug, r.ID+"/") {
				found = append(found, s.Slug)
			}
		}
		return found
	}

	aBrowse, err := sel.Select(ctx, alice.ID, spellicon.Selection{Browse: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(own(aBrowse)) == 0 {
		t.Fatal("owner's browse omitted the published pack")
	}
	if !hasSlug(aBrowse, "fireball") {
		t.Fatal("browse omitted the SRD builtins")
	}
	bBrowse, err := sel.Select(ctx, bob.ID, spellicon.Selection{Browse: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(own(bBrowse)) != 0 {
		t.Fatal("browse leaked a private pack's spells")
	}

	pinned, err := sel.Select(ctx, alice.ID, spellicon.Selection{
		Browse:   true,
		Versions: map[string]string{r.ID: r.Releases[0].Release.Version},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(own(pinned)) == 0 {
		t.Fatal("pinned release lost its spells")
	}
	if _, err := sel.Select(ctx, alice.ID, spellicon.Selection{
		Browse:   true,
		Versions: map[string]string{r.ID: "0.0.0"},
	}); err == nil {
		t.Fatal("unknown pinned version accepted")
	}
	if _, err := sel.Select(ctx, alice.ID, spellicon.Selection{
		Browse:   true,
		Versions: map[string]string{"no-such-pack": "1.0.0"},
	}); err == nil {
		t.Fatal("pinning an absent pack accepted")
	}
}

// A slug scoped to one pack finds only that pack's spells: the same local
// name in the SRD is a different spell and must not be substituted.
func TestSelectSlugInsideScopeOnly(t *testing.T) {
	ctx := context.Background()
	sel, packs, _ := fixture(t)
	alice := owner("anon:alice")
	r := publishExample(t, ctx, packs, alice)
	release := r.Releases[0].Release

	scoped := []domain.Release{{ID: r.ID, Version: release.Version}}
	if _, err := sel.Select(ctx, alice.ID, spellicon.Selection{
		Packs: scoped,
		Slug:  r.ID + "/guiding-mark",
	}); err != nil {
		t.Fatalf("qualified slug in scope: %v", err)
	}
	// The unqualified local name is not this scope's spell.
	if _, err := sel.Select(ctx, alice.ID, spellicon.Selection{
		Packs: scoped,
		Slug:  "guiding-mark",
	}); err == nil {
		t.Fatal("unqualified slug matched a pack-owned spell")
	}
}

// A second release makes the pin meaningful: with 2.1.0 published, pinning
// 2.0.0 must still select the older release rather than the newest.
func TestSelectBrowsePinsOlderRelease(t *testing.T) {
	ctx := context.Background()
	sel, packs, _ := fixture(t)
	alice := owner("anon:alice")
	r := publishExample(t, ctx, packs, alice)
	first := r.Releases[0].Release.Version

	var doc map[string]any
	if err := json.Unmarshal(r.Draft, &doc); err != nil {
		t.Fatal(err)
	}
	doc["manifest"].(map[string]any)["version"] = "2.1.0"
	doc["locales"].(map[string]any)["en"].(map[string]any)["spells"].(map[string]any)["guiding-mark"].(map[string]any)["name"] = "Guiding Mark Revised"
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	r, err = packs.Save(ctx, alice.ID, r.ID, r.Title, r.Revision, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err = packs.Publish(ctx, alice.ID, r.ID, r.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Releases) != 2 {
		t.Fatalf("releases = %d, want 2", len(r.Releases))
	}

	pinned, err := sel.Select(ctx, alice.ID, spellicon.Selection{
		Browse:   true,
		Versions: map[string]string{r.ID: first},
		Slug:     r.ID + "/guiding-mark",
	})
	if err != nil {
		t.Fatalf("pinning %s: %v", first, err)
	}
	if len(pinned) != 1 || pinned[0].Name != "Guiding Mark" {
		t.Fatalf("pinned selection = %+v, want the original English name", pinned)
	}
}

// The list form is the resume path: exactly the named spells, in the order
// the caller sent them, ignoring the filter, with repeats collapsed so a
// duplicated name can never queue the same paid icon twice.
func TestSelectSlugsExplicitSet(t *testing.T) {
	ctx := context.Background()
	sel, _, _ := fixture(t)

	spells, err := sel.Select(ctx, owner("anon:a").ID, spellicon.Selection{
		Slugs:  []string{"fireball", "acid-splash", "fireball"},
		Filter: catalog.SpellFilter{Name: "definitely not any of these"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := slugs(spells)
	if !slices.Equal(got, []string{"fireball", "acid-splash"}) {
		t.Fatalf("explicit set = %v", got)
	}
	if spells[0].Name != "Fireball" || spells[1].Name != "Acid Splash" {
		// The prompt vocabulary is English whatever locale the request
		// negotiated; these are the en names.
		t.Fatalf("names = %q, %q", spells[0].Name, spells[1].Name)
	}
}

// The list is all-or-nothing: a name the scope cannot hold -- absent from
// the catalogue or private to another account -- rejects the whole set
// rather than silently dropping to the spells that did resolve.
func TestSelectSlugsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	sel, packs, _ := fixture(t)
	alice := owner("anon:alice")
	bob := owner("anon:bob")
	r := publishExample(t, ctx, packs, alice)

	if _, err := sel.Select(ctx, alice.ID, spellicon.Selection{
		Slugs: []string{"fireball", "no-such-spell"},
	}); err == nil {
		t.Fatal("unknown name inside a list selected")
	}
	if _, err := sel.Select(ctx, bob.ID, spellicon.Selection{
		Browse: true,
		Slugs:  []string{"fireball", r.ID + "/guiding-mark"},
	}); err == nil {
		t.Fatal("private pack slug resolved for another account")
	}
	// The owner reaches the same set fine -- visibility, not spelling.
	spells, err := sel.Select(ctx, alice.ID, spellicon.Selection{
		Browse: true,
		Slugs:  []string{r.ID + "/guiding-mark", "fireball"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(slugs(spells), []string{r.ID + "/guiding-mark", "fireball"}) {
		t.Fatalf("owner's explicit set = %v", slugs(spells))
	}
}

// Slugs respects the scope the way the singular slug does: inside a pinned
// pack it resolves both the pack's own spells and its dependencies, and the
// unqualified name of a pack-owned spell is not this scope's spell.
func TestSelectSlugsInsideScopeOnly(t *testing.T) {
	ctx := context.Background()
	sel, packs, _ := fixture(t)
	alice := owner("anon:alice")
	r := publishExample(t, ctx, packs, alice)
	scoped := []domain.Release{{ID: r.ID, Version: r.Releases[0].Release.Version}}

	spells, err := sel.Select(ctx, alice.ID, spellicon.Selection{
		Packs: scoped,
		Slugs: []string{r.ID + "/guiding-mark", "fireball"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(slugs(spells), []string{r.ID + "/guiding-mark", "fireball"}) {
		t.Fatalf("scoped set = %v", slugs(spells))
	}
	if _, err := sel.Select(ctx, alice.ID, spellicon.Selection{
		Packs: scoped,
		Slugs: []string{"guiding-mark"},
	}); err == nil {
		t.Fatal("unqualified slug matched a pack-owned spell")
	}
}

// The usecase defends the shape too, not just the transport: singular and
// list never combine, and a present-but-empty list is not an empty run --
// a direct caller gets the same refusal the endpoint sends.
func TestSelectSlugsShapeIsValidated(t *testing.T) {
	ctx := context.Background()
	sel, _, _ := fixture(t)

	if _, err := sel.Select(ctx, owner("anon:a").ID, spellicon.Selection{
		Slug:  "fireball",
		Slugs: []string{"acid-splash"},
	}); err == nil {
		t.Fatal("slug and slugs together accepted")
	}
	if _, err := sel.Select(ctx, owner("anon:a").ID, spellicon.Selection{
		Slugs: []string{},
	}); err == nil {
		t.Fatal("empty slug list accepted")
	}
}
