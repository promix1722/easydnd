package agent

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// customOption keeps content the selected rules do not have -- and refuses to
// make a custom copy of content they do.
//
// A sheet prints everything a character has, most of which the build already
// grants. Each of those arriving here would otherwise become a custom entry
// shadowing the real one, which is how an imported rogue ended up with a
// custom Sneak Attack beside the catalogue's.
func (a *Agent) customOption(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	state, err := domain.Project(s.Log, cat)
	if err != nil {
		return nil, err
	}
	refKind, _ := rules.ParseRefKind(args.Kind)
	_, structural := structuralEvents[refKind]
	selected := args.Selected == nil || *args.Selected
	if lookup := lookupKinds(args.Kind); args.Kind != "note" && charuc.CatalogCandidates(cat, lookup[0]) != nil {
		var found AgentCandidate
		known := false
		if args.Ref != "" {
			// A ref the model wrote names something or it is a mistake. Dropping
			// it in silence is how a known spell became a nameless custom one.
			if found, err = a.resolve(ctx, s, cat, lookup, args.Ref, nil, false); err != nil {
				return nil, err
			}
			known = true
		} else if strings.TrimSpace(args.Name) != "" {
			found, known = a.identify(ctx, s, cat, state, args.Kind, args.Name)
		}
		if known && selected {
			ref, _ := rules.ParseRef(found.Ref)
			native := map[string]any{"native": true, "ref": found.Ref, "name": found.Name}
			done := func(reason string) (any, error) {
				s.expected = slices.DeleteFunc(s.expected, func(key string) bool { return key == "custom:"+args.ID })
				native["reason"] = reason
				return native, nil
			}
			switch {
			case agentHolds(state, cat, ref):
				return done("already part of the build; nothing to add")
			case structural:
				fact := agentArgs{Ref: found.Ref, Source: args.Source}
				if args.Kind == "class" {
					fact.Level = args.Level
				}
				result, err := a.importFact(ctx, s, cat, fact)
				if err != nil {
					return nil, err
				}
				a.recordProgress(ctx, s, "resolve_import_facts", fact, raw(result))
				return done("the selected rules have it; imported as the catalogue entry")
			case ref.Kind == rules.RefSpell:
				if _, err := a.knowSpells(ctx, s, cat, []string{found.Ref}); err != nil {
					return nil, err
				}
				return done("the selected rules have it; added as a spell known outside the class's count")
			case ref.Kind == rules.RefItem || ref.Kind == rules.RefMagicItem:
				fact, _, err := a.inventoryFact(ctx, s, cat, agentArgs{Ref: found.Ref, Count: args.Count, Placement: args.Placement, Source: args.Source})
				if err == nil {
					_, err = a.importFact(ctx, s, cat, fact)
				}
				if err != nil {
					return nil, err
				}
				a.recordProgress(ctx, s, "set_inventory", agentArgs{Ref: found.Ref, Name: found.Name, Value: fact.Value}, nil)
				return done("the selected rules have it; added to the inventory as the catalogue item")
			}
		}
		// A catalogue entry the character has although nothing offers it -- a
		// spell beyond the class's count, a feature from a level not reached --
		// keeps its identity and is recorded as this character's exception.
		args.Ref = ""
		if known {
			args.Ref = found.Ref
			if ref, _ := rules.ParseRef(found.Ref); args.Kind == "spell" && ref.Kind == rules.RefSpell {
				if spell, ok := cat.Spells.Get(ref.Slug); ok && spell.Level == 0 {
					args.Kind = "cantrip"
				}
			}
		}
	}

	// A custom entry is something the rules lack that a character is built
	// from: a class, a race, a background, a spell, an item. A feature, a
	// trait, a feat or a note the catalogue does not know is not that -- it is
	// what a model writes to quiet a checklist or to record that a field was
	// left blank, and the owner finds it later as junk on the sheet.
	if slices.Contains([]string{"feature", "trait", "feat", "note"}, args.Kind) && args.Ref == "" {
		s.expected = slices.DeleteFunc(s.expected, func(key string) bool { return key == "custom:"+args.ID })
		return map[string]any{"kept": false, "reason": "Not kept: custom entries are only for a class, subclass, race, subrace, background, cantrip, spell or item the selected rules lack. A feature or trait the build grants needs nothing; anything else worth telling the user goes in your message, not on the character."}, nil
	}
	// Optional schema values may arrive as zero/empty from a model. They are
	// absence of evidence, never asserted mechanics for unrelated kinds.
	if args.Kind != "class" || args.HitDie != nil && *args.HitDie == 0 {
		args.HitDie = nil
	}
	if args.Kind != "race" {
		args.Speed = nil
	}
	if args.Kind != "class" && args.Kind != "spell" || args.Level != nil && *args.Level == 0 {
		args.Level = nil
	}
	if args.Kind != "class" && args.Kind != "subclass" && args.Kind != "spell" && args.Kind != "cantrip" {
		args.Ability = ""
	}
	if args.Ability == "none" || args.Ability == "unknown" {
		args.Ability = ""
	}
	if code := abilityNames[strings.ToLower(args.Ability)]; code != "" {
		args.Ability = code
	}
	if args.Kind != "item" {
		args.Count = 0
		args.Placement = ""
	}
	if args.Kind != "spell" && args.Kind != "cantrip" {
		args.Mode = ""
	}

	if args.Definition != "" {
		compiler, ok := a.service.Source().(interface {
			CompilePrivate(context.Context, pack.Lock, string, []byte, rules.Locale) (*catalog.Catalog, error)
		})
		if !ok {
			return nil, fmt.Errorf("private pack compiler unavailable")
		}
		compiled, err := compiler.CompilePrivate(ctx, s.Log.RulesLock(), s.ID, []byte(args.Definition), s.Locale)
		if err != nil && (args.ID == "" || args.Kind == "" || strings.TrimSpace(args.Name) == "") {
			return nil, err
		}
		if err == nil {
			log := s.Log.Clone()
			log.Events[0].RulesLock = compiled.Lock.Clone()
			if err := charuc.ValidateImported(compiled, log); err != nil {
				return nil, err
			}
			s.Log = log
			return map[string]any{"automated": true, "lock": compiled.Lock, "namespace": "import-" + s.ID}, nil
		}
	}

	if args.ID == "" || args.Kind == "" || strings.TrimSpace(args.Name) == "" || len(args.Description) > 16000 {
		return nil, fmt.Errorf("id, kind, name and bounded description required")
	}
	if args.Kind == "class" && args.Level == nil && args.Ref != "" {
		ref, _ := rules.ParseRef(args.Ref)
		for _, taken := range state.Identity.Classes {
			if taken.Class == ref.Slug {
				level := taken.Level
				args.Level = &level
			}
		}
	}
	if args.Parent != "" {
		kind := "class"
		if args.Kind == "subrace" {
			kind = "race"
		}
		// Loose, because a parent is one of a handful of entries and a model
		// writes it as the sheet does: "Sorcerer", not a slug.
		parent, err := a.resolve(ctx, s, cat, []string{kind}, args.Parent, nil, true)
		if err != nil {
			return nil, err
		}
		ref, _ := rules.ParseRef(parent.Ref)
		args.Parent = ref.Slug.String()
	}
	customLog := s.Log
	if len(s.Files) > 0 && args.Kind == "item" && (args.Selected == nil || *args.Selected) {
		customLog = clearImportedInventory(customLog, args.Placement)
	}
	option := domain.CustomOption{ID: args.ID, Kind: args.Kind, Name: args.Name, Description: args.Description, Source: args.Source, Parent: args.Parent, Ability: args.Ability, Mode: args.Mode, Placement: args.Placement, Level: args.Level, HitDie: args.HitDie, Speed: args.Speed, Count: args.Count, Selected: selected, Reference: args.Ref}
	log, err := charuc.UpsertCustom(customLog, cat, option)
	if err != nil {
		return nil, err
	}
	s.Log = log
	entry := AgentManual{ID: args.ID, Kind: args.Kind, Name: args.Name, Description: args.Description, Source: args.Source}
	found := false
	for i, v := range s.Manual {
		if v.ID == entry.ID {
			s.Manual[i] = entry
			found = true
			break
		}
	}
	if !found {
		s.Manual = append(s.Manual, entry)
	}
	return map[string]any{"manual": entry, "automated": false}, nil
}

// lookupKinds are the collections a sheet's word for a kind can mean. A sheet
// does not distinguish a class feature from a racial trait, or a cantrip from
// any other spell.
func lookupKinds(kind string) []string {
	if kinds := map[string][]string{"cantrip": {"spell"}, "feature": {"feature", "trait"}, "trait": {"trait", "feature"}, "item": {"item", "magic-item"}}[kind]; kinds != nil {
		return kinds
	}
	if mapped, ok := naturalKinds[kind]; ok {
		return lookupKinds(mapped)
	}
	return []string{kind}
}

// identify finds the catalogue entry a printed name means on this character:
// first among what the build already holds, then within the selected rules.
//
// What the build holds is a small closed set, so a feature printed with its
// dice -- "Sneak Attack (2d6)" -- still finds the feature. Items and spells
// are held to an exact name everywhere: "Silver Dagger" is not the dagger the
// character carries.
func (a *Agent) identify(ctx context.Context, s *AgentSession, cat *catalog.Catalog, state domain.State, kind, name string) (AgentCandidate, bool) {
	lookup := lookupKinds(kind)
	refKind, _ := rules.ParseRefKind(lookup[0])
	_, structural := structuralEvents[refKind]
	decorated := lookup[0] == "feature" || lookup[0] == "trait"
	held := func(ref rules.Ref) bool { return agentHolds(state, cat, ref) }
	if found, err := a.resolve(ctx, s, cat, lookup, name, held, decorated || structural); err == nil && (decorated || structural || found.Score >= .99) {
		return found, true
	}
	scope, loose := entityScope(state, cat, lookup[0])
	if found, err := a.resolve(ctx, s, cat, lookup, name, scope, loose); err == nil && (structural || found.Score >= .99) {
		return found, true
	}
	return AgentCandidate{}, false
}

// agentHolds reports whether the character as built already has an entry,
// whatever granted it.
func agentHolds(state domain.State, cat *catalog.Catalog, ref rules.Ref) bool {
	in := func(list []rules.Slug) bool { return slices.Contains(list, ref.Slug) }
	stacked := func(stacks []domain.ItemStack) bool {
		return slices.ContainsFunc(stacks, func(stack domain.ItemStack) bool { return stack.Item == ref.Slug && stack.Count > 0 })
	}
	skilled := func(skill rules.Slug) bool { return state.Skills.BySkill[skill].Proficiency != rules.NotProficient }
	switch ref.Kind {
	case rules.RefRace:
		return state.Identity.Race == ref.Slug
	case rules.RefSubrace:
		return state.Identity.Subrace == ref.Slug
	case rules.RefBackground:
		return state.Identity.Background == ref.Slug
	case rules.RefAlignment:
		return state.Identity.Alignment == ref.Slug
	case rules.RefClass:
		return slices.ContainsFunc(state.Identity.Classes, func(c domain.ClassLevel) bool { return c.Class == ref.Slug })
	case rules.RefSubclass:
		return slices.ContainsFunc(state.Identity.Classes, func(c domain.ClassLevel) bool { return c.Subclass == ref.Slug })
	case rules.RefFeat:
		return in(state.Feats)
	case rules.RefFeature:
		return in(state.Features)
	case rules.RefTrait:
		return in(state.Traits)
	case rules.RefSpell:
		return in(state.Spells.Cantrips) || in(state.Spells.Known) || in(state.Spells.Prepared)
	case rules.RefItem, rules.RefMagicItem:
		return stacked(state.Equipment.Equipped) || stacked(state.Equipment.Backpack) || stacked(state.Equipment.Loot)
	case rules.RefLanguage:
		return in(state.Base.Languages)
	case rules.RefSkill:
		return skilled(ref.Slug)
	case rules.RefProficiency:
		if def, ok := cat.Proficiencies.Get(ref.Slug); ok && def.Reference.Kind == rules.RefSkill {
			return skilled(def.Reference.Slug)
		}
		return in(state.Proficiencies)
	}
	return false
}
