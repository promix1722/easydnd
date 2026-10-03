package character

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"unicode"

	"github.com/agnivade/levenshtein"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// Candidates describe identity only. A separate choice validation determines
// whether this character may select the identified entry for a given purpose.
type AgentCandidate struct {
	Ref     string  `json:"ref"`
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
	Level   *int    `json:"level,omitempty"`
	Details any     `json:"details,omitempty"`
}

func normalizedName(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }), " ")
}
func nameScore(query, name string) float64 {
	q, n := normalizedName(query), normalizedName(name)
	if q == "" || n == "" {
		return 0
	}
	if q == n {
		return 1
	}
	// These words change spell identity rather than spelling. Never normalize
	// Mass/Greater/Lesser away or rank their absence as a typo.
	for _, word := range []string{"mass", "greater", "lesser", "масс", "массовое", "высшее", "малое"} {
		if slices.Contains(strings.Fields(q), word) != slices.Contains(strings.Fields(n), word) {
			return 0
		}
	}
	score := 1 - float64(levenshtein.ComputeDistance(q, n))/float64(max(len([]rune(q)), len([]rune(n))))
	qt, nt := strings.Fields(q), strings.Fields(n)
	overlap := 0
	for _, t := range qt {
		if slices.Contains(nt, t) {
			overlap++
		}
	}
	token := float64(2*overlap) / float64(len(qt)+len(nt))
	return max(score, token*.95)
}
func catalogCandidates(cat *catalog.Catalog, kind string) []AgentCandidate {
	fields := map[string]string{"race": "Races", "subrace": "Subraces", "class": "Classes", "subclass": "Subclasses", "background": "Backgrounds", "feat": "Feats", "feature": "Features", "trait": "Traits", "spell": "Spells", "item": "Items", "magic-item": "MagicItems", "language": "Languages", "skill": "Skills", "proficiency": "Proficiencies"}
	field, ok := fields[kind]
	if !ok {
		return nil
	}
	rk, ok := rules.ParseRefKind(kind)
	if !ok {
		return nil
	}
	collection := reflect.ValueOf(cat).Elem().FieldByName(field)
	if !collection.IsValid() {
		return nil
	}
	entries := collection.MethodByName("All").Call(nil)[0]
	out := []AgentCandidate{}
	for i := 0; i < entries.Len(); i++ {
		v := entries.Index(i)
		entry := v.FieldByName("Entry").Interface().(catalog.Entry)
		c := AgentCandidate{Ref: rules.NewRef(rk, entry.Slug).Canonical(), Name: entry.Name, Details: v.Interface()}
		if l := v.FieldByName("Level"); l.IsValid() && l.Kind() == reflect.Int {
			n := int(l.Int())
			c.Level = &n
		}
		out = append(out, c)
	}
	return out
}
func (a *Agent) search(ctx context.Context, s *AgentSession, kind, query string, level *int) ([]AgentCandidate, error) {
	locales, err := a.service.catalog.Locales(ctx)
	if err != nil {
		return nil, err
	}
	best := map[string]AgentCandidate{}
	for _, locale := range locales {
		cat, err := catalog.LoadLocked(ctx, a.service.catalog, locale, s.Log.RulesLock())
		if err != nil {
			return nil, err
		}
		for _, c := range catalogCandidates(cat, kind) {
			if level != nil && c.Level != nil && *level != *c.Level {
				continue
			}
			score := nameScore(query, c.Name)
			ref, _ := rules.ParseRef(c.Ref)
			score = max(score, nameScore(query, ref.Slug.String()))
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
