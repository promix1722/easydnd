package file

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

var collectionTypes = map[string]func() any{
	"abilities": func() any { return &[]AbilityScore{} }, "skills": func() any { return &[]Skill{} },
	"alignments": func() any { return &[]Named{} }, "languages": func() any { return &[]Language{} },
	"conditions": func() any { return &[]Named{} }, "damage-types": func() any { return &[]Named{} },
	"magic-schools": func() any { return &[]Named{} }, "weapon-properties": func() any { return &[]Named{} },
	"proficiencies": func() any { return &[]Proficiency{} }, "equipment-categories": func() any { return &[]EquipmentCategory{} },
	"races": func() any { return &[]Race{} }, "subraces": func() any { return &[]Subrace{} }, "traits": func() any { return &[]Trait{} },
	"classes": func() any { return &[]Class{} }, "class-levels": func() any { return &[]ClassLevel{} }, "subclasses": func() any { return &[]Subclass{} }, "features": func() any { return &[]Feature{} },
	"backgrounds": func() any { return &[]Background{} }, "feats": func() any { return &[]Feat{} }, "equipment": func() any { return &[]Item{} }, "magic-items": func() any { return &[]MagicItem{} }, "spells": func() any { return &[]Spell{} },
}
var kindCollections = map[string]string{"ability": "abilities", "skill": "skills", "alignment": "alignments", "language": "languages", "condition": "conditions", "damage-type": "damage-types", "magic-school": "magic-schools", "weapon-property": "weapon-properties", "proficiency": "proficiencies", "equipment-category": "equipment-categories", "race": "races", "subrace": "subraces", "trait": "traits", "class": "classes", "subclass": "subclasses", "feature": "features", "background": "backgrounds", "feat": "feats", "item": "equipment", "magic-item": "magic-items", "spell": "spells", "resource": "resources", "rule": "rules", "action": "actions"}

func validateMechanics(m PackMechanics) error {
	check := func(e Expression) error { return e.domain().Validate(0) }
	for _, o := range m.Overrides {
		ref, ok := rules.ParseRef(string(o.Target))
		if !ok || len(strings.Split(string(o.Target), ":")) != 3 || o.Version == "" {
			return fmt.Errorf("replacement needs canonical target and version constraint")
		}
		switch ref.Kind {
		case rules.RefResource:
			if o.Resource == nil || o.Rule != nil {
				return fmt.Errorf("invalid resource replacement")
			}
			if err := validateMechanics(PackMechanics{Resources: []ResourceDefinition{*o.Resource}}); err != nil {
				return err
			}
		case rules.RefRule:
			if o.Rule == nil || o.Resource != nil {
				return fmt.Errorf("invalid rule replacement")
			}
			if err := validateMechanics(PackMechanics{Rules: []RuleDefinition{*o.Rule}}); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported replacement kind")
		}
	}
	if m.Core != nil {
		if m.Core.MinScore < 1 || m.Core.MaxScore < m.Core.MinScore || m.Core.MaxScore > 1000 || m.Core.AbilityScoreIncrease < 1 {
			return fmt.Errorf("invalid ability policy")
		}
		if m.Core.MaxLevel < 1 || m.Core.MaxLevel > 100 {
			return fmt.Errorf("invalid maximum level")
		}
		for _, e := range []Expression{m.Core.HitPointFirst, m.Core.HitPointLater} {
			if err := check(e); err != nil {
				return err
			}
		}
		for _, ps := range [][]RecoveryPolicy{m.Core.HitDiceRecovery, m.Core.SpellSlotRecovery} {
			if err := validateRecovery(ps); err != nil {
				return err
			}
		}
		if err := check(m.Core.Proficiency); err != nil {
			return err
		}
		if err := check(m.Core.AbilityModifier); err != nil {
			return err
		}
	}
	ids := map[string]bool{}
	for _, r := range m.Resources {
		if r.ID == "" || ids[r.ID] {
			return fmt.Errorf("duplicate or empty resource ID %q", r.ID)
		}
		ids[r.ID] = true
		if r.Kind != "pool" && r.Kind != "parameter" {
			return fmt.Errorf("unknown resource kind %q", r.Kind)
		}
		if _, ok := rules.ParseRef(string(r.Owner)); !ok {
			return fmt.Errorf("invalid resource owner %q", r.Owner)
		}
		if r.MinimumLevel < 0 || r.Input == "" {
			return fmt.Errorf("invalid resource activation/input %q", r.ID)
		}
		if r.Capacity == nil && len(r.Rows) == 0 {
			return fmt.Errorf("resource %s requires a capacity expression or progression", r.ID)
		}
		if r.Capacity != nil {
			if err := check(*r.Capacity); err != nil {
				return err
			}
		}
		last := -1
		for _, row := range r.Rows {
			if row.From <= last || row.Capacity < 0 || row.SlotLevel < 0 || row.SlotLevel > 9 {
				return fmt.Errorf("invalid progression in %s", r.ID)
			}
			if row.Rational != nil && (r.Kind != "parameter" || row.Rational.Denominator <= 0) {
				return fmt.Errorf("invalid rational parameter")
			}
			if row.Boolean != nil && r.Kind != "parameter" {
				return fmt.Errorf("boolean belongs to parameter")
			}
			if row.Rational != nil && row.Boolean != nil {
				return fmt.Errorf("ambiguous parameter type")
			}
			last = row.From
			if row.Dice != "" {
				if _, err := rules.ParseDice(row.Dice); err != nil {
					return err
				}
			}
		}
		if r.SharedKey != "" && r.Combine != "max" && r.Combine != "sum" {
			return fmt.Errorf("shared resource %s needs max or sum capacity policy", r.ID)
		}
		for _, rec := range r.Recovery {
			if rec.When != nil {
				if err := check(*rec.When); err != nil {
					return err
				}
			}
			if rec.Trigger == "" {
				return fmt.Errorf("empty recovery trigger")
			}
			switch rec.Operation {
			case "all":
			case "amount", "budget":
				if err := check(rec.Amount); err != nil {
					return err
				}
				if rec.Operation == "budget" && rec.Budget == "" {
					return fmt.Errorf("missing recovery budget")
				}
			default:
				return fmt.Errorf("unknown recovery operation %q", rec.Operation)
			}
		}
	}
	ids = map[string]bool{}
	for _, r := range m.Rules {
		if r.ID == "" || ids[r.ID] {
			return fmt.Errorf("duplicate or empty rule ID %q", r.ID)
		}
		ids[r.ID] = true
		if _, ok := rules.ParseRef(string(r.Owner)); !ok {
			return fmt.Errorf("invalid rule owner %q", r.Owner)
		}
		if r.When != nil {
			if err := check(*r.When); err != nil {
				return err
			}
		}
		if len(r.Effects) == 0 && len(r.Choices) == 0 && !r.Manual {
			return fmt.Errorf("empty rule %s must declare manual resolution", r.ID)
		}
		for _, e := range r.Effects {
			switch e.Op {
			case "grant":
				if ref, ok := rules.ParseRef(string(e.Ref)); !ok || !grantKind(ref.Kind) {
					return fmt.Errorf("invalid grant reference")
				}
			case "add", "max", "set":
				if !validEffectTarget(e.Target) {
					return fmt.Errorf("missing effect target")
				}
				if err := check(e.Value); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported effect operation %q", e.Op)
			}
		}
	}
	ids = map[string]bool{}
	for _, a := range m.Actions {
		if a.ID == "" || ids[a.ID] || !a.Manual {
			return fmt.Errorf("invalid action: v1 requires manual outcomes")
		}
		ids[a.ID] = true
		if _, ok := rules.ParseRef(string(a.Owner)); !ok && a.Owner != "" {
			return fmt.Errorf("invalid action owner")
		}
		if a.Owner == "" && (a.When != nil || a.MinimumLevel != 0 || len(a.Costs) > 0) {
			return fmt.Errorf("an action without an owner takes no condition or cost")
		}
		if a.Kind != "" && !slices.Contains(ActionKinds, a.Kind) {
			return fmt.Errorf("unknown action kind %q", a.Kind)
		}
		if a.When != nil {
			if err := check(*a.When); err != nil {
				return err
			}
		}
		for _, cost := range a.Costs {
			if cost.Resource == "" {
				return fmt.Errorf("missing cost resource")
			}
			if err := check(cost.Amount); err != nil {
				return err
			}
		}
	}
	seenBenefits := map[string]bool{}
	for _, b := range m.SpellBenefits {
		if b.ID == "" || seenBenefits[b.ID] || b.Level < 1 || b.Level > 20 || b.Count < 0 || b.SpellLevel < -1 || b.SpellLevel > 9 {
			return fmt.Errorf("invalid spell benefit %s", b.ID)
		}
		seenBenefits[b.ID] = true
		if b.Mode != "cantrip" && b.Mode != "known" && b.Mode != "prepared" && b.Mode != "arcanum" && b.Mode != "mastery" && b.Mode != "spellbook" {
			return fmt.Errorf("invalid spell benefit mode %s", b.ID)
		}
		if b.From != "" && b.From != "class" && b.From != "any" && b.From != "book" {
			return fmt.Errorf("invalid spell benefit list %s", b.ID)
		}
	}

	for id, c := range m.Casting {
		if c.Selection != "" && c.Selection != "known" && c.Selection != "prepared" && c.Selection != "spellbook" {
			return fmt.Errorf("invalid spell selection %s", id)
		}
		if c.PrepareDivisor < 0 || c.BookStart < 0 || c.BookPerLevel < 0 {
			return fmt.Errorf("invalid spell selection counts %s", id)
		}
		if c.Kind != "shared" && c.Kind != "independent" {
			return fmt.Errorf("invalid caster profile %s", id)
		}
		if c.Denominator < 1 || c.Numerator < 0 || c.StartsAt < 1 || (c.Rounding != "floor" && c.Rounding != "ceil") {
			return fmt.Errorf("invalid caster contribution %s", id)
		}
	}
	return nil
}

// normalizeID accepts canonical refs or local IDs. Qualified cross-pack refs
// must explicitly name a declared dependency; the registry checks that graph.
// The six standard scores have global identities even in an independently
// authored core pack. Packs still cannot introduce additional scores.
func normalizeAbility(value string) string {
	if strings.Contains(value, ":") {
		r, ok := rules.ParseRef(value)
		if !ok || r.Kind != rules.RefAbility {
			return value
		}
	}
	last := value
	if at := strings.LastIndexAny(last, ":/"); at >= 0 {
		last = last[at+1:]
	}
	if _, ok := rules.ParseAbility(last); ok {
		return last
	}
	return value
}
func normalizeID(packID, value string) string {
	if value == "" {
		return ""
	}
	if r, ok := rules.ParseRef(value); ok {
		if r.Kind == rules.RefAbility {
			return normalizeAbility(value)
		}
		return r.Slug.String()
	}
	if strings.Contains(value, "/") {
		return value
	}
	return rules.QualifiedSlug(packID, value).String()
}
func normalizeRef(packID, value string) string {
	if r, ok := rules.ParseRef(value); ok && r.Kind == rules.RefAbility {
		return "ability:" + normalizeAbility(value)
	}
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ":")
	if len(parts) == 3 {
		if r, ok := rules.ParseRef(value); ok {
			return r.String()
		}
		return value
	}
	if len(parts) == 2 {
		return parts[0] + ":" + normalizeID(packID, parts[1])
	}
	return value
}

var slugFields = map[string]bool{"alignments": true, "list": true, "slug": true, "class": true, "classes": true, "subclass": true, "subclasses": true, "race": true, "races": true, "subrace": true, "subraces": true, "traits": true, "features": true, "feature": true, "parent": true, "spell": true, "spells": true, "skills": true, "languages": true, "ability": true, "savingThrows": true, "abilityPriority": true, "spellcastingAbility": true, "proficiencies": true, "startingProficiencies": true, "multiclassProficiencies": true, "invocations": true, "variants": true, "item": true, "school": true, "damageType": true, "twoHandedDamageType": true, "damageResistance": true, "properties": true, "uses": true}

func normalizeValue(packID, key string, v any) any {
	switch x := v.(type) {
	case string:
		if key == "ability" || key == "savingThrows" || key == "abilityPriority" || key == "spellcastingAbility" {
			return normalizeAbility(x)
		}
		if len(strings.Split(x, ":")) == 3 && key != "ref" && key != "reference" && key != "owner" {
			if r, ok := rules.ParseRef(x); ok {
				return r.Slug.String()
			}
		}
		if key == "ref" || key == "reference" || key == "owner" {
			return normalizeRef(packID, x)
		}
		if slugFields[key] || key == "prompt" || key == "key" {
			return normalizeID(packID, x)
		}
	case []any:
		for i, v := range x {
			x[i] = normalizeValue(packID, key, v)
		}
	case map[string]any:
		for k, v := range x {
			x[k] = normalizeValue(packID, k, v)
		}
		// An option set drawn from an equipment category names it by slug.
		// "category" is not a slug field in general -- a weapon's is "simple"
		// -- so it is qualified only here; left bare, a namespaced pack asks
		// for "arcane-foci", holds "pack/arcane-foci", and offers nothing.
		if x["kind"] == "equipment-category" {
			if category, ok := x["category"].(string); ok {
				x["category"] = normalizeID(packID, category)
			}
		}
	}
	return v
}
func normalizedEntities(p *PackDocument) (map[string][]any, error) {
	out := map[string][]any{}
	for name, raw := range p.Entities {
		var rows []any
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, err
		}
		for i, v := range rows {
			rows[i] = normalizeValue(p.Manifest.ID, "", v)
			m := rows[i].(map[string]any)
			if name == "abilities" {
				m["slug"] = normalizeAbility(m["slug"].(string))
			}
			if name == "equipment" || name == "magic-items" {
				if x, ok := m["category"].(string); ok {
					m["category"] = normalizeID(p.Manifest.ID, x)
				}
			}
			if name == "equipment-categories" {
				if a, ok := m["items"].([]any); ok {
					for i, v := range a {
						a[i] = normalizeID(p.Manifest.ID, v.(string))
					}
				}
			}
		}
		out[name] = rows
	}
	return out, nil
}

func validateReferences(docs []*PackDocument, entities map[string][]any, m PackMechanics, prose map[string]Bundle) error {
	known := map[string]map[string]bool{}
	for collection, rows := range entities {
		known[collection] = map[string]bool{}
		if collection == "class-levels" {
			continue
		}
		for _, row := range rows {
			id, _ := row.(map[string]any)["slug"].(string)
			if known[collection][id] {
				return fmt.Errorf("duplicate compiled identity %s/%s", collection, id)
			}
			known[collection][id] = true
		}
	}
	// Prose for an entity nobody defines would otherwise vanish silently --
	// the one way a typo in an overlay's canonical key could go unnoticed.
	for collection, bundle := range prose {
		if collection == "class-levels" || known[collection] == nil {
			continue
		}
		for id := range bundle {
			if !known[collection][id] {
				return fmt.Errorf("localized identity without entity %s/%s", collection, id)
			}
		}
	}
	known["resources"] = map[string]bool{}
	for _, r := range m.Resources {
		known["resources"][r.ID] = true
	}
	known["rules"] = map[string]bool{}
	for _, r := range m.Rules {
		known["rules"][r.ID] = true
	}
	check := func(kind, id string) error {
		collection, ok := kindCollections[kind]
		if !ok || !known[collection][id] {
			return fmt.Errorf("unresolved reference %s:%s", kind, id)
		}
		return nil
	}
	ref := func(value string) error {
		if value == "" {
			return nil
		}
		r, ok := rules.ParseRef(value)
		if !ok {
			return fmt.Errorf("invalid reference %s", value)
		}
		return check(r.Kind.String(), r.Slug.String())
	}
	fields := map[string]string{"alignments": "alignment", "class": "class", "classes": "class", "subclass": "subclass", "subclasses": "subclass", "race": "race", "races": "race", "subrace": "subrace", "subraces": "subrace", "traits": "trait", "features": "feature", "feature": "feature", "spell": "spell", "spells": "spell", "skills": "skill", "languages": "language", "ability": "ability", "savingThrows": "ability", "abilityPriority": "ability", "spellcastingAbility": "ability", "proficiencies": "proficiency", "startingProficiencies": "proficiency", "multiclassProficiencies": "proficiency", "invocations": "feature", "variants": "magic-item", "item": "item", "school": "magic-school", "damageType": "damage-type", "twoHandedDamageType": "damage-type", "damageResistance": "damage-type", "properties": "weapon-property", "uses": "resource"}
	var walk func(string, any) error
	walk = func(key string, value any) error {
		switch x := value.(type) {
		case string:
			if x == "" {
				return nil
			}
			if key == "ref" || key == "reference" {
				return ref(x)
			}
			if key == "races" && known["subraces"][x] {
				return nil
			}
			if kind := fields[key]; kind != "" {
				return check(kind, x)
			}
		case []any:
			for _, v := range x {
				if err := walk(key, v); err != nil {
					return err
				}
			}
		case map[string]any:
			for k, v := range x {
				if err := walk(k, v); err != nil {
					return err
				}
			}
			// "Any item in this category" with no such category is a choice
			// with nothing to choose; say so at load, not on the build screen.
			if x["kind"] == "equipment-category" {
				category, _ := x["category"].(string)
				if err := check("equipment-category", category); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for name, rows := range entities {
		for _, row := range rows {
			if err := walk("", row); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	input := func(value string) error {
		for _, prefix := range []string{"class:", "ability:", "modifier:"} {
			if strings.HasPrefix(value, prefix) {
				kind := strings.TrimSuffix(prefix, ":")
				if kind == "modifier" {
					kind = "ability"
				}
				return check(kind, strings.TrimPrefix(value, prefix))
			}
		}
		if value == "equipped:armor" || value == "equipped:shield" || value == "level" || value == "proficiency" || value == "score" || value == "hitDie" {
			return nil
		}
		return fmt.Errorf("unknown expression input %q", value)
	}
	var expression func(Expression) error
	expression = func(e Expression) error {
		if e.Op == "read" {
			if err := input(e.Ref); err != nil {
				return err
			}
		}
		for _, a := range e.Args {
			if err := expression(a); err != nil {
				return err
			}
		}
		return nil
	}
	for _, r := range m.Resources {
		if err := ref(string(r.Owner)); err != nil {
			return err
		}
		if err := input(r.Input); err != nil {
			return err
		}
		if r.Capacity != nil {
			if err := expression(*r.Capacity); err != nil {
				return err
			}
		}
		for _, p := range r.Recovery {
			if p.When != nil {
				if err := expression(*p.When); err != nil {
					return err
				}
			}
			if err := expression(p.Amount); err != nil {
				return err
			}
		}
	}
	for _, a := range m.Actions {
		if err := ref(string(a.Owner)); err != nil {
			return err
		}
		if a.When != nil {
			if err := expression(*a.When); err != nil {
				return err
			}
		}
		for _, cost := range a.Costs {
			if err := check("resource", cost.Resource); err != nil {
				return err
			}
			if err := expression(cost.Amount); err != nil {
				return err
			}
		}
	}
	for _, benefit := range m.SpellBenefits {
		if err := ref(string(benefit.Owner)); err != nil {
			return err
		}
		if benefit.Class != "" {
			if err := check("class", benefit.Class); err != nil {
				return err
			}
		}
		for _, spell := range benefit.Spells {
			if err := check("spell", spell); err != nil {
				return err
			}
		}
	}
	for _, requirement := range m.ChoiceRequirements {
		if requirement.Prompt == "" || requirement.Pick == "" || len(requirement.AnyProficiency) == 0 {
			return fmt.Errorf("invalid choice requirement")
		}
		if err := check("item", requirement.Pick); err != nil {
			return err
		}
		for _, prof := range requirement.AnyProficiency {
			if err := check("proficiency", prof); err != nil {
				return err
			}
		}
	}
	grants := map[string][]string{}
	for _, r := range m.Rules {
		if err := ref(string(r.Owner)); err != nil {
			return err
		}
		if r.When != nil {
			if err := expression(*r.When); err != nil {
				return err
			}
		}
		for _, e := range r.Effects {
			if strings.HasPrefix(e.Target, "abilities.") {
				if err := check("ability", strings.TrimPrefix(e.Target, "abilities.")); err != nil {
					return err
				}
			}
			if err := ref(string(e.Ref)); err != nil {
				return err
			}
			if err := expression(e.Value); err != nil {
				return err
			}
			if e.Op == "grant" {
				grants[string(r.Owner)] = append(grants[string(r.Owner)], string(e.Ref))
			}
		}
	}
	state := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("cyclic rule grants at %s", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, to := range grants[id] {
			if err := visit(to); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for id := range grants {
		if err := visit(id); err != nil {
			return err
		}
	}
	for id, p := range m.Casting {
		if known["subclasses"][id] {
			if p.List == "" || p.Ability == "" {
				return fmt.Errorf("subclass casting %s requires list and ability", id)
			}
		} else if err := check("class", id); err != nil {
			return err
		}
		if p.List != "" {
			if err := check("class", p.List); err != nil {
				return err
			}
		}
		if p.Ability != "" {
			if err := check("ability", p.Ability); err != nil {
				return err
			}
		}
		if p.Resource != "" {
			if err := check("resource", p.Resource); err != nil {
				return err
			}
		}
	}
	for _, p := range docs {

		if p.Mechanics.Core != nil {
			// These policies run with restricted environments. Exhaustive bounded
			// checks catch missing inputs and division by zero before readiness.
			core := p.Mechanics.Core
			for n := 0; n <= core.MaxLevel; n++ {
				if _, err := core.Proficiency.domain().Eval(rules.Variables{"level": n}); err != nil {
					return err
				}
			}
			for n := core.MinScore; n <= core.MaxScore; n++ {
				if _, err := core.AbilityModifier.domain().Eval(rules.Variables{"score": n}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
