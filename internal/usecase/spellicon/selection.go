package spellicon

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	packuc "github.com/promix1722/easydnd/internal/usecase/pack"
)

// Selection names the catalogue slice one icon run works on.
//
// Exactly one of Browse or Packs picks the releases; neither means the
// aggregate SRD compendium, which is what the spells screen calls "scope".
// Filter is the screen's search box: every field of it applies, and there is
// no paging -- a run is queued over the whole matching set. Locale is the
// visitor's own language, so a name typed in Russian still finds the spell,
// while the Spell values handed to the generator always carry the English
// name the prompt is written for. Slug picks one spell directly, ignoring the
// filter, which is what the per-row action sends. Slugs is the same direct
// pick stretched to an explicit list -- a resume of exactly the icons a
// previous run left unfinished -- and the two never mix: a Selection that
// sets both, or sends an empty list, is refused.
type Selection struct {
	// Browse reads every release the caller may see, latest unless Versions
	// pins one -- the "browse" scope on the spells screen.
	Browse bool
	// Packs is an exact rules context: the "?packs=id@version" scope, resolved
	// and authorized as one lock.
	Packs []domain.Release
	// Versions overrides the latest-release choice per pack when Browse is
	// set; keyed by pack id, valued "1.2.0".
	Versions map[string]string
	Filter   catalog.SpellFilter
	Locale   string
	// Slug picks one spell directly, ignoring the filter.
	Slug string
	// Slugs picks an explicit set the same way; nil means a normal filtered
	// run, a non-nil slice -- even an empty one -- is the list itself.
	Slugs []string
}

// Selector turns a Selection into the spells a run will work on.
//
// Authorization is not repeated here: it delegates to the pack service's
// List and Resolve, so a caller can only ever reach spells the catalogue
// endpoints would already have shown them. Everything runs in process -- the
// catalogue is the same Source the HTTP layer serves.
type Selector struct {
	packs  *packuc.Service
	source catalog.Source
}

func NewSelector(packs *packuc.Service, source catalog.Source) *Selector {
	return &Selector{packs: packs, source: source}
}

// pair is one resolved set of releases read twice: once in the visitor's
// locale, which is what the filter's free text must match, and once in
// English, which is the language the artwork prompt is written in.
type pair struct {
	cat *catalog.Catalog
	en  *catalog.Catalog
	// only restricts the pair to one pack's own spells, the way browsing does:
	// a lock pulled in for dependencies must not nominate its neighbour's
	// entries for artwork under this row.
	only string
}

// Select resolves the scope and returns every spell the run should cover.
func (s *Selector) Select(ctx context.Context, owner user.ID, sel Selection) ([]Spell, error) {
	if sel.Slugs != nil && (sel.Slug != "" || len(sel.Slugs) == 0) {
		// Singular and list never mix, and a present-but-empty list names
		// nothing: both are a malformed request, not an empty run.
		return nil, types.NewValidationError("invalid spell selection").Because(ReasonInvalidRequest)
	}
	locale := rules.Locale(sel.Locale)
	if locale == "" {
		locale = rules.DefaultLocale
	}
	var pairs []pair
	switch {
	case sel.Browse:
		found, err := s.browse(ctx, owner, locale, sel.Versions)
		if err != nil {
			return nil, err
		}
		pairs = found
	case len(sel.Packs) > 0:
		lock, err := s.packs.Resolve(ctx, owner, sel.Packs)
		if err != nil {
			return nil, err
		}
		p, err := s.pair(ctx, locale, lock, "")
		if err != nil {
			return nil, err
		}
		pairs = []pair{p}
	default:
		p, err := s.pair(ctx, locale, domain.Lock{}, "")
		if err != nil {
			return nil, err
		}
		pairs = []pair{p}
	}
	if sel.Slug != "" {
		sel.Slugs = []string{sel.Slug}
	}
	if sel.Slugs != nil {
		// Named spells, wherever the scope keeps them. A qualified slug can
		// only belong to its own pack, so the first pair that holds it is the
		// answer; a slug nobody holds was never in this scope. The list is
		// all-or-nothing -- one name that does not resolve fails the whole
		// set rather than quietly generating the reachable subset -- and it
		// dedupes on the catalogue slug so one paid icon can never be
		// queued twice under two spellings.
		out := make([]Spell, 0, len(sel.Slugs))
		seen := make(map[string]bool, len(sel.Slugs))
		for _, slug := range sel.Slugs {
			resolved, found := Spell{}, false
			for _, p := range pairs {
				if spell, ok := p.cat.Spells.Get(rules.Slug(slug)); ok && p.has(spell) {
					resolved, found = p.spellOf(spell), true
					break
				}
			}
			if !found {
				return nil, types.NewValidationError("no such spell in the selected catalogue").Because(ReasonInvalidRequest)
			}
			if !seen[resolved.Slug] {
				seen[resolved.Slug] = true
				out = append(out, resolved)
			}
		}
		return out, nil
	}
	out := []Spell{}
	for _, p := range pairs {
		for _, spell := range p.cat.Spells.All() {
			if p.has(spell) && sel.Filter.Matches(spell) {
				out = append(out, p.spellOf(spell))
			}
		}
	}
	return out, nil
}

// pair loads one lock in both locales. The English load is free whenever the
// request already negotiated it, and any other time it is a second read of
// data the source keeps immutable and cached.
func (s *Selector) pair(ctx context.Context, locale rules.Locale, lock domain.Lock, only string) (pair, error) {
	cat, err := catalog.LoadLocked(ctx, s.source, locale, lock)
	if err != nil {
		return pair{}, err
	}
	en := cat
	if locale != rules.DefaultLocale {
		en, err = catalog.LoadLocked(ctx, s.source, rules.DefaultLocale, lock)
		if err != nil {
			return pair{}, err
		}
	}
	return pair{cat: cat, en: en, only: only}, nil
}

// has reports whether the spell belongs to this pair at all. Unrestricted
// pairs take everything the catalogue holds; pack-pinned pairs take only the
// entries that pack itself owns.
func (p pair) has(spell catalog.Spell) bool {
	return p.only == "" || (spell.Provenance != nil && spell.Provenance.PackID == p.only)
}

// spellOf projects a catalogue spell onto the generator's vocabulary: the
// slug it will be filed under, the English name the prompt interpolates, and
// the school slug the prompt looks its palette up by. The school is a slug
// rather than prose, so it is the same in every locale -- it is read from the
// English catalogue anyway to stay inside the one-language contract.
func (p pair) spellOf(spell catalog.Spell) Spell {
	out := Spell{Slug: spell.Slug.String(), Name: spell.Name, School: spell.School.String()}
	if en, ok := p.en.Spells.Get(spell.Slug); ok {
		out.Name = en.Name
		out.School = en.School.String()
	}
	return out
}

// browse mirrors the packs catalogue's browsing read: every release the
// caller can see, newest version unless Versions pins one, each release
// resolved with its dependency closure. The release-choice rules are copied
// from internal/api/http/v1/pack/browse.go so the artwork tool sees exactly
// the catalogue the screen in front of it was showing -- including skipping
// a release that resolves or loads badly rather than failing the whole set.
func (s *Selector) browse(ctx context.Context, owner user.ID, locale rules.Locale, versions map[string]string) ([]pair, error) {
	rows, err := s.packs.List(ctx, owner)
	if err != nil {
		return nil, err
	}
	overrides := maps.Clone(versions)
	if overrides == nil {
		overrides = map[string]string{}
	}
	pairs := []pair{}
	for _, row := range rows {
		if row.Archived || len(row.Releases) == 0 {
			continue
		}
		slices.SortFunc(row.Releases, func(a, b domain.Document) int {
			av, ae := semver.StrictNewVersion(a.Release.Version)
			bv, be := semver.StrictNewVersion(b.Release.Version)
			if ae != nil || be != nil {
				return strings.Compare(a.Release.Version, b.Release.Version)
			}
			return av.Compare(bv)
		})
		chosen := row.Releases[len(row.Releases)-1].Release
		available := []string{}
		for _, r := range row.Releases {
			available = append(available, r.Release.Version)
			if overrides[row.ID] == r.Release.Version {
				chosen = r.Release
			}
		}
		if version := overrides[row.ID]; version != "" {
			if !slices.Contains(available, version) {
				return nil, types.NewNotFoundError("release unavailable").Because("pack.unavailable")
			}
			delete(overrides, row.ID)
		}
		lock, err := s.packs.Resolve(ctx, owner, []domain.Release{chosen})
		if err != nil {
			continue
		}
		p, err := s.pair(ctx, locale, lock, row.ID)
		if err != nil {
			continue
		}
		pairs = append(pairs, p)
	}
	if len(overrides) > 0 {
		return nil, types.NewNotFoundError("release unavailable").Because("pack.unavailable")
	}
	return pairs, nil
}
