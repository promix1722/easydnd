package agent

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/agnivade/levenshtein"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// AgentCandidate is a catalogue entry a name may have meant.
type AgentCandidate = charuc.Candidate

func normalizedName(s string) string {
	// Russian prints "ё" or "е" for the same letter, by the typesetter's taste.
	return strings.Join(strings.FieldsFunc(strings.ReplaceAll(strings.ToLower(s), "ё", "е"), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }), " ")
}

// nameScore says how likely a printed name is a catalogue name.
//
// Sheets print names their own way: "Rope, Hempen" for "Hempen Rope",
// "Arrows" for "Arrow", "Rations" for "Rations (1 day)". Those are the same
// identity, so they are compared as sets of singular words before spelling
// distance gets a say. loose additionally lets the sheet say more than the
// catalogue does -- "Path of the Totem Warrior" for "Totem Warrior" -- which is
// only safe where the candidates are a closed set: unscoped, it would turn
// "Fire Shield" into "Shield".
func nameScore(query, name string, loose bool) float64 {
	q, n := normalizedName(query), normalizedName(name)
	if q == "" || n == "" {
		return 0
	}
	if q == n {
		return 1
	}
	qt, nt := strings.Fields(q), strings.Fields(n)
	// These words change spell identity rather than spelling. Never normalize
	// Mass/Greater/Lesser away or rank their absence as a typo.
	for _, word := range []string{"mass", "greater", "lesser", "масс", "массовое", "массовый", "массовая", "множественное", "множественный", "множественная", "высшее", "высший", "высшая", "малое", "малый", "малая"} {
		if slices.Contains(qt, word) != slices.Contains(nt, word) {
			return 0
		}
	}
	qs, ns := singulars(qt), singulars(nt)
	queryInName, nameInQuery := subset(qs, ns), subset(ns, qs)
	switch {
	case queryInName && nameInQuery:
		return .99
	case queryInName, nameInQuery && loose:
		return .9
	}
	score := 1 - float64(levenshtein.ComputeDistance(q, n))/float64(max(len([]rune(q)), len([]rune(n))))
	overlap := 0
	for _, t := range qt {
		if slices.Contains(nt, t) {
			overlap++
		}
	}
	token := float64(2*overlap) / float64(len(qt)+len(nt))
	return max(score, token*.95)
}

func singulars(tokens []string) []string {
	out := make([]string, len(tokens))
	for i, t := range tokens {
		if len(t) > 3 && strings.HasSuffix(t, "s") {
			t = t[:len(t)-1]
		}
		out[i] = t
	}
	return out
}

func subset(part, whole []string) bool {
	for _, t := range part {
		if !slices.Contains(whole, t) {
			return false
		}
	}
	return true
}

// search ranks catalogue entries of one kind against a name, across every
// locale the pinned packs ship. keep narrows the candidates to a scope the
// caller knows -- the subclasses of the character's class, one prompt's
// options -- and is what makes a loose match safe.
func (a *Agent) search(ctx context.Context, s *AgentSession, kind, query string, level *int, keep func(rules.Ref) bool, loose bool) ([]AgentCandidate, error) {
	locales, err := a.service.Source().Locales(ctx)
	if err != nil {
		return nil, err
	}
	best := map[string]AgentCandidate{}
	for _, locale := range locales {
		cat, err := catalog.LoadLocked(ctx, a.service.Source(), locale, s.Log.RulesLock())
		if err != nil {
			return nil, err
		}
		cat = domain.WithCustomCatalog(s.Log, cat)
		for _, c := range charuc.CatalogCandidates(cat, kind) {
			if level != nil && c.Level != nil && *level != *c.Level {
				continue
			}
			ref, _ := rules.ParseRef(c.Ref)
			if keep != nil && !keep(ref) {
				continue
			}
			score := max(nameScore(query, c.Name, loose), nameScore(query, ref.Slug.String(), false))
			if query == c.Ref || query == ref.Canonical() {
				score = 1
			}
			if score < .45 {
				continue
			}
			c.Score = score
			if old, ok := best[c.Ref]; !ok || old.Score < c.Score {
				best[c.Ref] = c
			}
		}
	}
	out := []AgentCandidate{}
	for _, c := range best {
		out = append(out, c)
	}
	slices.SortFunc(out, func(x, y AgentCandidate) int {
		if x.Score > y.Score {
			return -1
		}
		if x.Score < y.Score {
			return 1
		}
		return strings.Compare(x.Ref, y.Ref)
	})
	if len(out) > 8 {
		out = out[:8]
	}
	return out, nil
}

// candidatesError is a name the catalogue could not settle on its own. It
// carries what the name could have meant, so the model's next call can say.
type candidatesError struct {
	message    string
	Candidates []AgentCandidate
}

func (e *candidatesError) Error() string { return e.message }

// naturalKinds are the words a sheet (and a model) uses for a catalogue kind.
var naturalKinds = map[string]string{"cantrip": "spell", "weapon": "item", "armor": "item", "tool": "item", "equipment": "item", "gear": "item"}

// resolve turns what a tool was given -- a canonical ref, a bare slug or a
// printed name in any locale -- into exactly one catalogue entry of one of
// the kinds, or an error listing the entries it could have meant.
//
// An exact or reordered name wins outright. Anything weaker has to be the only
// plausible reading: two entries a hair apart are a question for the model,
// not a guess for the server.
func (a *Agent) resolve(ctx context.Context, s *AgentSession, cat *catalog.Catalog, kinds []string, text string, keep func(rules.Ref) bool, loose bool) (AgentCandidate, error) {
	text = strings.TrimSpace(text)
	entry := func(ref rules.Ref) (AgentCandidate, bool) {
		for _, c := range charuc.CatalogCandidates(cat, ref.Kind.String()) {
			if c.Ref == ref.Canonical() {
				c.Score = 1
				return c, true
			}
		}
		return AgentCandidate{}, false
	}
	if strings.Count(text, ":") == 2 {
		// A pack-qualified ref is exact or it is wrong. It never falls back to
		// a name, which is what keeps it from silently switching packs.
		ref, err := resolveAgentReference(cat, text)
		if err != nil {
			return AgentCandidate{}, err
		}
		if c, ok := entry(ref); ok && (len(kinds) == 0 || slices.Contains(kinds, ref.Kind.String())) && (keep == nil || keep(ref)) {
			return c, nil
		}
		return AgentCandidate{}, fmt.Errorf("reference %q is not one of the entries this accepts", text)
	}
	if prefix, rest, cut := strings.Cut(text, ":"); cut {
		kind := prefix
		if mapped, ok := naturalKinds[kind]; ok {
			kind = mapped
		}
		if _, isKind := rules.ParseRefKind(kind); isKind && (len(kinds) == 0 || slices.Contains(kinds, kind)) {
			kinds, text = []string{kind}, strings.TrimSpace(rest)
		}
	}
	if len(kinds) == 0 {
		return AgentCandidate{}, fmt.Errorf("say what kind %q is, as kind:Name", text)
	}
	found := []AgentCandidate{}
	for _, kind := range kinds {
		if mapped, ok := naturalKinds[kind]; ok {
			kind = mapped
		}
		// A slug, with or without its pack, names its entry outright.
		if ref, err := resolveAgentReference(cat, kind+":"+text); err == nil && (keep == nil || keep(ref)) {
			if c, ok := entry(ref); ok {
				return c, nil
			}
		}
		candidates, err := a.search(ctx, s, kind, text, nil, keep, loose)
		if err != nil {
			return AgentCandidate{}, err
		}
		found = append(found, candidates...)
	}
	slices.SortStableFunc(found, func(x, y AgentCandidate) int {
		// The pack carries a few entries twice, once with their mechanics and
		// once as a bare name. Between two spellings of one name the entry
		// that can actually be worn or swung is the one a character means.
		if x.Score >= .99 && y.Score >= .99 && typedItem(x) != typedItem(y) {
			if typedItem(x) {
				return -1
			}
			return 1
		}
		if x.Score > y.Score {
			return -1
		}
		if x.Score < y.Score {
			return 1
		}
		return strings.Compare(x.Ref, y.Ref)
	})
	brief := []AgentCandidate{}
	for _, c := range found[:min(len(found), 8)] {
		brief = append(brief, AgentCandidate{Ref: c.Ref, Name: c.Name, Score: c.Score, Level: c.Level})
	}
	what := strings.Join(kinds, "/")
	if len(found) == 0 || found[0].Score < .85 {
		return AgentCandidate{}, &candidatesError{message: fmt.Sprintf("no %s named %q in the selected rules; pick one of the candidates by ref, or keep it as custom content if none is it", what, text), Candidates: brief}
	}
	if found[0].Score < .99 && len(found) > 1 && found[1].Score > found[0].Score-.05 {
		return AgentCandidate{}, &candidatesError{message: fmt.Sprintf("%q could be several %s entries; say which by ref", text, what), Candidates: brief}
	}
	return found[0], nil
}

func typedItem(c AgentCandidate) bool {
	item, ok := c.Details.(catalog.Item)
	return ok && (item.Weapon != nil || item.Armor != nil || item.Gear != nil || item.Tool != nil || item.Vehicle != nil)
}

// Bare tool identifiers are local aliases within the selected rules lock.
// Explicit pack-qualified references never silently switch packs.
func resolveAgentReference(cat *catalog.Catalog, input string) (rules.Ref, error) {
	ref, ok := rules.ParseRef(input)
	if !ok {
		return rules.Ref{}, fmt.Errorf("invalid reference %q; use a canonical reference from search_catalog", input)
	}
	matches := []rules.Ref{}
	for _, candidate := range charuc.CatalogCandidates(cat, ref.Kind.String()) {
		available, _ := rules.ParseRef(candidate.Ref)
		if available == ref {
			return available, nil
		}
		local := available.Slug.String()
		if _, tail, qualified := strings.Cut(local, "/"); qualified {
			local = tail
		}
		if local == ref.Slug.String() {
			matches = append(matches, available)
		}
	}
	if strings.Count(input, ":") == 1 && !strings.Contains(ref.Slug.String(), "/") && len(matches) == 1 {
		return matches[0], nil
	}
	suggestions := []string{}
	for _, match := range matches {
		suggestions = append(suggestions, match.Canonical())
	}
	return rules.Ref{}, fmt.Errorf("reference %q is not available in the selected rules; use an exact search_catalog reference (local matches: %v)", input, suggestions)
}
