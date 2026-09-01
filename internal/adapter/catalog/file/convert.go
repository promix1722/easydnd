package file

import (
	"fmt"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// conv converts wire values to domain values, latching the first error.
//
// Straight-line conversion code with a single check at the end reads far
// better than several hundred `if err != nil` blocks, and the latch means a
// malformed file still reports the first thing that was actually wrong rather
// than a cascade of consequences. The pattern is bufio.Scanner's.
type conv struct {
	err error
	// where names the file being converted, so an error says which one.
	where string
}

// fail records the first error and ignores the rest.
func (c *conv) fail(format string, a ...any) {
	if c.err == nil {
		c.err = types.NewValidationError("%s: %s", c.where, fmt.Sprintf(format, a...))
	}
}

// Err returns the first error encountered, or nil.
func (c *conv) Err() error { return c.err }

func (c *conv) ability(s string) rules.Ability {
	if s == "" {
		return rules.AbilityNone
	}
	a, ok := rules.ParseAbility(s)
	if !ok {
		c.fail("unknown ability %q", s)
	}
	return a
}

func (c *conv) abilities(ss []string) []rules.Ability {
	out := make([]rules.Ability, 0, len(ss))
	for _, s := range ss {
		out = append(out, c.ability(s))
	}
	return out
}

func (c *conv) size(s string) rules.Size {
	if s == "" {
		return rules.SizeNone
	}
	size, ok := rules.ParseSize(s)
	if !ok {
		c.fail("unknown size %q", s)
	}
	return size
}

func (c *conv) dice(s string) rules.Dice {
	if s == "" {
		return rules.Dice{}
	}
	d, err := rules.ParseDice(s)
	if err != nil {
		c.fail("bad dice %q: %v", s, err)
	}
	return d
}

func (c *conv) cost(w Cost) rules.Coins {
	if w.Unit == "" {
		return rules.Coins{}
	}
	unit, err := rules.ParseCoinUnit(w.Unit)
	if err != nil {
		c.fail("bad coin unit %q", w.Unit)
	}
	return rules.Coins{Amount: w.Amount, Unit: unit}
}

func (c *conv) ref(w Ref) rules.Ref {
	if w == "" {
		return rules.Ref{}
	}
	r, ok := rules.ParseRef(string(w))
	if !ok {
		c.fail("bad reference %q", w)
	}
	return r
}

// slugs converts a list of slug strings. Empty input yields nil rather than an
// empty slice, so a round trip through JSON omitempty is stable.
func slugs(ss []string) []rules.Slug {
	if len(ss) == 0 {
		return nil
	}
	out := make([]rules.Slug, 0, len(ss))
	for _, s := range ss {
		out = append(out, rules.Slug(s))
	}
	return out
}

func (c *conv) choiceKind(s string) rules.ChoiceKind {
	if s == "" {
		return rules.ChooseNothing
	}
	k, ok := rules.ParseChoiceKind(s)
	if !ok {
		c.fail("unknown choice kind %q", s)
	}
	return k
}

func (c *conv) choice(w *Choice) *rules.Choice {
	if w == nil {
		return nil
	}
	out := c.choiceValue(*w)
	return &out
}

func (c *conv) choiceValue(w Choice) rules.Choice {
	return rules.Choice{
		Prompt: rules.Slug(w.Prompt),
		Choose: w.Choose,
		Kind:   c.choiceKind(w.Kind),
		From:   c.optionSet(w.From),
	}
}

func (c *conv) choices(ws []Choice) []rules.Choice {
	if len(ws) == 0 {
		return nil
	}
	out := make([]rules.Choice, 0, len(ws))
	for _, w := range ws {
		out = append(out, c.choiceValue(w))
	}
	return out
}

func (c *conv) optionSet(w OptionSet) rules.OptionSet {
	kind, ok := optionSetKinds[w.Kind]
	if !ok {
		c.fail("unknown option set kind %q", w.Kind)
	}
	set := rules.OptionSet{Kind: kind, Category: rules.Slug(w.Category)}
	if w.Collection != "" {
		collection, known := rules.ParseRefKind(w.Collection)
		if !known {
			c.fail("unknown collection %q", w.Collection)
		}
		set.Collection = collection
	}
	for _, opt := range w.Options {
		set.Options = append(set.Options, c.option(opt))
	}
	return set
}

// option turns a tagged wire struct back into the sealed domain interface.
//
// The switch is the one place where an unrecognised option kind can be
// caught; the sealed interface makes every consumer downstream exhaustive by
// construction.
func (c *conv) option(w Option) rules.Option {
	switch w.Kind {
	case OptionRef:
		return rules.RefOption{Ref: c.ref(w.Ref), Count: w.Count}
	case OptionNested:
		if w.Choice == nil {
			c.fail("nested option has no choice")
			return rules.NestedOption{}
		}
		return rules.NestedOption{Choice: c.choiceValue(*w.Choice)}
	case OptionBundle:
		items := make([]rules.Option, 0, len(w.Items))
		for _, item := range w.Items {
			items = append(items, c.option(item))
		}
		return rules.BundleOption{Items: items}
	case OptionAbilityBonus:
		return rules.AbilityBonusOption{Ability: c.ability(w.Ability), Bonus: w.Bonus}
	case OptionDamage:
		return rules.DamageOption{
			Damage: rules.Damage{Dice: c.dice(w.Dice), Type: rules.Slug(w.DamageType)},
			Notes:  rules.Slug(w.Key),
		}
	case OptionMoney:
		if w.Cost == nil {
			c.fail("money option has no cost")
			return rules.MoneyOption{}
		}
		return rules.MoneyOption{Coins: c.cost(*w.Cost)}
	case OptionSize:
		return rules.SizeOption{Size: c.size(w.Size)}
	case OptionText:
		return rules.TextOption{Key: rules.Slug(w.Key), Alignments: slugs(w.Alignments)}
	case OptionAction:
		return rules.ActionOption{Key: rules.Slug(w.Key), Count: w.Count, Recharge: rules.Slug(w.Recharge)}
	case OptionScoreMinimum:
		return rules.ScoreMinimumOption{Ability: c.ability(w.Ability), Minimum: w.Minimum}
	default:
		c.fail("unknown option kind %q", w.Kind)
		return rules.RefOption{}
	}
}

func (c *conv) prerequisites(ws []Prerequisite) []catalog.Prerequisite {
	if len(ws) == 0 {
		return nil
	}
	out := make([]catalog.Prerequisite, 0, len(ws))
	for _, w := range ws {
		kind, ok := prerequisiteKinds[w.Kind]
		if !ok {
			c.fail("unknown prerequisite kind %q", w.Kind)
		}
		out = append(out, catalog.Prerequisite{
			Kind:         kind,
			Ability:      c.ability(w.Ability),
			MinimumScore: w.MinimumScore,
			Level:        w.Level,
			Ref:          c.ref(w.Ref),
		})
	}
	return out
}

func (c *conv) itemStacks(ws []ItemStack) []catalog.ItemStack {
	if len(ws) == 0 {
		return nil
	}
	out := make([]catalog.ItemStack, 0, len(ws))
	for _, w := range ws {
		out = append(out, catalog.ItemStack{Item: rules.Slug(w.Item), Count: w.Count})
	}
	return out
}

// entry builds the identity-and-prose head shared by every catalogue entity.
// A slug with no prose in any locale yields its own slug as the name, which is
// ugly on screen but traceable, and far better than an empty string.
func entry(slug string, bundle Bundle) catalog.Entry {
	p, ok := bundle[slug]
	if !ok {
		return catalog.Entry{Slug: rules.Slug(slug), Name: slug}
	}
	name := p.Name
	if name == "" {
		name = slug
	}
	return catalog.Entry{Slug: rules.Slug(slug), Name: name, Desc: p.Desc}
}

// prose returns the bundle entry for a slug, or the zero Prose.
func prose(slug string, bundle Bundle) Prose { return bundle[slug] }

// The per-entity conversions. Each takes the mechanics record and the locale
// bundle for its collection, and produces the resolved domain value.

func (c *conv) abilityScore(w AbilityScore, b Bundle) catalog.AbilityScore {
	p := prose(w.Slug, b)
	return catalog.AbilityScore{
		Entry:    entry(w.Slug, b),
		Ability:  c.ability(w.Slug),
		FullName: p.Field(ProseFullName),
		Skills:   slugs(w.Skills),
	}
}

func (c *conv) skill(w Skill, b Bundle) catalog.Skill {
	return catalog.Skill{Entry: entry(w.Slug, b), Ability: c.ability(w.Ability)}
}

func (c *conv) alignment(w Named, b Bundle) catalog.Alignment {
	return catalog.Alignment{
		Entry:        entry(w.Slug, b),
		Abbreviation: prose(w.Slug, b).Field(ProseAbbreviation),
	}
}

func (c *conv) language(w Language, b Bundle) catalog.Language {
	kind, ok := languageTypes[w.Type]
	if !ok && w.Type != "" {
		c.fail("unknown language type %q", w.Type)
	}
	p := prose(w.Slug, b)
	return catalog.Language{
		Entry:           entry(w.Slug, b),
		Type:            kind,
		Script:          p.Field(ProseScript),
		TypicalSpeakers: p.Block(ProseTypicalSpeakers),
	}
}

func (c *conv) proficiency(w Proficiency, b Bundle) catalog.ProficiencyDef {
	kind, ok := proficiencyTypes[w.Type]
	if !ok && w.Type != "" {
		c.fail("unknown proficiency type %q", w.Type)
	}
	return catalog.ProficiencyDef{
		Entry:     entry(w.Slug, b),
		Type:      kind,
		Reference: c.ref(w.Reference),
		Classes:   slugs(w.Classes),
		Races:     slugs(w.Races),
	}
}

func (c *conv) race(w Race, b Bundle) catalog.Race {
	p := prose(w.Slug, b)
	return catalog.Race{
		Entry:                 entry(w.Slug, b),
		Speed:                 rules.Feet(w.Speed),
		Size:                  c.size(w.Size),
		AbilityBonuses:        c.abilityBonuses(w.AbilityBonuses),
		AbilityBonusOptions:   c.choice(w.AbilityBonusOptions),
		Languages:             slugs(w.Languages),
		LanguageOptions:       c.choice(w.LanguageOptions),
		StartingProficiencies: slugs(w.StartingProficiencies),
		ProficiencyOptions:    c.choices(w.ProficiencyOptions),
		Traits:                slugs(w.Traits),
		Subraces:              slugs(w.Subraces),
		AgeDesc:               p.Block(ProseAge),
		AlignmentDesc:         p.Block(ProseAlignment),
		SizeDesc:              p.Block(ProseSizeDesc),
		LanguageDesc:          p.Block(ProseLanguageDesc),
	}
}

func (c *conv) abilityBonuses(ws []AbilityBonus) []catalog.AbilityBonus {
	if len(ws) == 0 {
		return nil
	}
	out := make([]catalog.AbilityBonus, 0, len(ws))
	for _, w := range ws {
		out = append(out, catalog.AbilityBonus{Ability: c.ability(w.Ability), Bonus: w.Bonus})
	}
	return out
}

func (c *conv) subrace(w Subrace, b Bundle) catalog.Subrace {
	return catalog.Subrace{
		Entry:                 entry(w.Slug, b),
		Race:                  rules.Slug(w.Race),
		AbilityBonuses:        c.abilityBonuses(w.AbilityBonuses),
		Traits:                slugs(w.Traits),
		StartingProficiencies: slugs(w.StartingProficiencies),
		LanguageOptions:       c.choice(w.LanguageOptions),
	}
}

func (c *conv) trait(w Trait, b Bundle) catalog.Trait {
	t := catalog.Trait{
		Entry:              entry(w.Slug, b),
		Races:              slugs(w.Races),
		Subraces:           slugs(w.Subraces),
		Proficiencies:      slugs(w.Proficiencies),
		ProficiencyOptions: c.choice(w.ProficiencyOptions),
	}
	// Only allocate the payload for the traits that actually carry one, so a
	// nil Specific keeps meaning "prose only".
	if w.BreathWeapon != nil || w.SpellOptions != nil || w.SubtraitOptions != nil || len(w.DamageResistance) > 0 {
		t.Specific = &catalog.TraitSpecific{
			BreathWeapon:     c.choice(w.BreathWeapon),
			SpellOptions:     c.choice(w.SpellOptions),
			SubtraitOptions:  c.choice(w.SubtraitOptions),
			DamageResistance: slugs(w.DamageResistance),
		}
	}
	return t
}

func (c *conv) class(w Class, b Bundle) catalog.Class {
	cl := catalog.Class{
		Entry:                    entry(w.Slug, b),
		HitDie:                   w.HitDie,
		SavingThrows:             c.abilities(w.SavingThrows),
		Proficiencies:            slugs(w.Proficiencies),
		ProficiencyOptions:       c.choices(w.ProficiencyOptions),
		StartingEquipment:        c.itemStacks(w.StartingEquipment),
		StartingEquipmentOptions: c.choices(w.StartingEquipmentOptions),
		Subclasses:               slugs(w.Subclasses),
		MultiClassing: catalog.MultiClassing{
			Prerequisites:      c.prerequisites(w.MulticlassPrerequisites),
			Proficiencies:      slugs(w.MulticlassProficiencies),
			ProficiencyOptions: c.choices(w.MulticlassOptions),
		},
	}
	// Nil spellcasting and empty spellcasting mean different things: a
	// barbarian has none, a wizard has one that begins at level 1.
	if w.SpellcastingAbility != "" {
		cl.Spellcasting = &catalog.Spellcasting{
			Level:   w.SpellcastingLevel,
			Ability: c.ability(w.SpellcastingAbility),
			Info:    namedText(prose(w.Slug, b).Block(ProseSpellcasting)),
		}
	}
	return cl
}

// namedText carries the class spellcasting sections. The bundle stores them
// as alternating title and body paragraphs, which is the flattest encoding
// that survives a translator reordering nothing.
func namedText(blocks []string) []catalog.NamedText {
	out := make([]catalog.NamedText, 0, len(blocks)/2)
	for i := 0; i+1 < len(blocks); i += 2 {
		out = append(out, catalog.NamedText{Name: blocks[i], Desc: []string{blocks[i+1]}})
	}
	return out
}

func (c *conv) classLevel(w ClassLevel) catalog.ClassLevel {
	level := catalog.ClassLevel{
		Class:               rules.Slug(w.Class),
		Subclass:            rules.Slug(w.Subclass),
		Level:               w.Level,
		ProficiencyBonus:    w.ProficiencyBonus,
		AbilityScoreBonuses: w.AbilityScoreBonuses,
		Features:            slugs(w.Features),
		CantripsKnown:       w.CantripsKnown,
		SpellsKnown:         w.SpellsKnown,
	}
	for key, count := range w.SpellSlots {
		n := spellLevelIndex(key)
		if n < 1 || n > catalog.MaxSpellLevel {
			c.fail("spell slot level %q out of range for %s level %d", key, w.Class, w.Level)
			continue
		}
		level.SpellSlots[n] = count
	}
	for _, r := range w.Resources {
		res := catalog.LevelResource{Key: rules.Slug(r.Key), Number: r.Number, Text: r.Text}
		if r.Dice != "" {
			d := c.dice(r.Dice)
			res.Dice = &d
		}
		level.Resources = append(level.Resources, res)
	}
	return level
}

// spellLevelIndex parses a spell-slot map key. It returns -1 for anything
// that is not a small positive integer.
func spellLevelIndex(key string) int {
	n := 0
	for _, r := range key {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
		if n > catalog.MaxSpellLevel {
			return -1
		}
	}
	if key == "" {
		return -1
	}
	return n
}

func (c *conv) subclass(w Subclass, b Bundle) catalog.Subclass {
	sub := catalog.Subclass{
		Entry:  entry(w.Slug, b),
		Class:  rules.Slug(w.Class),
		Flavor: prose(w.Slug, b).Field(ProseFlavor),
		Levels: w.Levels,
	}
	for _, s := range w.Spells {
		sub.Spells = append(sub.Spells, catalog.SubclassSpell{Spell: rules.Slug(s.Spell), Level: s.Level})
	}
	return sub
}

func (c *conv) feature(w Feature, b Bundle) catalog.Feature {
	f := catalog.Feature{
		Entry:         entry(w.Slug, b),
		Class:         rules.Slug(w.Class),
		Subclass:      rules.Slug(w.Subclass),
		Level:         w.Level,
		Parent:        rules.Slug(w.Parent),
		Prerequisites: c.prerequisites(w.Prerequisites),
	}
	if w.ExpertiseOptions != nil || w.SubfeatureOptions != nil ||
		w.EnemyTypeOptions != nil || w.TerrainTypeOptions != nil || len(w.Invocations) > 0 {
		f.Specific = &catalog.FeatureSpecific{
			ExpertiseOptions:   c.choice(w.ExpertiseOptions),
			SubfeatureOptions:  c.choice(w.SubfeatureOptions),
			EnemyTypeOptions:   c.choice(w.EnemyTypeOptions),
			TerrainTypeOptions: c.choice(w.TerrainTypeOptions),
			Invocations:        slugs(w.Invocations),
		}
	}
	return f
}

func (c *conv) background(w Background, b Bundle) catalog.Background {
	bg := catalog.Background{
		Entry:                    entry(w.Slug, b),
		StartingProficiencies:    slugs(w.StartingProficiencies),
		LanguageOptions:          c.choice(w.LanguageOptions),
		StartingEquipment:        c.itemStacks(w.StartingEquipment),
		StartingEquipmentOptions: c.choices(w.StartingEquipmentOptions),
		Feature:                  rules.Slug(w.Feature),
	}
	if w.StartingGold != nil {
		bg.StartingGold = c.cost(*w.StartingGold)
	}
	if w.PersonalityTraits != nil {
		bg.PersonalityTraits = c.choiceValue(*w.PersonalityTraits)
	}
	if w.Ideals != nil {
		bg.Ideals = c.choiceValue(*w.Ideals)
	}
	if w.Bonds != nil {
		bg.Bonds = c.choiceValue(*w.Bonds)
	}
	if w.Flaws != nil {
		bg.Flaws = c.choiceValue(*w.Flaws)
	}
	return bg
}

func (c *conv) feat(w Feat, b Bundle) catalog.Feat {
	return catalog.Feat{Entry: entry(w.Slug, b), Prerequisites: c.prerequisites(w.Prerequisites)}
}

func (c *conv) item(w Item, b Bundle) catalog.Item {
	p := prose(w.Slug, b)
	it := catalog.Item{
		Entry:    entry(w.Slug, b),
		Category: rules.Slug(w.Category),
		Cost:     c.cost(w.Cost),
		Weight:   w.Weight,
	}
	if w.Weapon != nil {
		it.Weapon = c.weapon(*w.Weapon)
	}
	if w.Armor != nil {
		it.Armor = c.armor(*w.Armor)
	}
	if w.Gear != nil {
		it.Gear = &catalog.Gear{
			GearCategory: rules.Slug(w.Gear.GearCategory),
			Quantity:     w.Gear.Quantity,
			Contents:     c.itemStacks(w.Gear.Contents),
		}
	}
	if w.Tool != nil {
		it.Tool = &catalog.Tool{ToolCategory: rules.Slug(w.Tool.ToolCategory)}
	}
	if w.Vehicle != nil {
		it.Vehicle = &catalog.Vehicle{
			VehicleCategory: rules.Slug(w.Vehicle.VehicleCategory),
			Speed:           w.Vehicle.Speed,
			SpeedUnit:       p.Field(ProseSpeedUnit),
			Capacity:        p.Field(ProseCapacity),
		}
	}
	return it
}

func (c *conv) weapon(w Weapon) *catalog.Weapon {
	category, ok := weaponCategories[w.Category]
	if !ok && w.Category != "" {
		c.fail("unknown weapon category %q", w.Category)
	}
	weaponRange, ok := weaponRanges[w.Range]
	if !ok && w.Range != "" {
		c.fail("unknown weapon range %q", w.Range)
	}
	out := &catalog.Weapon{
		Category:    category,
		Range:       weaponRange,
		NormalRange: rules.Feet(w.NormalRange),
		LongRange:   rules.Feet(w.LongRange),
		ThrowNormal: rules.Feet(w.ThrowNormal),
		ThrowLong:   rules.Feet(w.ThrowLong),
		Properties:  slugs(w.Properties),
	}
	if w.Dice != "" {
		out.Damage = &rules.Damage{Dice: c.dice(w.Dice), Type: rules.Slug(w.DamageType)}
	}
	if w.TwoHandedDice != "" {
		out.TwoHandedDamage = &rules.Damage{Dice: c.dice(w.TwoHandedDice), Type: rules.Slug(w.TwoHandedDamageType)}
	}
	return out
}

func (c *conv) armor(w Armor) *catalog.Armor {
	category, ok := armorCategories[w.Category]
	if !ok && w.Category != "" {
		c.fail("unknown armor category %q", w.Category)
	}
	return &catalog.Armor{
		Category:            category,
		BaseAC:              w.BaseAC,
		AddsDexBonus:        w.AddsDexBonus,
		MaxDexBonus:         w.MaxDexBonus,
		StrengthMinimum:     w.StrengthMinimum,
		StealthDisadvantage: w.StealthDisadvantage,
	}
}

func (c *conv) magicItem(w MagicItem, b Bundle) catalog.MagicItem {
	rarity, ok := rarities[w.Rarity]
	if !ok && w.Rarity != "" {
		c.fail("unknown rarity %q", w.Rarity)
	}
	return catalog.MagicItem{
		Entry:     entry(w.Slug, b),
		Category:  rules.Slug(w.Category),
		Rarity:    rarity,
		Variants:  slugs(w.Variants),
		IsVariant: w.IsVariant,
	}
}

func (c *conv) spell(w Spell, b Bundle) catalog.Spell {
	p := prose(w.Slug, b)
	s := catalog.Spell{
		Entry:         entry(w.Slug, b),
		Source:        rules.Slug(w.Source),
		Level:         w.Level,
		School:        rules.Slug(w.School),
		CastingTime:   c.castingTime(w.CastingTime),
		Range:         c.spellRange(w.Range),
		Duration:      c.duration(w.Duration),
		Components:    catalog.Components(w.Components),
		Material:      p.Field(ProseMaterial),
		Ritual:        w.Ritual,
		Concentration: w.Concentration,
		Classes:       slugs(w.Classes),
		Subclasses:    slugs(w.Subclasses),
		HigherLevel:   p.Block(ProseHigherLevel),
	}
	if w.AttackType != "" {
		attack, ok := attackTypes[w.AttackType]
		if !ok {
			c.fail("unknown attack type %q", w.AttackType)
		}
		s.AttackType = attack
	}
	if w.Save != nil {
		effect, ok := saveEffects[w.Save.Success]
		if !ok && w.Save.Success != "" {
			c.fail("unknown save outcome %q", w.Save.Success)
		}
		s.Save = &catalog.SavingThrow{Ability: c.ability(w.Save.Ability), Success: effect}
	}
	if w.Damage != nil {
		s.Damage = &catalog.SpellDamage{
			Type:    rules.Slug(w.Damage.Type),
			Scaling: c.scaling(w.Damage.Scaling),
		}
	}
	if w.Heal != nil {
		heal := c.scaling(*w.Heal)
		s.Heal = &heal
	}
	if w.Area != nil {
		kind, ok := areaKinds[w.Area.Kind]
		if !ok {
			c.fail("unknown area kind %q", w.Area.Kind)
		}
		s.Area = &catalog.AreaOfEffect{Kind: kind, Size: rules.Feet(w.Area.Size)}
	}
	return s
}

func (c *conv) castingTime(w CastingTime) catalog.CastingTime {
	kind, ok := castingTimeKinds[w.Kind]
	if !ok {
		c.fail("unknown casting time kind %q", w.Kind)
	}
	return catalog.CastingTime{Kind: kind, Amount: w.Amount, Unit: c.timeUnit(w.Unit)}
}

func (c *conv) spellRange(w SpellRange) catalog.SpellRange {
	kind, ok := spellRangeKinds[w.Kind]
	if !ok {
		c.fail("unknown range kind %q", w.Kind)
	}
	return catalog.SpellRange{Kind: kind, Distance: rules.Feet(w.Distance)}
}

func (c *conv) duration(w Duration) catalog.Duration {
	kind, ok := durationKinds[w.Kind]
	if !ok {
		c.fail("unknown duration kind %q", w.Kind)
	}
	return catalog.Duration{Kind: kind, Amount: w.Amount, Unit: c.timeUnit(w.Unit), UpTo: w.UpTo}
}

func (c *conv) timeUnit(s string) catalog.TimeUnit {
	if s == "" {
		return catalog.TimeUnitNone
	}
	unit, ok := timeUnits[s]
	if !ok {
		c.fail("unknown time unit %q", s)
	}
	return unit
}

// scaling converts a spell's scaling table. The two maps stay distinct
// because a nil one means "does not scale that way", and collapsing them
// would make a cantrip look like a levelled spell with no slots.
func (c *conv) scaling(w SpellScaling) catalog.SpellScaling {
	return catalog.SpellScaling{
		AtSlotLevel:      c.diceMap(w.AtSlotLevel),
		AtCharacterLevel: c.diceMap(w.AtCharacterLevel),
	}
}

func (c *conv) diceMap(w map[string]string) map[int]rules.Dice {
	if len(w) == 0 {
		return nil
	}
	out := make(map[int]rules.Dice, len(w))
	for key, expr := range w {
		n := 0
		for _, r := range key {
			if r < '0' || r > '9' {
				n = -1
				break
			}
			n = n*10 + int(r-'0')
		}
		if n < 0 {
			c.fail("bad scaling level %q", key)
			continue
		}
		out[n] = c.dice(expr)
	}
	return out
}
