package agent

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

var spellNameNoise = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]|^\s*\S+['’]s\s+`)

// bareSpellName is a printed spell name without what a sheet adds to it: a
// parenthesised note ("(R)", "(ritual)") and the leading owner's name the SRD
// drops ("Tasha's", "Melf's").
func bareSpellName(printed string) string {
	return strings.TrimSpace(spellNameNoise.ReplaceAllString(printed, " "))
}

// assignSpells distributes the sheet's cantrips and spells over the build's
// spell prompts -- the class's per-level picks, a racial cantrip -- and keeps
// what is left as spells known outside the build's count.
//
// It is the same puzzle as the skills. An Arcane Trickster's Mage Hand is
// already granted, its other two cantrips fill its own prompt, and the high
// elf's cantrip is then a question the sheet does not answer; a model left to
// it tries Mage Hand in each prompt in turn. Prompts that must be answered are
// filled first, larger ones before smaller, so the one left open is the one
// worth asking the owner about.
func (a *Agent) assignSpells(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	names := s.spells
	if len(args.Spells) > 0 {
		names = args.Spells
	}
	if len(names) == 0 {
		// Not an error: a fighter's sheet lists none, and the procedure calls
		// this for every character.
		return map[string]any{"assigned": []any{}, "next": "The sheet lists no spells; nothing to assign. If it does list some, pass spells [names]."}, nil
	}
	// The assignment is done whole, so the spell picks made so far are taken
	// back first. See assignSkills for why they are removed outright.
	kept := slices.DeleteFunc(slices.Clone(s.Log.Events), func(e domain.Event) bool {
		return !e.Observed && len(e.Changes) == 0 && len(e.Choices) == 1 && strings.Contains(e.Choices[0].Prompt.String(), "/spell/")
	})
	log, err := domain.Rebuild(kept)
	if err != nil {
		return nil, err
	}
	if _, err := domain.Project(log, cat); err != nil {
		return nil, err
	}
	s.Log = log

	native := nativeLog(s.Log)
	state, err := domain.Project(native, cat)
	if err != nil {
		return nil, err
	}
	wanted, unknown := []rules.Slug{}, []map[string]any{}
	for _, printed := range names {
		// A sheet says "Tasha's Hideous Laughter" or "Detect Magic (R)" where
		// the rules say "Hideous Laughter" and "Detect Magic", so the name is
		// tried again without its owner and its notes. Nothing looser: every
		// spell there is is a candidate here, and "a name inside the printed
		// one" would turn Cause Fear into Fear.
		found, err := a.resolve(ctx, s, cat, []string{"spell"}, printed, nil, false)
		if bare := bareSpellName(printed); err != nil && bare != printed {
			found, err = a.resolve(ctx, s, cat, []string{"spell"}, bare, nil, false)
		}
		if err != nil {
			failure := map[string]any{}
			_ = json.Unmarshal(agentError(err), &failure)
			failure["name"] = printed
			unknown = append(unknown, failure)
			continue
		}
		if ref, _ := rules.ParseRef(found.Ref); !agentHolds(state, cat, ref) && !slices.Contains(wanted, ref.Slug) {
			wanted = append(wanted, ref.Slug)
		}
	}
	prompts, err := domain.Prompts(native, cat)
	if err != nil {
		return nil, err
	}
	offered := func(p domain.Prompt) map[rules.Slug]bool {
		offers := map[rules.Slug]bool{}
		for _, key := range rules.OptionKeys(cat.ResolveChoice(p.Choice).From) {
			if slices.Contains(wanted, key) && !slices.Contains(p.Held, key) {
				offers[key] = true
			}
		}
		return offers
	}
	slots := []*promptSlot{}
	for _, p := range prompts {
		if p.Choice.Kind == rules.ChooseSpell && p.Purpose != "custom" && !p.Optional {
			if offers := offered(p); len(offers) > 0 {
				slots = append(slots, &promptSlot{prompt: p, offers: offers})
			}
		}
	}
	// A class learns only so many spells of the highest level it can cast, and
	// that allowance is counted over all its prompts together. A sheet with
	// more -- a wizard's scribed scroll -- would have every prompt holding one
	// refused, so the surplus is kept out of the prompts and stays known.
	limits, err := domain.SpellRules(native, cat)
	if err != nil {
		return nil, err
	}
	for _, limit := range limits {
		if limit.MaxLevelCount == nil {
			continue
		}
		taken := 0
		for _, key := range wanted {
			if spell, ok := cat.Spells.Get(key); !ok || spell.Level != limit.MaxLevel {
				continue
			}
			counted := false
			for _, slot := range slots {
				if slot.prompt.Source != limit.Source || slot.prompt.Purpose != limit.Purpose || !slot.offers[key] {
					continue
				}
				if !counted {
					counted, taken = true, taken+1
				}
				if taken > *limit.MaxLevelCount {
					delete(slot.offers, key)
				}
			}
		}
	}
	slices.SortStableFunc(slots, func(x, y *promptSlot) int { return y.prompt.Choice.Choose - x.prompt.Choice.Choose })
	extra := matchPicks(slots, wanted)

	nameOf := catalogNames(cat)
	assigned, partial := []map[string]any{}, []map[string]any{}
	answer := func(p domain.Prompt, picks []rules.Slug) bool {
		names, keys := []string{}, []string{}
		for _, pick := range picks {
			names, keys = append(names, nameOf(rules.NewRef(rules.RefSpell, pick))), append(keys, pick.String())
		}
		entry := map[string]any{"prompt": localKey(cat, p.Choice.Prompt.String()), "picks": names}
		if len(picks) < p.Choice.Choose && !p.UpTo {
			entry["choose"] = p.Choice.Choose
			partial = append(partial, entry)
			return false
		}
		if _, err := a.answer(ctx, s, cat, agentArgs{Prompt: p.Choice.Prompt.String(), Picks: keys, Source: args.Source}); err != nil {
			_ = json.Unmarshal(agentError(err), &entry)
			partial = append(partial, entry)
			return false
		}
		assigned = append(assigned, entry)
		return true
	}
	for _, slot := range slots {
		// A prompt the sheet does not fill stays open, but the spells that
		// were headed for it are still the character's: they are kept as
		// known rather than dropped with the unanswered prompt.
		if !answer(slot.prompt, slot.picks) {
			extra = append(extra, slot.picks...)
		}
	}
	// A prompt that must be answered and that none of the sheet's remaining
	// spells fits is a question the sheet leaves open.
	for _, p := range prompts {
		if p.Choice.Kind == rules.ChooseSpell && p.Purpose != "custom" && !p.Optional && !slices.ContainsFunc(slots, func(slot *promptSlot) bool { return slot.prompt.Choice.Prompt == p.Choice.Prompt }) {
			partial = append(partial, map[string]any{"prompt": localKey(cat, p.Choice.Prompt.String()), "picks": []string{}, "choose": p.Choice.Choose})
		}
	}
	// Preparation is chosen from what is now known, and takes the sheet's
	// spells again rather than what is left of them.
	marked, unprepared := []rules.Slug{}, []string{}
	for _, printed := range s.prepared {
		if found, err := a.resolve(ctx, s, cat, []string{"spell"}, printed, nil, true); err == nil {
			ref, _ := rules.ParseRef(found.Ref)
			marked = append(marked, ref.Slug)
		}
	}
	if prompts, err = domain.Prompts(nativeLog(s.Log), cat); err != nil {
		return nil, err
	}
	for _, p := range prompts {
		if p.Choice.Kind != rules.ChooseSpell || p.Purpose == "custom" || !p.Optional || !p.UpTo {
			continue
		}
		picks := []rules.Slug{}
		for key := range offered(p) {
			picks = append(picks, key)
		}
		// The ones the sheet marks as prepared first; a sheet that marks none
		// gets the first few by name, which is a guess the owner can change.
		slices.SortFunc(picks, func(x, y rules.Slug) int {
			if px, py := slices.Contains(marked, x), slices.Contains(marked, y); px != py {
				if px {
					return -1
				}
				return 1
			}
			return strings.Compare(x.String(), y.String())
		})
		// Everything this prompt offers is already the character's to prepare
		// -- a cleric's whole list, a wizard's book -- so what does not fit is
		// simply not prepared, not an extra spell known from nowhere.
		extra = slices.DeleteFunc(extra, func(key rules.Slug) bool { return slices.Contains(picks, key) })
		for _, key := range picks[min(len(picks), p.Choice.Choose):] {
			unprepared = append(unprepared, nameOf(rules.NewRef(rules.RefSpell, key)))
		}
		if picks = picks[:min(len(picks), p.Choice.Choose)]; len(picks) > 0 {
			answer(p, picks)
		}
	}
	beyond := []string{}
	if len(extra) > 0 {
		refs := []string{}
		for _, key := range extra {
			refs = append(refs, rules.NewRef(rules.RefSpell, key).Canonical())
			beyond = append(beyond, nameOf(rules.NewRef(rules.RefSpell, key)))
		}
		if _, err := a.knowSpells(ctx, s, cat, refs); err != nil {
			return nil, err
		}
	}
	open, err := a.openPrompts(s, cat)
	if err != nil {
		return nil, err
	}
	return map[string]any{"assigned": assigned, "partial": partial, "beyond": beyond, "unprepared": unprepared, "unknown": unknown, "open": open, "next": "partial prompts are ones the sheet's spells do not fill: the build grants a spell the sheet does not list, so ask_user which, offering to leave it open; the picks shown there are kept as known meanwhile. beyond are spells the sheet lists past what the build's prompts take; they are kept as known. unprepared are spells on the class's list past its preparation limit: nothing to do. unknown are names the rules do not have or cannot tell apart: pass the right candidate's ref to assign_spells, or keep it with upsert_custom_option kind spell if none is it."}, nil
}
