package character

import (
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// Project folds a log into a State against a catalogue.
//
// It is pure: the same log and the same catalogue always produce the same
// state, with no clock, no randomness and no I/O. That is what makes the
// event-sourced design worth its cost -- a character can be re-derived at any
// time, and a projection bug is fixed by shipping new code rather than by
// migrating stored data.
//
// The order of derivation matters and is fixed:
//
//  1. replay events in sequence, accumulating chosen entries and raw scores
//  2. resolve the catalogue entries those choices name
//  3. compute the derived values -- proficiency bonus, then ability
//     modifiers, then skills and saving throws, then armor class, initiative
//     and the spellcasting summaries
//  4. assemble actions, deriving from equipment and prepared spells before
//     appending the ones stored manually
//
// Step 3 is ordered because each stage feeds the next: proficiency bonus
// depends on character level, saving throws depend on the proficiency bonus,
// and the spell save DC depends on both.
//
// Step 4 is not implemented. Deriving an attack from an equipped weapon and a
// casting from a prepared spell is the battle tracker's groundwork, not
// character creation's, and State.Actions carries only what a change event
// put there.
//
// # Base scores versus final scores
//
// An init event records the scores as *generated* -- the point buy, the
// standard array, the dice. Racial bonuses and Ability Score Improvements are
// applied here, every time. That is the only arrangement under which choosing
// a different race changes the sheet, which is the entire point of projecting
// rather than storing; the alternative would make the player re-enter six
// numbers whenever they went back a step.
func Project(log Log, cat *catalog.Catalog) (State, error) {
	if cat == nil {
		return State{}, types.NewValidationError("projecting against a nil catalogue")
	}
	if err := log.Validate(); err != nil {
		return State{}, err
	}

	p := projector{cat: cat, answers: foldAnswers(log)}
	return p.run(log)
}

// projector carries the working state of one projection.
type projector struct {
	cat     *catalog.Catalog
	answers answers

	state State

	// Changes fall into three tiers, and the tier decides when they are
	// applied. Getting this wrong is silent: the change lands, and the number
	// it was meant to affect was computed before or after it.
	//
	//   inputs    seed derivation -- ability scores, name, alignment. They
	//             must land before anything reads them, and the hit point
	//             maximum reads Constitution.
	//   equipment moves items between the carried lists. It lands after the
	//             starting kit has been granted, because you cannot equip
	//             armor you have not been given, and before armor class is
	//             derived, because armor class reads what is equipped.
	//   overrides are everything else, applied last so that a DM's ruling
	//             wins over the rules rather than being recomputed away.
	inputs    []seqChange
	equipment []seqChange
	overrides []seqChange

	// proficiencies is every proficiency slug granted, in grant order,
	// before it is sorted into skills, saving throws and the rest.
	proficiencies []rules.Slug

	// expertise is the skills doubled by a feature.
	expertise []rules.Slug
}

// seqChange is a change together with the event that carried it, so an error
// can say where a bad path came from.
type seqChange struct {
	Seq    int
	Change Change
}

func (p *projector) run(log Log) (State, error) {
	p.replay(log)

	if err := p.applyChanges(p.inputs); err != nil {
		return State{}, err
	}
	// Ability scores are finalised before anything that reads them. The hit
	// point maximum is the reason this cannot wait: it is Constitution
	// modifier per level, and a half-elf who put their +1 into Constitution
	// would otherwise be three hit points short at 3rd level.
	p.deriveAbilities()

	p.applyRace()
	p.applyBackground()
	p.advanceToDesiredLevel()
	p.applyClasses()
	p.applyEquipmentChoices()

	if err := p.applyChanges(p.equipment); err != nil {
		return State{}, err
	}

	p.deriveProficiencies()
	p.deriveStatus()

	if err := p.applyChanges(p.overrides); err != nil {
		return State{}, err
	}
	return p.state, nil
}

// replay walks the log in order, recording what was chosen. Nothing is
// derived here: this stage only says which catalogue entries the character
// named and which changes were requested.
func (p *projector) replay(log Log) {
	for _, e := range log.Events {
		switch e.Type {
		case EventInit, EventChange:
			for _, ch := range e.Changes {
				sc := seqChange{Seq: e.Seq, Change: ch}
				switch {
				case isInputPath(ch.Path):
					p.inputs = append(p.inputs, sc)
				case isEquipmentPath(ch.Path):
					p.equipment = append(p.equipment, sc)
				default:
					p.overrides = append(p.overrides, sc)
				}
			}
		case EventRace:
			p.state.Identity.Race = e.Ref.Slug
		case EventSubrace:
			p.state.Identity.Subrace = e.Ref.Slug
		case EventBackground:
			p.state.Identity.Background = e.Ref.Slug
		case EventClass:
			p.takeLevel(e.Ref.Slug, max(e.Level, 1))
		case EventLevel:
			p.takeLevel(e.Ref.Slug, e.Level)
		case EventSubclass:
			p.setSubclass(e.Ref.Slug)
		case EventFeat:
			if !e.Ref.Slug.IsZero() && !slices.Contains(p.state.Feats, e.Ref.Slug) {
				p.state.Feats = append(p.state.Feats, e.Ref.Slug)
			}
		case EventNote, EventNone:
		}
	}
}

// takeLevel records that the character now has a level in a class. Levels are
// idempotent by number rather than cumulative, so replaying a log twice
// cannot inflate them.
func (p *projector) takeLevel(class rules.Slug, level int) {
	if class.IsZero() || level < 1 {
		return
	}
	for i, c := range p.state.Identity.Classes {
		if c.Class == class {
			if level > c.Level {
				p.state.Identity.Classes[i].Level = level
			}
			return
		}
	}
	p.state.Identity.Classes = append(p.state.Identity.Classes, ClassLevel{Class: class, Level: level})
}

// advanceToDesiredLevel raises a single-class character to the level they
// declared they are building towards.
//
// Levels used to be taken one event at a time, each one answering "which class
// does this go into?" -- a question with one answer while multiclassing is
// off, asked eight times on the way to ninth level. There is nothing to
// record, so nothing records it: the declaration *is* the level, and
// applyClasses grants what that level grants. What the player is actually
// asked is what those levels open -- the archetype, the improvements -- which
// Prompts derives from the same number.
//
// Raise only. Going back down is not something the rules do, and takeLevel is
// already max-by-number for the same reason.
//
// With two classes this does nothing, because then the question is real again.
// That is the whole of what turning multiclassing back on has to undo.
func (p *projector) advanceToDesiredLevel() {
	if len(p.state.Identity.Classes) != 1 {
		return
	}
	if p.state.Identity.DesiredLevel > p.state.Identity.Classes[0].Level {
		p.state.Identity.Classes[0].Level = p.state.Identity.DesiredLevel
	}
}

// setSubclass attaches a subclass to whichever class offers it.
func (p *projector) setSubclass(subclass rules.Slug) {
	entry, ok := p.cat.Subclasses.Get(subclass)
	if !ok {
		return
	}
	for i, c := range p.state.Identity.Classes {
		if c.Class == entry.Class {
			p.state.Identity.Classes[i].Subclass = subclass
			return
		}
	}
}

// isInputPath reports whether a change seeds derivation rather than
// overriding it.
//
// The split is what makes both kinds of change work. Ability scores and the
// character's name are inputs: nothing derives them, and everything else
// derives *from* them, so they must land before the rules run. A hit point
// maximum set by a DM is an override: the rules would otherwise recompute it
// and the ruling would vanish.
func isInputPath(path Path) bool {
	switch path {
	case "identity.name", "identity.alignment", "identity.desiredLevel",
		"identity.ruleset", "abilities.method":
		return true
	}
	segments := path.Segments()
	return len(segments) == 2 && segments[0] == "abilities"
}

// isEquipmentPath reports whether a change moves items between the carried
// lists, which has to happen between the starting kit being granted and armor
// class being derived from what is worn.
func isEquipmentPath(path Path) bool {
	segments := path.Segments()
	return len(segments) == 2 && segments[0] == "equipment"
}

// applyRace resolves the race and subrace: bonuses, speed, size, languages,
// traits and the proficiencies they grant.
func (p *projector) applyRace() {
	race, ok := p.cat.Races.Get(p.state.Identity.Race)
	if !ok {
		return
	}
	p.state.Base.Size = race.Size
	if race.Speed > 0 {
		p.state.Base.Speeds = append(p.state.Base.Speeds, Speed{Kind: Walking, Distance: race.Speed})
	}
	p.addLanguages(race.Languages...)
	p.addLanguages(p.answers.slugs(race.LanguageOptions)...)
	p.proficiencies = append(p.proficiencies, race.StartingProficiencies...)
	for _, choice := range race.ProficiencyOptions {
		p.proficiencies = append(p.proficiencies, p.answers.slugs(&choice)...)
	}
	p.state.Traits = append(p.state.Traits, race.Traits...)

	if subrace, ok := p.cat.Subraces.Get(p.state.Identity.Subrace); ok {
		p.state.Traits = append(p.state.Traits, subrace.Traits...)
		p.proficiencies = append(p.proficiencies, subrace.StartingProficiencies...)
		p.addLanguages(p.answers.slugs(subrace.LanguageOptions)...)
	}

	// Traits carry prompts of their own -- the half-elf's two skills come
	// from Skill Versatility, not from the race entry.
	for _, slug := range p.state.Traits {
		trait, ok := p.cat.Traits.Get(slug)
		if !ok {
			continue
		}
		p.proficiencies = append(p.proficiencies, trait.Proficiencies...)
		p.proficiencies = append(p.proficiencies, p.answers.slugs(trait.ProficiencyOptions)...)
	}
	p.state.Base.Senses = sensesFor(p.state.Traits)
}

// applyBackground resolves the background's proficiencies, languages,
// equipment and roleplaying picks.
func (p *projector) applyBackground() {
	background, ok := p.cat.Backgrounds.Get(p.state.Identity.Background)
	if !ok {
		return
	}
	p.proficiencies = append(p.proficiencies, background.StartingProficiencies...)
	p.addLanguages(p.answers.slugs(background.LanguageOptions)...)
	p.addStacks(background.StartingEquipment)
	if background.StartingGold.Amount > 0 {
		p.addCoins(background.StartingGold)
	}
	if !background.Feature.IsZero() {
		p.state.Features = append(p.state.Features, background.Feature)
	}

	// Nothing here writes the roleplaying lines. They are the player's own
	// words now, arriving as changes to identity.personalityTraits and its
	// three siblings, and this used to *assign* them from the picked
	// suggestion -- so choosing a background after writing them would have
	// wiped what was written.
}

// applyClasses resolves every class the character has levels in: hit points,
// Hit Dice, proficiencies, features and starting equipment.
func (p *projector) applyClasses() {
	for i, taken := range p.state.Identity.Classes {
		class, ok := p.cat.Classes.Get(taken.Class)
		if !ok {
			continue
		}
		first := i == 0
		g := classGrant(class, 1, first)
		p.proficiencies = append(p.proficiencies, g.Proficiencies...)
		for _, choice := range g.Choices {
			p.proficiencies = append(p.proficiencies, p.answers.slugs(&choice)...)
		}
		p.addStacks(g.Equipment)

		// The first class grants its saving throws; a class taken later does
		// not, which is the 2014 multiclassing rule.
		if first {
			for _, ability := range class.SavingThrows {
				p.setSavingThrow(ability)
			}
		}

		p.addHitPoints(class.HitDie, taken.Level, first)
		p.addHitDice(class.HitDie, taken.Level)

		features := featuresThrough(p.cat, taken.Class, taken.Level)
		if !taken.Subclass.IsZero() {
			features = append(features, featuresThrough(p.cat, taken.Subclass, taken.Level)...)
		}
		p.state.Features = append(p.state.Features, features...)
		p.applyFeaturePrompts(features)

		if row, ok := p.cat.ClassLevel(taken.Class, taken.Level); ok {
			p.addClassResources(row)
		}
	}
}

// applyFeaturePrompts collects what the answered prompts on a feature grant.
// Expertise is the one that changes a number rather than adding to a list.
func (p *projector) applyFeaturePrompts(features []rules.Slug) {
	for _, slug := range features {
		feature, ok := p.cat.Features.Get(slug)
		if !ok || feature.Specific == nil {
			continue
		}
		if feature.Specific.ExpertiseOptions != nil {
			p.expertise = append(p.expertise, p.answers.slugs(feature.Specific.ExpertiseOptions)...)
		}
		if feature.Specific.SubfeatureOptions != nil {
			p.state.Features = append(p.state.Features, p.answers.slugs(feature.Specific.SubfeatureOptions)...)
		}
	}
}

// addHitPoints adds the hit points a class's levels contribute.
//
// The first level of the character's first class takes the full hit die; every
// level after that takes the SRD's fixed average, which for a die of size d is
// d/2 + 1. Both are increased by the Constitution modifier, and the modifier
// is read after the input changes and racial bonuses have landed.
//
// Rolling is deliberately not an option here: Project is documented as having
// no randomness, and a projection that rolled would give a different sheet on
// every read. A rolled hit point total is recorded as a change event, which
// overrides this.
func (p *projector) addHitPoints(hitDie, level int, first bool) {
	if hitDie <= 0 || level < 1 {
		return
	}
	conModifier := p.state.Abilities.Modifier(rules.Constitution)
	average := hitDie/2 + 1

	gained := 0
	if first {
		gained += hitDie + conModifier
		gained += (level - 1) * (average + conModifier)
	} else {
		gained += level * (average + conModifier)
	}
	p.state.Base.HitPoints.Max += gained
}

func (p *projector) addHitDice(hitDie, level int) {
	if hitDie <= 0 || level < 1 {
		return
	}
	dice := rules.Dice{Terms: []rules.DiceTerm{{Count: level, Faces: hitDie}}}
	p.state.Resources.HitDice = append(p.state.Resources.HitDice, Pool{
		Max:      level,
		Recharge: OnLongRest,
		Dice:     &dice,
	})
}

func (p *projector) addClassResources(row catalog.ClassLevel) {
	for _, resource := range row.Resources {
		pool := Pool{Key: resource.Key, Max: resource.Number}
		if resource.Dice != nil {
			dice := *resource.Dice
			pool.Dice = &dice
		}
		p.state.Resources.Class = append(p.state.Resources.Class, pool)
	}
}

// applyEquipmentChoices resolves the starting-equipment prompts into stacks.
//
// Everything lands in the backpack. Nothing in the catalogue says what a
// character is wearing, and guessing -- strapping on the shield that came in
// the same bundle as a two-handed weapon -- would produce an armor class with
// no rule behind it. Equipping is an explicit change event.
func (p *projector) applyEquipmentChoices() {
	class, ok := p.cat.Classes.Get(p.firstClass())
	if ok {
		for _, choice := range class.StartingEquipmentOptions {
			p.addChosenEquipment(choice)
		}
	}
	if background, ok := p.cat.Backgrounds.Get(p.state.Identity.Background); ok {
		for _, choice := range background.StartingEquipmentOptions {
			p.addChosenEquipment(choice)
		}
	}
}

func (p *projector) addChosenEquipment(choice rules.Choice) {
	p.answers.chosen(choice, func(o rules.Option) {
		switch opt := o.(type) {
		case rules.RefOption:
			if opt.Ref.Kind == rules.RefItem || opt.Ref.Kind == rules.RefMagicItem {
				p.addStacks([]catalog.ItemStack{{Item: opt.Ref.Slug, Count: max(opt.Count, 1)}})
			}
		case rules.MoneyOption:
			p.addCoins(opt.Coins)
		}
	})
}

func (p *projector) firstClass() rules.Slug {
	if len(p.state.Identity.Classes) == 0 {
		return ""
	}
	return p.state.Identity.Classes[0].Class
}

// deriveAbilities applies racial bonuses and Ability Score Improvements on
// top of the base scores recorded by the init event.
func (p *projector) deriveAbilities() {
	if p.state.Abilities.Scores == nil {
		p.state.Abilities.Scores = make(map[rules.Ability]int)
	}
	for _, ability := range rules.Abilities() {
		if _, ok := p.state.Abilities.Scores[ability]; !ok {
			p.state.Abilities.Scores[ability] = 10
		}
	}

	race, ok := p.cat.Races.Get(p.state.Identity.Race)
	if !ok {
		return
	}
	for _, bonus := range race.AbilityBonuses {
		p.state.Abilities.Scores[bonus.Ability] += bonus.Bonus
	}
	if race.AbilityBonusOptions != nil {
		p.answers.chosen(*race.AbilityBonusOptions, func(o rules.Option) {
			if bonus, ok := o.(rules.AbilityBonusOption); ok {
				p.state.Abilities.Scores[bonus.Ability] += bonus.Bonus
			}
		})
	}
	if subrace, ok := p.cat.Subraces.Get(p.state.Identity.Subrace); ok {
		for _, bonus := range subrace.AbilityBonuses {
			p.state.Abilities.Scores[bonus.Ability] += bonus.Bonus
		}
	}
	p.applyAbilityScoreImprovements()
}

// applyAbilityScoreImprovements applies the "+2 to one ability, +1 to two, or
// a feat" choice at every level that grants one.
//
// The improvement is not in the SRD data -- the feature row is bare, and only
// the cumulative AbilityScoreBonuses count marks the level -- so the prompt is
// synthesised, and it must be synthesised identically here and in Prompts.
// asiPrompt is shared between them for exactly that reason.
//
// Picking the same ability twice is how "+2 to one" is expressed, which is
// why the bonuses are summed rather than deduplicated.
func (p *projector) applyAbilityScoreImprovements() {
	for _, taken := range p.state.Identity.Classes {
		for level := 1; level <= taken.Level; level++ {
			if !grantsAbilityScoreImprovement(p.cat, taken.Class, level) {
				continue
			}
			prompt := asiPrompt(taken.Class, level)
			for _, key := range p.answers.picks(prompt + "/0") {
				ability, ok := rules.ParseAbility(key.String())
				if !ok {
					continue
				}
				p.state.Abilities.Scores[ability]++
			}
			for _, feat := range p.answers.picks(prompt + "/1") {
				if !feat.IsZero() && !slices.Contains(p.state.Feats, feat) {
					p.state.Feats = append(p.state.Feats, feat)
				}
			}
		}
	}
}

// deriveProficiencies sorts every granted proficiency into the place that can
// use it: a skill, a saving throw, or the sheet's "other proficiencies" list.
func (p *projector) deriveProficiencies() {
	p.seedSkills()
	for _, slug := range p.proficiencies {
		def, ok := p.cat.Proficiencies.Get(slug)
		if !ok {
			p.addOtherProficiency(slug)
			continue
		}
		switch {
		case def.Type == catalog.ProficiencySavingThrows:
			p.setSavingThrow(abilityOfRef(def.Reference))
		case def.Reference.Kind == rules.RefSkill:
			p.setSkill(def.Reference.Slug, rules.Proficient)
		default:
			p.addOtherProficiency(slug)
		}
	}
	// Expertise is applied after plain proficiency, because doubling a
	// proficiency bonus the character does not have would be wrong.
	for _, slug := range p.expertise {
		skill := slug
		if def, ok := p.cat.Proficiencies.Get(slug); ok && def.Reference.Kind == rules.RefSkill {
			skill = def.Reference.Slug
		}
		// Presence in the map is not the question -- seedSkills put every
		// skill in it. Expertise doubles a proficiency bonus, so there has to
		// be one, and the level is the only thing that says so.
		//
		// Phrased as holds() in prompts.go phrases the same question, down to
		// the operator: that is what decides which skills the prompt offers,
		// and a projector that then declined one of them would be a second
		// opinion about the same rule.
		if p.state.Skills.BySkill[skill].Proficiency != rules.NotProficient {
			p.setSkill(skill, rules.Expertise)
		}
	}
}

func abilityOfRef(ref rules.Ref) rules.Ability {
	ability, _ := rules.ParseAbility(ref.Slug.String())
	return ability
}

// seedSkills puts every skill in the compendium on the sheet, untrained.
//
// A sheet that lists only the skills something trained is the wrong sheet to
// read at a table: the question asked most often is what to roll for a skill
// the character has *no* training in, and that is exactly the row such a sheet
// omits. Seeding here rather than filling the gaps in each client keeps the
// rules in one place -- a browser adding the ability modifier itself would be
// a second implementation to disagree, and it would be wrong the day Jack of
// All Trades starts halving a bonus.
//
// The zero SkillState is NotProficient with no bonus; deriveStatus computes
// every Bonus afterwards, so an untrained skill ends up at the bare ability
// modifier. This runs before any grant because setSkill only ever raises a
// level, so seeding cannot lower one.
func (p *projector) seedSkills() {
	if p.state.Skills.BySkill == nil {
		p.state.Skills.BySkill = make(map[rules.Slug]SkillState, p.cat.Skills.Len())
	}
	for _, slug := range p.cat.Skills.Slugs() {
		if _, known := p.state.Skills.BySkill[slug]; !known {
			p.state.Skills.BySkill[slug] = SkillState{}
		}
	}
}

func (p *projector) setSkill(skill rules.Slug, level rules.Proficiency) {
	current := p.state.Skills.BySkill[skill]
	if level > current.Proficiency {
		current.Proficiency = level
	}
	p.state.Skills.BySkill[skill] = current
}

func (p *projector) setSavingThrow(ability rules.Ability) {
	if ability == rules.AbilityNone {
		return
	}
	if p.state.SavingThrows.ByAbility == nil {
		p.state.SavingThrows.ByAbility = make(map[rules.Ability]SavingThrowState)
	}
	state := p.state.SavingThrows.ByAbility[ability]
	state.Proficient = true
	p.state.SavingThrows.ByAbility[ability] = state
}

func (p *projector) addOtherProficiency(slug rules.Slug) {
	if slug.IsZero() || slices.Contains(p.state.Proficiencies, slug) {
		return
	}
	p.state.Proficiencies = append(p.state.Proficiencies, slug)
}

func (p *projector) addLanguages(languages ...rules.Slug) {
	for _, language := range languages {
		if language.IsZero() || slices.Contains(p.state.Base.Languages, language) {
			continue
		}
		p.state.Base.Languages = append(p.state.Base.Languages, language)
	}
}

func (p *projector) addStacks(stacks []catalog.ItemStack) {
	for _, stack := range stacks {
		if stack.Item.IsZero() {
			continue
		}
		p.state.Equipment.Backpack = append(p.state.Equipment.Backpack, ItemStack{
			Item:  stack.Item,
			Count: max(stack.Count, 1),
		})
	}
}

func (p *projector) addCoins(coins rules.Coins) {
	if coins.Amount == 0 {
		return
	}
	if p.state.Equipment.Purse == nil {
		p.state.Equipment.Purse = make(rules.Purse)
	}
	p.state.Equipment.Purse[coins.Unit] += coins.Amount
}

// deriveStatus computes the numbers a player reads off constantly. The order
// is fixed because each stage feeds the next.
func (p *projector) deriveStatus() {
	level := p.state.Identity.Level()
	profBonus := proficiencyBonus(level)
	p.state.Status.ProficiencyBonus = profBonus

	for skill, state := range p.state.Skills.BySkill {
		def, ok := p.cat.Skills.Get(skill)
		if !ok {
			continue
		}
		state.Bonus = p.state.Abilities.Modifier(def.Ability) + state.Proficiency.Apply(profBonus)
		p.state.Skills.BySkill[skill] = state
	}
	for _, ability := range rules.Abilities() {
		state := p.state.SavingThrows.ByAbility[ability]
		state.Bonus = p.state.Abilities.Modifier(ability)
		if state.Proficient {
			state.Bonus += profBonus
		}
		if p.state.SavingThrows.ByAbility == nil {
			p.state.SavingThrows.ByAbility = make(map[rules.Ability]SavingThrowState)
		}
		p.state.SavingThrows.ByAbility[ability] = state
	}

	dexModifier := p.state.Abilities.Modifier(rules.Dexterity)
	p.state.Status.ArmorClass = armorClass(p.state.Equipment.Equipped, p.cat, dexModifier)
	p.state.Status.Initiative = dexModifier
	// Reads the Perception bonus rather than recomputing it, which is what
	// makes this right for a character nothing trained in Perception: the
	// skill is on the sheet either way now, carrying the bare Wisdom
	// modifier. It used to read a missing key and quietly drop that modifier,
	// so every untrained character had passive Perception exactly 10.
	p.state.Status.PassivePerception = 10 + p.state.Skills.BySkill[perceptionSkill].Bonus

	slots, pact := spellSlots(p.cat, p.state.Identity.Classes)
	p.state.Resources.SpellSlots = slots
	p.state.Resources.Class = append(p.state.Resources.Class, pact...)

	p.state.Status.Spellcasting = spellcastingSummaries(
		p.cat, p.state.Identity.Classes, p.state.Abilities, profBonus)
	if len(p.state.Status.Spellcasting) > 0 {
		p.state.Spells.Ability = p.state.Status.Spellcasting[0].Ability
	}

	p.state.Base.HitPoints.Current = p.state.Base.HitPoints.Max
}

// perceptionSkill is the skill passive Perception reads.
const perceptionSkill rules.Slug = "perception"

// proficiencyBonus is the number added to anything the character is
// proficient in, derived from character level.
//
// It is computed rather than read from ClassLevel.ProficiencyBonus because a
// multiclassed character has no single class level to read it at: a cleric
// 3 / wizard 3 has proficiency bonus +3, the bonus of a 6th-level character,
// not the +2 either class row would report. The formula and the compendium
// agree for every single-class case, which TestProficiencyBonusMatchesTheData
// pins.
func proficiencyBonus(characterLevel int) int {
	if characterLevel < 1 {
		return 2
	}
	return 2 + (characterLevel-1)/4
}
