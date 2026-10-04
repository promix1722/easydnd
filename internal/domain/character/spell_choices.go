package character

import (
	"fmt"
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// SpellSource preserves independent learning and preparation limits. The same
// spell may legitimately be learned from two classes with different abilities.
type SpellSource struct {
	Source                                                 rules.Ref
	Class                                                  rules.Slug
	Ability                                                rules.Ability
	Cantrips, Known, Spellbook, Prepared, Arcanum, Mastery []rules.Slug
	PreparationLimit                                       int
}

type spellBuilder struct {
	state   State
	cat     *catalog.Catalog
	answers answers
	all     bool
	prompts []Prompt
	sources []SpellSource
}

func spellChoices(state State, cat *catalog.Catalog, answers answers, all bool) ([]Prompt, []SpellSource) {
	b := spellBuilder{state: state, cat: cat, answers: answers, all: all}
	for _, taken := range state.Identity.Classes {
		b.class(taken)
	}
	for _, slug := range state.Traits {
		trait, ok := cat.Traits.Get(slug)
		if !ok {
			continue
		}
		source := SpellSource{Source: rules.NewRef(rules.RefTrait, slug)}
		if trait.Specific != nil && trait.Specific.SpellOptions != nil {
			for _, spell := range answers.slugs(trait.Specific.SpellOptions) {
				if def, ok := cat.Spells.Get(spell); ok && def.Level == 0 {
					source.Cantrips = appendUnique(source.Cantrips, spell)
				}
			}
		}
		for _, benefit := range cat.Mechanics.SpellBenefits {
			if benefit.Owner == source.Source && ownerLevel(state, cat, benefit.Owner) >= benefit.Level {
				source.Ability = benefit.Ability
				b.benefit(&source, benefit, state.Identity.Level(), 9)
			}
		}
		if len(source.Cantrips)+len(source.Known)+len(source.Prepared) > 0 {
			b.sources = append(b.sources, source)
		}
	}
	b.custom()
	return b.prompts, b.sources
}

func appendUnique(into []rules.Slug, values ...rules.Slug) []rules.Slug {
	for _, value := range values {
		if !slices.Contains(into, value) {
			into = append(into, value)
		}
	}
	return into
}

func (b *spellBuilder) pool(class rules.Slug, minLevel, maxLevel int, extra []rules.Slug) []rules.Slug {
	var out []rules.Slug
	for _, spell := range b.cat.Spells.All() {
		if spell.Level < minLevel || spell.Level > maxLevel {
			continue
		}
		if class.IsZero() || slices.Contains(spell.Classes, class) || slices.Contains(extra, spell.Slug) {
			out = append(out, spell.Slug)
		}
	}
	return out
}

func spellPrompt(class rules.Slug, purpose string, level int) rules.Slug {
	return rules.Slug(fmt.Sprintf("%s/spell/%s/%d", class, purpose, level))
}

func (b *spellBuilder) pick(source rules.Ref, class rules.Slug, id rules.Slug, purpose string, level, count int, pool, held []rules.Slug, optional, upTo bool) []rules.Slug {
	if count <= 0 {
		return nil
	}
	p := Prompt{Choice: rules.Choice{Prompt: id, Kind: rules.ChooseSpell, Choose: count, From: refOptions(rules.RefSpell, pool)},
		Source: source, Group: GroupClass, Level: level, Purpose: purpose, UpTo: upTo, Optional: optional,
		Event: PromptEvent{Type: EventLevel, Ref: rules.NewRef(rules.RefClass, class), Level: level}, Held: slices.Clone(held)}
	if class.IsZero() {
		p.Group = GroupRace
		p.Event = PromptEvent{Type: EventRace, Ref: rules.NewRef(rules.RefRace, b.state.Identity.Race)}
	}
	picks := b.answers.picks(id)
	valid := len(picks) == count || (upTo && len(picks) > 0 && len(picks) <= count)
	var selected []rules.Slug
	for _, slug := range picks {
		if !slices.Contains(pool, slug) || slices.Contains(held, slug) || slices.Contains(selected, slug) {
			valid = false
			continue
		}
		selected = append(selected, slug)
	}
	if b.all || !valid {
		b.prompts = append(b.prompts, p)
	}
	// Invalid persisted answers grant nothing and reopen the question.
	if !valid {
		return nil
	}
	return selected
}

func maxSpellLevel(cat *catalog.Catalog, class rules.Slug, level int) int {
	row, _ := cat.ClassLevel(class, level)
	for n := MaxSpellLevel; n > 0; n-- {
		if row.SpellSlots[n] > 0 {
			return n
		}
	}
	return 0
}

func (b *spellBuilder) class(taken ClassLevel) {
	owner, profile, ok := castingProfile(b.cat, taken)
	if !ok || profile.Selection == "" || taken.Level < profile.StartsAt {
		return
	}
	ability := castingAbility(b.cat, taken.Class, profile)
	if ability == "" {
		return
	}
	kind := rules.RefClass
	if owner != taken.Class {
		kind = rules.RefSubclass
	}
	list := profile.List
	if list == "" {
		list = taken.Class
	}
	source := SpellSource{Source: rules.NewRef(kind, owner), Class: taken.Class, Ability: ability}
	var automatic, extra []rules.Slug
	currentMax := maxSpellLevel(b.cat, owner, taken.Level)
	subclass, _ := b.cat.Subclasses.Get(taken.Subclass)
	for _, grant := range subclass.Spells {
		if grant.Level <= taken.Level && profile.ExpandedSubclass {
			extra = appendUnique(extra, grant.Spell)
		}
	}
	for level := 1; level <= taken.Level; level++ {
		maxLevel := maxSpellLevel(b.cat, owner, level)
		// Automatic subclass spells live in explicit benefits, so a flattened
		// terrain table cannot accidentally grant every Circle of the Land spell.
		knownBonus := 0
		for _, benefit := range b.cat.Mechanics.SpellBenefits {
			if benefit.Class != taken.Class || benefit.Level != level || ownerLevel(b.state, b.cat, benefit.Owner) < level {
				continue
			}
			if len(benefit.Spells) > 0 && benefit.Mode == "prepared" {
				automatic = appendUnique(automatic, benefit.Spells...)
				continue
			}
			if benefit.From != "book" {
				b.benefit(&source, benefit, level, maxLevel)
			}
			if benefit.CountsKnown {
				knownBonus += benefit.Count
			}
		}
		if level < profile.StartsAt {
			continue
		}
		row, _ := b.cat.ClassLevel(owner, level)
		previous, _ := b.cat.ClassLevel(owner, level-1)
		cantrips := b.pick(source.Source, taken.Class, spellPrompt(owner, "cantrip", level), "cantrip", level, row.CantripsKnown-previous.CantripsKnown,
			b.pool(list, 0, 0, nil), source.Cantrips, false, false)
		source.Cantrips = appendUnique(source.Cantrips, cantrips...)
		count := row.SpellsKnown - previous.SpellsKnown - knownBonus
		mode := "known"
		held := source.Known
		if profile.Selection == "spellbook" {
			mode = "spellbook"
			count = profile.BookPerLevel
			if level == profile.StartsAt {
				count = profile.BookStart
			}
			held = source.Spellbook
		}
		learned := b.pick(source.Source, taken.Class, spellPrompt(owner, mode, level), mode, level, count, b.pool(list, 1, currentMax, extra), held, false, false)
		if mode == "spellbook" {
			source.Spellbook = appendUnique(source.Spellbook, learned...)
		} else {
			source.Known = appendUnique(source.Known, learned...)
		}
		for _, benefit := range b.cat.Mechanics.SpellBenefits {
			if benefit.Class == taken.Class && benefit.Level == level && benefit.From == "book" && ownerLevel(b.state, b.cat, benefit.Owner) >= level {
				b.benefit(&source, benefit, level, maxLevel)
			}
		}
		if profile.ReplaceKnown && level > profile.StartsAt && (b.all || len(b.answers.picks(spellPrompt(owner, "forget", level))) > 0) {
			// Read old swaps for compatibility, but never offer a new replacement workflow.
			promptStart := len(b.prompts)
			// One optional replacement per gained class level. Both answers live in
			// the ordinary log, so replay applies the swap before later acquisitions.
			before := slices.Clone(source.Known)
			replaceable := slices.Clone(before)
			for _, benefit := range b.cat.Mechanics.SpellBenefits {
				if benefit.Class == taken.Class && len(benefit.Spells) > 0 {
					replaceable = slices.DeleteFunc(replaceable, func(s rules.Slug) bool { return slices.Contains(benefit.Spells, s) })
				}
			}
			forgotten := b.pick(source.Source, taken.Class, spellPrompt(owner, "forget", level), "forget", level, 1, replaceable, nil, true, false)
			if len(forgotten) > 0 {
				replacement := b.pick(source.Source, taken.Class, spellPrompt(owner, "replace", level), "replace", level, 1, b.pool(list, 1, maxLevel, extra), before, false, false)
				if len(replacement) > 0 {
					source.Known = slices.DeleteFunc(source.Known, func(s rules.Slug) bool { return s == forgotten[0] })
					source.Known = appendUnique(source.Known, replacement...)
				}
			}
			if !b.all {
				b.prompts = b.prompts[:promptStart]
			}
		}
	}
	if profile.Selection == "known" {
		source.Prepared = appendUnique(source.Prepared, source.Known...)
	}
	automatic = appendUnique(automatic, source.Prepared...)
	if profile.PrepareDivisor > 0 {
		source.PreparationLimit = max(1, taken.Level/profile.PrepareDivisor+b.state.Abilities.Modifier(source.Ability))
		pool := b.pool(list, 1, maxSpellLevel(b.cat, owner, taken.Level), nil)
		if profile.Selection == "spellbook" {
			pool = slices.Clone(source.Spellbook)
		}
		pool = slices.DeleteFunc(pool, func(s rules.Slug) bool { return slices.Contains(automatic, s) })
		prepared := b.pick(source.Source, taken.Class, spellPrompt(owner, "prepared", taken.Level), "prepared", taken.Level, source.PreparationLimit, pool, nil, true, true)
		source.Prepared = appendUnique(source.Prepared, prepared...)
	}
	source.Prepared = appendUnique(source.Prepared, automatic...)
	b.sources = append(b.sources, source)
}

func (b *spellBuilder) benefit(source *SpellSource, benefit catalog.SpellBenefit, level, maxLevel int) {
	pool := slices.Clone(benefit.Spells)
	picks := pool
	if benefit.Count > 0 {
		minLevel, upper := benefit.SpellLevel, benefit.SpellLevel
		if minLevel < 0 {
			minLevel = 0
			upper = maxLevel
		}
		listClass := benefit.Class
		if benefit.From == "any" {
			listClass = ""
		}
		pool = b.pool(listClass, minLevel, upper, nil)
		if benefit.From == "book" {
			pool = slices.DeleteFunc(pool, func(s rules.Slug) bool { return !slices.Contains(source.Spellbook, s) })
		}
		held := append(slices.Clone(source.Known), source.Cantrips...)
		if benefit.Mode == "mastery" {
			held = source.Mastery
		}
		if benefit.Mode == "prepared" {
			held = nil
		}
		picks = b.pick(benefit.Owner, benefit.Class, rules.Slug(fmt.Sprintf("%s/spell/%s", benefit.ID, benefit.Mode)), benefit.Mode, level, benefit.Count, pool, held, false, false)
	}
	for _, slug := range picks {
		spell, ok := b.cat.Spells.Get(slug)
		if !ok {
			continue
		}
		switch benefit.Mode {
		case "arcanum":
			source.Arcanum = appendUnique(source.Arcanum, slug)
		case "mastery":
			source.Mastery = appendUnique(source.Mastery, slug)
		case "prepared":
			source.Prepared = appendUnique(source.Prepared, slug)
		case "spellbook":
			source.Spellbook = appendUnique(source.Spellbook, slug)
		default:
			if spell.Level == 0 {
				source.Cantrips = appendUnique(source.Cantrips, slug)
			} else {
				source.Known = appendUnique(source.Known, slug)
			}
		}
	}
}

func (p *projector) applySpells() {
	_, sources := spellChoices(p.state, p.cat, p.answers, false)
	p.state.Spells.Sources = sources
	for _, source := range sources {
		p.state.Spells.Cantrips = appendUnique(p.state.Spells.Cantrips, source.Cantrips...)
		p.state.Spells.Known = appendUnique(p.state.Spells.Known, source.Known...)
		p.state.Spells.Known = appendUnique(p.state.Spells.Known, source.Spellbook...)
		p.state.Spells.Prepared = appendUnique(p.state.Spells.Prepared, source.Prepared...)
	}
}
