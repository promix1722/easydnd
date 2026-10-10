package character

import (
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// deriveAbilities applies racial bonuses and Ability Score Improvements on
// top of the base scores recorded by the init event.
func (p *projector) deriveAbilities() {
	if p.state.Abilities.Scores == nil {
		p.state.Abilities.Scores = make(map[rules.Ability]int)
	}
	for _, ability := range p.cat.AbilityIDs() {
		if _, ok := p.state.Abilities.Scores[ability]; !ok {
			p.state.Abilities.Scores[ability] = 10
		}
	}

	// Guarded rather than returned from. A missing race means no racial
	// bonus and nothing else: the improvements below are a class's business,
	// and an early return here quietly dropped them from every character
	// without one.
	if race, ok := p.cat.Races.Get(p.state.Identity.Race); ok {
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

// deriveStatus computes the numbers a player reads off constantly. The order
// is fixed because each stage feeds the next.
func (p *projector) deriveStatus() {
	level := p.state.Identity.Level()
	profBonus, err := p.cat.Mechanics.Core.Proficiency.Eval(rules.Variables{"level": level})
	if err != nil {
		p.err = err
	}
	p.state.Status.ProficiencyBonus = profBonus

	for skill, state := range p.state.Skills.BySkill {
		def, ok := p.cat.Skills.Get(skill)
		if !ok {
			continue
		}
		state.Bonus = p.state.Abilities.Modifier(def.Ability) + state.Proficiency.Apply(profBonus)
		p.state.Skills.BySkill[skill] = state
	}
	for _, ability := range p.cat.AbilityIDs() {
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
