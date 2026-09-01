package catalog

import (
	"strconv"

	domain "github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// converter turns domain values into response shapes.
//
// It holds the catalogue because two things cannot be converted without it:
// a text option's prose, which the compendium stores only as a key into the
// locale bundle, and a class's subclass level, which is derived from where
// the subclass's own advancement rows begin. Both are resolved here rather
// than left to the client, because an option a player cannot read is an
// option they cannot pick.
type converter struct{ cat *domain.Catalog }

// Converter converts catalogue values into response shapes.
//
// It is exported for one caller: the character package's prompt endpoint,
// which serves rules.Choice values and must serve them in exactly the shape
// this package serves the compendium's own prompts. A prompt the build flow
// synthesises and a prompt the compendium poses have to look identical to a
// client, or the client needs two renderers.
type Converter struct{ inner converter }

// NewConverter builds a Converter over a catalogue.
func NewConverter(cat *domain.Catalog) Converter {
	return Converter{inner: converter{cat: cat}}
}

// ChoiceValue converts one prompt.
func (c Converter) ChoiceValue(ch rules.Choice) Choice { return c.inner.choiceValue(ch) }

func entryOf(e domain.Entry) Entry {
	return Entry{Slug: e.Slug.String(), Name: e.Name, Desc: e.Desc}
}

func slugStrings(slugs []rules.Slug) []string {
	if len(slugs) == 0 {
		return nil
	}
	out := make([]string, 0, len(slugs))
	for _, s := range slugs {
		out = append(out, s.String())
	}
	return out
}

func refString(ref rules.Ref) string {
	if ref.IsZero() {
		return ""
	}
	return ref.String()
}

func costOf(c rules.Coins) *Cost {
	if c.Amount == 0 {
		return nil
	}
	return &Cost{Amount: c.Amount, Unit: string(c.Unit)}
}

func damageOf(d *rules.Damage) *Damage {
	if d == nil {
		return nil
	}
	return &Damage{Dice: d.Dice.String(), Type: d.Type.String()}
}

func abilityBonuses(bonuses []domain.AbilityBonus) []AbilityBonus {
	if len(bonuses) == 0 {
		return nil
	}
	out := make([]AbilityBonus, 0, len(bonuses))
	for _, b := range bonuses {
		out = append(out, AbilityBonus{Ability: b.Ability.Slug().String(), Bonus: b.Bonus})
	}
	return out
}

func itemStacks(stacks []domain.ItemStack) []ItemStack {
	if len(stacks) == 0 {
		return nil
	}
	out := make([]ItemStack, 0, len(stacks))
	for _, s := range stacks {
		out = append(out, ItemStack{Item: s.Item.String(), Count: s.Count})
	}
	return out
}

func prerequisites(ps []domain.Prerequisite) []Prerequisite {
	if len(ps) == 0 {
		return nil
	}
	out := make([]Prerequisite, 0, len(ps))
	for _, p := range ps {
		out = append(out, Prerequisite{
			Kind:         p.Kind.String(),
			Ability:      p.Ability.Slug().String(),
			MinimumScore: p.MinimumScore,
			Level:        p.Level,
			Ref:          refString(p.Ref),
		})
	}
	return out
}

// choice converts a prompt, resolving every option's key and prose.
func (c converter) choice(ch *rules.Choice) *Choice {
	if ch == nil || ch.Prompt.IsZero() {
		return nil
	}
	out := c.choiceValue(*ch)
	return &out
}

func (c converter) choiceValue(ch rules.Choice) Choice {
	return Choice{
		Prompt: ch.Prompt.String(),
		Choose: ch.Choose,
		Kind:   ch.Kind.String(),
		From:   c.optionSet(ch.From),
	}
}

func (c converter) choices(chs []rules.Choice) []Choice {
	if len(chs) == 0 {
		return nil
	}
	out := make([]Choice, 0, len(chs))
	for _, ch := range chs {
		out = append(out, c.choiceValue(ch))
	}
	return out
}

func (c converter) optionSet(set rules.OptionSet) OptionSet {
	out := OptionSet{Kind: optionSetKindName(set.Kind)}
	switch set.Kind {
	case rules.OptionsFromEquipmentCategory:
		out.Category = set.Category.String()
	case rules.OptionsFromCollection:
		out.Collection = set.Collection.String()
	case rules.OptionsExplicit:
		out.Options = make([]Option, 0, len(set.Options))
		for _, option := range set.Options {
			out.Options = append(out.Options, c.option(option))
		}
	}
	return out
}

func optionSetKindName(k rules.OptionSetKind) string {
	switch k {
	case rules.OptionsFromEquipmentCategory:
		return "equipment-category"
	case rules.OptionsFromCollection:
		return "collection"
	case rules.OptionsExplicit:
		return "explicit"
	}
	return "unknown"
}

// option flattens the ten option kinds into one shape with a discriminator.
//
// Key comes from rules.OptionKey, which is the same function the projector
// uses to resolve a stored answer. Serving it means the client never computes
// an option's identity, it echoes what the server sent -- so the rule that a
// bundle is named by what is in it lives in exactly one place.
func (c converter) option(o rules.Option) Option {
	out := Option{Key: rules.OptionKey(o).String()}
	switch opt := o.(type) {
	case rules.RefOption:
		out.Kind = "ref"
		out.Ref = refString(opt.Ref)
		out.Count = opt.Count
	case rules.NestedOption:
		out.Kind = "nested"
		nested := c.choiceValue(opt.Choice)
		out.Choice = &nested
	case rules.BundleOption:
		out.Kind = "bundle"
		out.Items = make([]Option, 0, len(opt.Items))
		for _, item := range opt.Items {
			out.Items = append(out.Items, c.option(item))
		}
	case rules.AbilityBonusOption:
		out.Kind = "ability-bonus"
		out.Ability = opt.Ability.Slug().String()
		out.Bonus = opt.Bonus
	case rules.DamageOption:
		out.Kind = "damage"
		damage := opt.Damage
		out.Damage = damageOf(&damage)
		out.Text = c.term(opt.Notes)
	case rules.MoneyOption:
		out.Kind = "money"
		out.Cost = costOf(opt.Coins)
	case rules.SizeOption:
		out.Kind = "size"
		out.Size = opt.Size.String()
	case rules.TextOption:
		out.Kind = "text"
		out.Text = c.term(opt.Key)
		out.Alignments = slugStrings(opt.Alignments)
	case rules.ActionOption:
		out.Kind = "action"
		out.Text = c.term(opt.Key)
		out.Count = opt.Count
		out.Recharge = opt.Recharge.String()
	case rules.ScoreMinimumOption:
		out.Kind = "score-minimum"
		out.Ability = opt.Ability.Slug().String()
		out.Minimum = opt.Minimum
	}
	return out
}

// term resolves prose the choice grammar points at by key, falling back to
// the key itself so a missing translation reads as a slug rather than as an
// empty option.
func (c converter) term(key rules.Slug) string {
	if key.IsZero() {
		return ""
	}
	if term, ok := c.cat.Terms.Get(key); ok && term.Name != "" {
		return term.Name
	}
	return key.String()
}

// The per-collection conversions.

func (c converter) ability(a domain.AbilityScore) Ability {
	return Ability{Entry: entryOf(a.Entry), FullName: a.FullName, Skills: slugStrings(a.Skills)}
}

func (c converter) skill(s domain.Skill) Skill {
	return Skill{Entry: entryOf(s.Entry), Ability: s.Ability.Slug().String()}
}

func (c converter) alignment(a domain.Alignment) Alignment {
	return Alignment{Entry: entryOf(a.Entry), Abbreviation: a.Abbreviation}
}

func (c converter) language(l domain.Language) Language {
	return Language{
		Entry:           entryOf(l.Entry),
		Type:            l.Type.String(),
		Script:          l.Script,
		TypicalSpeakers: l.TypicalSpeakers,
	}
}

func (c converter) proficiency(p domain.ProficiencyDef) Proficiency {
	return Proficiency{
		Entry:     entryOf(p.Entry),
		Type:      p.Type.String(),
		Reference: refString(p.Reference),
	}
}

func (c converter) equipmentCategory(e domain.EquipmentCategory) EquipmentCategory {
	return EquipmentCategory{Entry: entryOf(e.Entry), Items: slugStrings(e.Items)}
}

func (c converter) race(r domain.Race) Race {
	return Race{
		Entry:                 entryOf(r.Entry),
		Speed:                 int(r.Speed),
		Size:                  r.Size.String(),
		AbilityBonuses:        abilityBonuses(r.AbilityBonuses),
		AbilityBonusOptions:   c.choice(r.AbilityBonusOptions),
		Languages:             slugStrings(r.Languages),
		LanguageOptions:       c.choice(r.LanguageOptions),
		StartingProficiencies: slugStrings(r.StartingProficiencies),
		ProficiencyOptions:    c.choices(r.ProficiencyOptions),
		Traits:                slugStrings(r.Traits),
		Subraces:              slugStrings(r.Subraces),
		AgeDesc:               r.AgeDesc,
		AlignmentDesc:         r.AlignmentDesc,
		SizeDesc:              r.SizeDesc,
		LanguageDesc:          r.LanguageDesc,
	}
}

func (c converter) subrace(s domain.Subrace) Subrace {
	return Subrace{
		Entry:                 entryOf(s.Entry),
		Race:                  s.Race.String(),
		AbilityBonuses:        abilityBonuses(s.AbilityBonuses),
		Traits:                slugStrings(s.Traits),
		StartingProficiencies: slugStrings(s.StartingProficiencies),
		LanguageOptions:       c.choice(s.LanguageOptions),
	}
}

func (c converter) trait(t domain.Trait) Trait {
	out := Trait{
		Entry:              entryOf(t.Entry),
		Races:              slugStrings(t.Races),
		Subraces:           slugStrings(t.Subraces),
		Proficiencies:      slugStrings(t.Proficiencies),
		ProficiencyOptions: c.choice(t.ProficiencyOptions),
	}
	if t.Specific != nil {
		out.BreathWeapon = c.choice(t.Specific.BreathWeapon)
		out.SpellOptions = c.choice(t.Specific.SpellOptions)
		out.SubtraitOptions = c.choice(t.Specific.SubtraitOptions)
		out.DamageResistance = slugStrings(t.Specific.DamageResistance)
	}
	return out
}

func (c converter) class(cl domain.Class) Class {
	out := Class{
		Entry:                    entryOf(cl.Entry),
		HitDie:                   cl.HitDie,
		SavingThrows:             abilitySlugs(cl.SavingThrows),
		Proficiencies:            slugStrings(cl.Proficiencies),
		ProficiencyOptions:       c.choices(cl.ProficiencyOptions),
		StartingEquipment:        itemStacks(cl.StartingEquipment),
		StartingEquipmentOptions: c.choices(cl.StartingEquipmentOptions),
		Subclasses:               slugStrings(cl.Subclasses),
		SubclassLevel:            c.subclassLevel(cl),
	}
	if cl.Spellcasting != nil {
		info := make([]NamedText, 0, len(cl.Spellcasting.Info))
		for _, n := range cl.Spellcasting.Info {
			info = append(info, NamedText{Name: n.Name, Desc: n.Desc})
		}
		out.Spellcasting = &Spellcasting{
			Level:   cl.Spellcasting.Level,
			Ability: cl.Spellcasting.Ability.Slug().String(),
			Info:    info,
		}
	}
	mc := MultiClassing{
		Prerequisites:      prerequisites(cl.MultiClassing.Prerequisites),
		Proficiencies:      slugStrings(cl.MultiClassing.Proficiencies),
		ProficiencyOptions: c.choices(cl.MultiClassing.ProficiencyOptions),
	}
	if mc.Prerequisites != nil || mc.Proficiencies != nil || mc.ProficiencyOptions != nil {
		out.MultiClassing = &mc
	}
	return out
}

// subclassLevel is where a class's subclass rows begin -- the level at which
// the subclass is chosen. Subclass.Levels is empty for all twelve SRD
// entries, so it cannot be read from there.
func (c converter) subclassLevel(cl domain.Class) int {
	best := 0
	for _, slug := range cl.Subclasses {
		for _, row := range c.cat.ClassLevels(slug) {
			if row.Level > 0 && (best == 0 || row.Level < best) {
				best = row.Level
			}
		}
	}
	return best
}

func abilitySlugs(abilities []rules.Ability) []string {
	if len(abilities) == 0 {
		return nil
	}
	out := make([]string, 0, len(abilities))
	for _, a := range abilities {
		out = append(out, a.Slug().String())
	}
	return out
}

func (c converter) classLevel(l domain.ClassLevel) ClassLevel {
	out := ClassLevel{
		Class:               l.Class.String(),
		Subclass:            l.Subclass.String(),
		Level:               l.Level,
		ProficiencyBonus:    l.ProficiencyBonus,
		AbilityScoreBonuses: l.AbilityScoreBonuses,
		Features:            slugStrings(l.Features),
		CantripsKnown:       l.CantripsKnown,
		SpellsKnown:         l.SpellsKnown,
	}
	for level, slots := range l.SpellSlots {
		if slots > 0 {
			if out.SpellSlots == nil {
				out.SpellSlots = make(map[string]int)
			}
			out.SpellSlots[strconv.Itoa(level)] = slots
		}
	}
	for _, r := range l.Resources {
		resource := LevelResource{Key: r.Key.String(), Number: r.Number, Text: r.Text}
		if r.Dice != nil {
			resource.Dice = r.Dice.String()
		}
		out.Resources = append(out.Resources, resource)
	}
	return out
}

func (c converter) subclass(s domain.Subclass) Subclass {
	return Subclass{Entry: entryOf(s.Entry), Class: s.Class.String(), Flavor: s.Flavor}
}

func (c converter) feature(f domain.Feature) Feature {
	out := Feature{
		Entry:    entryOf(f.Entry),
		Class:    f.Class.String(),
		Subclass: f.Subclass.String(),
		Level:    f.Level,
	}
	if f.Specific != nil {
		out.ExpertiseOptions = c.choice(f.Specific.ExpertiseOptions)
		out.SubfeatureOptions = c.choice(f.Specific.SubfeatureOptions)
		out.EnemyTypeOptions = c.choice(f.Specific.EnemyTypeOptions)
		out.TerrainTypeOptions = c.choice(f.Specific.TerrainTypeOptions)
	}
	return out
}

func (c converter) background(b domain.Background) Background {
	return Background{
		Entry:                    entryOf(b.Entry),
		StartingProficiencies:    slugStrings(b.StartingProficiencies),
		LanguageOptions:          c.choice(b.LanguageOptions),
		StartingEquipment:        itemStacks(b.StartingEquipment),
		StartingEquipmentOptions: c.choices(b.StartingEquipmentOptions),
		StartingGold:             costOf(b.StartingGold),
		Feature:                  b.Feature.String(),
		PersonalityTraits:        c.choice(&b.PersonalityTraits),
		Ideals:                   c.choice(&b.Ideals),
		Bonds:                    c.choice(&b.Bonds),
		Flaws:                    c.choice(&b.Flaws),
	}
}

func (c converter) feat(f domain.Feat) Feat {
	return Feat{Entry: entryOf(f.Entry), Prerequisites: prerequisites(f.Prerequisites)}
}

func (c converter) item(i domain.Item) Item {
	out := Item{
		Entry:    entryOf(i.Entry),
		Category: i.Category.String(),
		Cost:     costOf(i.Cost),
		Weight:   i.Weight,
	}
	if w := i.Weapon; w != nil {
		out.Weapon = &Weapon{
			Category:         w.Category.String(),
			Range:            w.Range.String(),
			Damage:           damageOf(w.Damage),
			TwoHandedDamage:  damageOf(w.TwoHandedDamage),
			NormalRange:      int(w.NormalRange),
			LongRange:        int(w.LongRange),
			ThrowNormalRange: int(w.ThrowNormal),
			ThrowLongRange:   int(w.ThrowLong),
			Properties:       slugStrings(w.Properties),
		}
	}
	if a := i.Armor; a != nil {
		out.Armor = &Armor{
			Category:            a.Category.String(),
			BaseAC:              a.BaseAC,
			AddsDexBonus:        a.AddsDexBonus,
			MaxDexBonus:         a.MaxDexBonus,
			StrengthMinimum:     a.StrengthMinimum,
			StealthDisadvantage: a.StealthDisadvantage,
		}
	}
	if g := i.Gear; g != nil {
		out.Gear = &Gear{
			GearCategory: g.GearCategory.String(),
			Quantity:     g.Quantity,
			Contents:     itemStacks(g.Contents),
		}
	}
	if tl := i.Tool; tl != nil {
		out.Tool = &Tool{ToolCategory: tl.ToolCategory.String()}
	}
	if v := i.Vehicle; v != nil {
		out.Vehicle = &Vehicle{
			VehicleCategory: v.VehicleCategory.String(),
			Speed:           v.Speed,
			SpeedUnit:       v.SpeedUnit,
			Capacity:        v.Capacity,
		}
	}
	return out
}

func (c converter) magicItem(m domain.MagicItem) MagicItem {
	return MagicItem{
		Entry:    entryOf(m.Entry),
		Category: m.Category.String(),
		Rarity:   m.Rarity.String(),
		Variant:  m.IsVariant,
		Variants: slugStrings(m.Variants),
	}
}

// spellSummary is what the collection endpoint serves: enough to search and
// filter -- including by casting time and components -- not the whole spell.
// 319 spells at full fidelity is a payload nobody needs in order to browse;
// ?slugs= returns the rest. The material component's text is prose and stays
// with the detail.
func (c converter) spellSummary(s domain.Spell) Spell {
	return Spell{
		Entry:         Entry{Slug: s.Slug.String(), Name: s.Name},
		Source:        s.Source.String(),
		Level:         s.Level,
		School:        s.School.String(),
		Classes:       slugStrings(s.Classes),
		Subclasses:    slugStrings(s.Subclasses),
		Ritual:        s.Ritual,
		Concentration: s.Concentration,
		CastingTime:   castingTimeOf(s),
		Components: &Components{
			Verbal:   s.Components.Verbal,
			Somatic:  s.Components.Somatic,
			Material: s.Components.Material,
		},
	}
}

func castingTimeOf(s domain.Spell) *RuleValue {
	return &RuleValue{
		Kind:   s.CastingTime.Kind.String(),
		Amount: s.CastingTime.Amount,
		Unit:   s.CastingTime.Unit.String(),
	}
}

func (c converter) spell(s domain.Spell) Spell {
	out := c.spellSummary(s)
	out.Desc = s.Desc
	out.HigherLevel = s.HigherLevel
	out.AttackType = s.AttackType.String()
	out.Range = &RuleValue{Kind: s.Range.Kind.String(), Distance: int(s.Range.Distance)}
	out.Duration = &RuleValue{
		Kind:   s.Duration.Kind.String(),
		Amount: s.Duration.Amount,
		Unit:   s.Duration.Unit.String(),
		UpTo:   s.Duration.UpTo,
	}
	out.Components.Text = s.Material
	if s.Save != nil {
		out.SavingThrow = &SavingThrow{
			Ability: s.Save.Ability.Slug().String(),
			Success: s.Save.Success.String(),
		}
	}
	if s.Area != nil {
		out.AreaOfEffect = &Area{Shape: s.Area.Kind.String(), Size: int(s.Area.Size)}
	}
	if s.Damage != nil {
		out.Damage = &SpellDamage{
			Type:        s.Damage.Type.String(),
			AtSlotLevel: scalingOf(s.Damage.Scaling.AtSlotLevel),
			AtCharLevel: scalingOf(s.Damage.Scaling.AtCharacterLevel),
		}
	}
	if s.Heal != nil {
		out.Healing = scalingOf(s.Heal.AtSlotLevel)
		if out.Healing == nil {
			out.Healing = scalingOf(s.Heal.AtCharacterLevel)
		}
	}
	return out
}

// scalingOf renders a scaling table. A nil result is meaningful: reading a
// missing table as zero damage is the silent error the domain's shape exists
// to prevent, so an absent table stays absent rather than becoming an empty
// object.
func scalingOf(table map[int]rules.Dice) map[string]string {
	if len(table) == 0 {
		return nil
	}
	out := make(map[string]string, len(table))
	for level, dice := range table {
		out[strconv.Itoa(level)] = dice.String()
	}
	return out
}

func (c converter) termEntry(t domain.Term) Term { return Term{Entry: entryOf(t.Entry)} }
