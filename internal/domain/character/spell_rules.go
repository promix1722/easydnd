package character

import (
	"slices"

	"github.com/promix1722/easydnd/internal/types"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// SpellRule explains acquisition allowances, not spell slots used for casting.
// It includes answered choices so the explanation survives saving the wizard.
type SpellRule struct {
	ID                                    rules.Slug
	Source                                rules.Ref
	Class                                 rules.Slug
	ClassLevel, Count, MinLevel, MaxLevel int
	MaxLevelCount                         *int
	Purpose                               string
	Optional                              bool
	ListClasses                           []rules.Slug
	Automatic                             []rules.Slug
}

func SpellRules(log Log, cat *catalog.Catalog) ([]SpellRule, error) {
	state, err := Project(log, cat)
	if err != nil {
		return nil, err
	}
	b := promptBuilder{cat: cat, state: state, answers: foldAnswers(log), scored: scoresWereSet(log), all: true}
	var out []SpellRule
	prompts := b.build()
	for _, p := range prompts {
		if p.Choice.Kind != rules.ChooseSpell || p.Purpose == "custom" || p.Purpose == "replace" || p.Purpose == "forget" {
			continue
		}
		r := SpellRule{ID: p.Choice.Prompt, Source: p.Source, ClassLevel: p.Level, Count: p.Choice.Choose, Purpose: p.Purpose, Optional: p.Optional, MinLevel: 10}
		if p.Event.Ref.Kind == rules.RefClass {
			r.Class = p.Event.Ref.Slug
			for _, taken := range state.Identity.Classes {
				if taken.Class == r.Class {
					r.ClassLevel = taken.Level
				}
			}
		}
		keys := rules.OptionKeys(p.Choice.From)
		{
			first := true
			for _, key := range keys {
				spell, ok := cat.Spells.Get(key)
				if !ok {
					continue
				}
				r.MinLevel = min(r.MinLevel, spell.Level)
				r.MaxLevel = max(r.MaxLevel, spell.Level)
				if first {
					r.ListClasses = slices.Clone(spell.Classes)
					first = false
				} else {
					r.ListClasses = slices.DeleteFunc(r.ListClasses, func(class rules.Slug) bool { return !slices.Contains(spell.Classes, class) })
				}
			}
		}
		if r.MinLevel == 10 {
			r.MinLevel = 0
		}
		if r.Source.Kind == rules.RefClass || r.Source.Kind == rules.RefSubclass {
			list := r.Class
			if profile, ok := cat.Mechanics.Casting[r.Source.Slug]; ok && profile.List != "" {
				list = profile.List
			}
			r.ListClasses = []rules.Slug{list}
		}
		merged := false
		for i := range out {
			if out[i].Source == r.Source && out[i].Purpose == r.Purpose && len(out[i].Automatic) == 0 {
				out[i].Count += r.Count
				out[i].MinLevel = min(out[i].MinLevel, r.MinLevel)
				out[i].MaxLevel = max(out[i].MaxLevel, r.MaxLevel)
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, r)
		}
	}
	for _, benefit := range cat.Mechanics.SpellBenefits {
		if len(benefit.Spells) == 0 || ownerLevel(state, cat, benefit.Owner) < benefit.Level {
			continue
		}
		r := SpellRule{ID: benefit.ID, Source: benefit.Owner, Class: benefit.Class, ClassLevel: benefit.Level, Purpose: benefit.Mode, Count: len(benefit.Spells), MinLevel: 10, Automatic: benefit.Spells}
		for _, slug := range benefit.Spells {
			if spell, ok := cat.Spells.Get(slug); ok {
				r.MinLevel = min(r.MinLevel, spell.Level)
				r.MaxLevel = max(r.MaxLevel, spell.Level)
			}
		}
		if r.MinLevel != 10 {
			out = append(out, r)
		}
	}
	for i := range out {
		r := &out[i]
		profile, ok := cat.Mechanics.Casting[r.Source.Slug]
		if !ok || (r.Source.Kind != rules.RefClass && r.Source.Kind != rules.RefSubclass) || (r.Purpose != "known" && r.Purpose != "spellbook") || (profile.Selection != "known" && profile.Selection != "spellbook") {
			continue
		}
		base, changes := 0, 0
		for _, prompt := range prompts {
			if prompt.Source == r.Source && prompt.Purpose == r.Purpose && maxSpellLevel(cat, r.Source.Slug, prompt.Level) == r.MaxLevel {
				base += prompt.Choice.Choose
			}
		}
		for level := profile.StartsAt + 1; level <= r.ClassLevel; level++ {
			if profile.ReplaceKnown && maxSpellLevel(cat, r.Source.Slug, level) == r.MaxLevel {
				changes++
			}
		}
		limit := min(r.Count, base+changes)
		r.MaxLevelCount = &limit
	}
	for _, extra := range []struct {
		mode         string
		count, level int
	}{
		{"cantrip", state.Spells.ExtraCantrips, 0}, {"known", state.Spells.ExtraKnown, 9},
	} {
		if extra.count > 0 {
			out = append(out, SpellRule{ID: rules.Slug("custom/limit/" + extra.mode), Source: rules.NewRef(rules.RefRule, "custom-spells"), Purpose: "custom-limit", Count: extra.count, MinLevel: extra.level, MaxLevel: extra.level, Optional: true})
		}
	}
	return out, nil
}

// Check only the highest learnable level, across all historical answers together.
// Explicit custom picks have a different source and do not use the class quota.
func ValidateSpellLimits(log Log, cat *catalog.Catalog) error {
	limits, err := SpellRules(log, cat)
	if err != nil {
		return err
	}
	state, err := Project(log, cat)
	if err != nil {
		return err
	}
	resolved, err := ResolvedSelections(log, cat)
	if err != nil {
		return err
	}
	for _, limit := range limits {
		if limit.MaxLevelCount == nil {
			continue
		}
		ordinary := map[rules.Slug]bool{}
		for _, choice := range resolved {
			if choice.Source != limit.Source || (choice.Purpose != limit.Purpose && choice.Purpose != "replace") {
				continue
			}
			for _, option := range choice.Options {
				if ref, ok := option.(rules.RefOption); ok {
					ordinary[ref.Ref.Slug] = true
				}
			}
		}
		count := 0
		for _, source := range state.Spells.Sources {
			if source.Source != limit.Source {
				continue
			}
			spells := source.Known
			if limit.Purpose == "spellbook" {
				spells = source.Spellbook
			}
			for _, slug := range spells {
				if spell, ok := cat.Spells.Get(slug); ok && ordinary[slug] && spell.Level == limit.MaxLevel {
					count++
				}
			}
		}
		if count > *limit.MaxLevelCount {
			return types.NewFieldValidationError("spell level allowance exceeded", types.FieldError{Field: "choices", Rule: "max-spell-level-count", Reason: "field.spells.maxLevelCount"})
		}
	}
	return nil
}

// Custom picks have an explicit source and never silently consume class/race
// allowances. The ordinary option validator still rejects unknowns/duplicates.
func (b *spellBuilder) custom() {
	source := SpellSource{Source: rules.NewRef(rules.RefRule, "custom-spells")}
	for _, mode := range []string{"cantrip", "known"} {
		low, high := 1, 9
		if mode == "cantrip" {
			low, high = 0, 0
		}
		pool := b.pool("", low, high, nil)
		if len(pool) == 0 {
			continue
		}
		id := rules.Slug("custom/spell/" + mode)
		prompt := Prompt{Choice: rules.Choice{Prompt: id, Kind: rules.ChooseSpell, Choose: len(pool), From: refOptions(rules.RefSpell, pool)}, Source: source.Source, Group: GroupClass, Purpose: "custom", Optional: true, UpTo: true, Event: PromptEvent{Type: EventChange}}
		picks := b.answers.picks(id)
		valid := len(picks) > 0
		seen := map[rules.Slug]bool{}
		for _, slug := range picks {
			if !slices.Contains(pool, slug) || seen[slug] {
				valid = false
			}
			seen[slug] = true
		}
		if b.all || !valid {
			b.prompts = append(b.prompts, prompt)
		}
		if !valid {
			continue
		}
		if mode == "cantrip" {
			source.Cantrips = slices.Clone(picks)
		} else {
			source.Known = slices.Clone(picks)
			source.Prepared = slices.Clone(picks)
		}
	}
	if len(source.Known)+len(source.Cantrips) > 0 {
		b.sources = append(b.sources, source)
	}
}
