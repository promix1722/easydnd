package character

import (
	"slices"

	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// applyChanges applies a batch of addressed mutations to the state.
//
// A path that does not resolve is an error naming the path and the event that
// carried it, rather than being silently dropped. That is what event.go's
// doc comment promises, and the reasoning is worth stating: a change is the
// escape hatch for a DM ruling or a homebrew adjustment, so a change that
// does nothing is a ruling the table believes is in effect and is not. An
// unreadable sheet is recoverable -- the client can truncate the log -- and a
// silently wrong one is not.
func (p *projector) applyChanges(changes []seqChange) error {
	for _, sc := range changes {
		if err := p.applyChange(sc); err != nil {
			return err
		}
	}
	return nil
}

func (p *projector) applyChange(sc seqChange) error {
	ch := sc.Change
	if ch.Op == OpNone {
		return types.NewValidationError("event %d: change to %q has no operator", sc.Seq, ch.Path)
	}

	segments := ch.Path.Segments()
	if len(segments) == 0 {
		return types.NewValidationError("event %d: change has no path", sc.Seq)
	}

	switch segments[0] {
	case "identity":
		return p.changeIdentity(sc, segments[1:])
	case "abilities":
		return p.changeAbilities(sc, segments[1:])
	case "base":
		return p.changeBase(sc, segments[1:])
	case "hitPoints":
		// hitPoints.* is accepted as a synonym for base.hitPoints.*: it is
		// the spelling event.go's own doc comment uses, and rejecting the
		// documented spelling would be a trap.
		return p.changeHitPoints(sc, segments[1:])
	case "status":
		return p.changeStatus(sc, segments[1:])
	case "equipment":
		return p.changeEquipment(sc, segments[1:])
	case "skills":
		return p.changeSkill(sc, segments[1:])
	case "savingThrows":
		return p.changeSavingThrow(sc, segments[1:])
	case "proficiencies":
		return p.changeSlugList(sc, &p.state.Proficiencies, segments[1:])
	case "conditions":
		return p.changeSlugList(sc, &p.state.Conditions, segments[1:])
	case "feats":
		return p.changeSlugList(sc, &p.state.Feats, segments[1:])
	case "traits":
		return p.changeSlugList(sc, &p.state.Traits, segments[1:])
	case "features":
		return p.changeSlugList(sc, &p.state.Features, segments[1:])
	}
	return p.unresolved(sc)
}

func (p *projector) unresolved(sc seqChange) error {
	return types.NewValidationError("event %d: %q does not address anything on the sheet",
		sc.Seq, sc.Change.Path)
}

func (p *projector) changeIdentity(sc seqChange, rest []string) error {
	if len(rest) != 1 {
		return p.unresolved(sc)
	}
	switch rest[0] {
	case "name":
		return setString(p, sc, &p.state.Identity.Name)
	case "alignment":
		return setSlug(p, sc, &p.state.Identity.Alignment)
	case "experience":
		return changeInt(p, sc, &p.state.Identity.Experience)
	case "personalityTraits":
		return changeStrings(p, sc, &p.state.Identity.PersonalityTraits)
	case "ideals":
		return changeStrings(p, sc, &p.state.Identity.Ideals)
	case "bonds":
		return changeStrings(p, sc, &p.state.Identity.Bonds)
	case "flaws":
		return changeStrings(p, sc, &p.state.Identity.Flaws)
	}
	return p.unresolved(sc)
}

func (p *projector) changeAbilities(sc seqChange, rest []string) error {
	if len(rest) != 1 {
		return p.unresolved(sc)
	}
	if rest[0] == "method" {
		return setSlug(p, sc, &p.state.Abilities.Method)
	}
	ability, ok := rules.ParseAbility(rest[0])
	if !ok {
		return p.unresolved(sc)
	}
	if p.state.Abilities.Scores == nil {
		p.state.Abilities.Scores = make(map[rules.Ability]int)
	}
	value, err := p.intValue(sc)
	if err != nil {
		return err
	}
	switch sc.Change.Op {
	case OpSet:
		p.state.Abilities.Scores[ability] = value
	case OpIncrement:
		p.state.Abilities.Scores[ability] += value
	default:
		return p.badOp(sc)
	}
	return nil
}

func (p *projector) changeBase(sc seqChange, rest []string) error {
	if len(rest) == 0 {
		return p.unresolved(sc)
	}
	if rest[0] == "hitPoints" {
		return p.changeHitPoints(sc, rest[1:])
	}
	if len(rest) != 1 {
		return p.unresolved(sc)
	}
	switch rest[0] {
	case "exhaustion":
		return changeInt(p, sc, &p.state.Base.Exhaustion)
	case "inspiration":
		return setBool(p, sc, &p.state.Base.Inspiration)
	case "languages":
		return p.changeSlugList(sc, &p.state.Base.Languages, nil)
	}
	return p.unresolved(sc)
}

func (p *projector) changeHitPoints(sc seqChange, rest []string) error {
	if len(rest) != 1 {
		return p.unresolved(sc)
	}
	switch rest[0] {
	case "max":
		if err := changeInt(p, sc, &p.state.Base.HitPoints.Max); err != nil {
			return err
		}
		// A raised maximum on a freshly projected sheet should not leave the
		// character standing at their old total; current is a projection of
		// max until damage is recorded against it.
		if p.state.Base.HitPoints.Current > p.state.Base.HitPoints.Max {
			p.state.Base.HitPoints.Current = p.state.Base.HitPoints.Max
		}
		return nil
	case "current":
		return changeInt(p, sc, &p.state.Base.HitPoints.Current)
	case "temporary":
		return changeInt(p, sc, &p.state.Base.HitPoints.Temporary)
	}
	return p.unresolved(sc)
}

func (p *projector) changeStatus(sc seqChange, rest []string) error {
	if len(rest) != 1 {
		return p.unresolved(sc)
	}
	switch rest[0] {
	case "armorClass":
		return changeInt(p, sc, &p.state.Status.ArmorClass)
	case "initiative":
		return changeInt(p, sc, &p.state.Status.Initiative)
	case "passivePerception":
		return changeInt(p, sc, &p.state.Status.PassivePerception)
	}
	return p.unresolved(sc)
}

// changeEquipment moves items between the character's three item lists.
//
// Equipping is the one that matters for the rules: armor class is derived
// from Equipped, and nothing in the catalogue says what a character wears, so
// "put on the leather armor" is necessarily an explicit event.
func (p *projector) changeEquipment(sc seqChange, rest []string) error {
	if len(rest) != 1 {
		return p.unresolved(sc)
	}
	var list *[]ItemStack
	switch rest[0] {
	case "equipped":
		list = &p.state.Equipment.Equipped
	case "backpack":
		list = &p.state.Equipment.Backpack
	case "loot":
		list = &p.state.Equipment.Loot
	default:
		return p.unresolved(sc)
	}

	value := sc.Change.Value
	switch sc.Change.Op {
	case OpAdd:
		for _, slug := range valueSlugs(value) {
			*list = append(*list, ItemStack{Item: slug, Count: 1})
		}
		return nil
	case OpRemove:
		for _, slug := range valueSlugs(value) {
			*list = slices.DeleteFunc(*list, func(s ItemStack) bool { return s.Item == slug })
		}
		return nil
	case OpSet:
		stacks := make([]ItemStack, 0, len(valueSlugs(value)))
		for _, slug := range valueSlugs(value) {
			stacks = append(stacks, ItemStack{Item: slug, Count: 1})
		}
		*list = stacks
		return nil
	}
	return p.badOp(sc)
}

// changeSkill sets how trained the character is in one skill:
// "skills.stealth" set to "expertise".
//
// It exists because a sheet imported from somewhere else states its
// proficiencies as facts rather than as answers to the prompts that granted
// them, and nothing else on State can carry that.
//
// The recomputation below is the subtle part. Skill changes are override tier,
// so they land after deriveStatus has already computed every Bonus; setting
// only the level would leave a sheet reading "Expertise" beside the bonus for
// plain proficiency. Recomputing here keeps the two consistent, and passive
// Perception with them, since it reads the Perception bonus.
func (p *projector) changeSkill(sc seqChange, rest []string) error {
	if len(rest) != 1 {
		return p.unresolved(sc)
	}
	skill := rules.Slug(rest[0])
	def, ok := p.cat.Skills.Get(skill)
	if !ok {
		return types.NewValidationError("event %d: %q is not a skill", sc.Seq, skill)
	}
	if sc.Change.Op != OpSet {
		return p.badOp(sc)
	}
	if sc.Change.Value.Kind != ValueString {
		return types.NewValidationError(
			"event %d: %q needs a proficiency level", sc.Seq, sc.Change.Path)
	}
	level, ok := rules.ParseProficiency(sc.Change.Value.Str)
	if !ok {
		return types.NewValidationError(
			"event %d: %q is not a proficiency level", sc.Seq, sc.Change.Value.Str)
	}

	if p.state.Skills.BySkill == nil {
		p.state.Skills.BySkill = make(map[rules.Slug]SkillState)
	}
	p.state.Skills.BySkill[skill] = SkillState{
		Proficiency: level,
		Bonus: p.state.Abilities.Modifier(def.Ability) +
			level.Apply(p.state.Status.ProficiencyBonus),
	}
	p.state.Status.PassivePerception = 10 + p.state.Skills.BySkill[perceptionSkill].Bonus
	return nil
}

// changeSavingThrow sets proficiency in one saving throw:
// "savingThrows.dex" set to true. It recomputes the bonus for the same reason
// changeSkill does.
func (p *projector) changeSavingThrow(sc seqChange, rest []string) error {
	if len(rest) != 1 {
		return p.unresolved(sc)
	}
	ability, ok := rules.ParseAbility(rest[0])
	if !ok {
		return p.unresolved(sc)
	}
	if sc.Change.Op != OpSet {
		return p.badOp(sc)
	}
	if sc.Change.Value.Kind != ValueBool {
		return types.NewValidationError("event %d: %q needs a boolean", sc.Seq, sc.Change.Path)
	}

	if p.state.SavingThrows.ByAbility == nil {
		p.state.SavingThrows.ByAbility = make(map[rules.Ability]SavingThrowState)
	}
	proficient := sc.Change.Value.Bool
	bonus := p.state.Abilities.Modifier(ability)
	if proficient {
		bonus += p.state.Status.ProficiencyBonus
	}
	p.state.SavingThrows.ByAbility[ability] = SavingThrowState{
		Proficient: proficient,
		Bonus:      bonus,
	}
	return nil
}

func (p *projector) changeSlugList(sc seqChange, list *[]rules.Slug, rest []string) error {
	if len(rest) != 0 {
		return p.unresolved(sc)
	}
	slugs := valueSlugs(sc.Change.Value)
	switch sc.Change.Op {
	case OpAdd:
		for _, slug := range slugs {
			if !slices.Contains(*list, slug) {
				*list = append(*list, slug)
			}
		}
		return nil
	case OpRemove:
		for _, slug := range slugs {
			*list = slices.DeleteFunc(*list, func(s rules.Slug) bool { return s == slug })
		}
		return nil
	case OpSet:
		*list = slices.Clone(slugs)
		return nil
	}
	return p.badOp(sc)
}

// valueSlugs reads a Value as a list of slugs, accepting either spelling so a
// caller adding one condition need not wrap it in a list.
func valueSlugs(v Value) []rules.Slug {
	switch v.Kind {
	case ValueSlug:
		if v.Slug.IsZero() {
			return nil
		}
		return []rules.Slug{v.Slug}
	case ValueSlugList:
		return v.Slugs
	case ValueString:
		if v.Str == "" {
			return nil
		}
		return []rules.Slug{rules.Slug(v.Str)}
	}
	return nil
}

func (p *projector) intValue(sc seqChange) (int, error) {
	if sc.Change.Value.Kind != ValueInt {
		return 0, types.NewValidationError("event %d: %q needs an integer, got %s",
			sc.Seq, sc.Change.Path, valueKindName(sc.Change.Value.Kind))
	}
	return sc.Change.Value.Int, nil
}

func (p *projector) badOp(sc seqChange) error {
	return types.NewValidationError("event %d: %q cannot be %s",
		sc.Seq, sc.Change.Path, sc.Change.Op)
}

func changeInt(p *projector, sc seqChange, target *int) error {
	value, err := p.intValue(sc)
	if err != nil {
		return err
	}
	switch sc.Change.Op {
	case OpSet:
		*target = value
	case OpIncrement:
		*target += value
	default:
		return p.badOp(sc)
	}
	return nil
}

func setString(p *projector, sc seqChange, target *string) error {
	if sc.Change.Op != OpSet {
		return p.badOp(sc)
	}
	if sc.Change.Value.Kind != ValueString {
		return types.NewValidationError("event %d: %q needs a string", sc.Seq, sc.Change.Path)
	}
	*target = sc.Change.Value.Str
	return nil
}

func setSlug(p *projector, sc seqChange, target *rules.Slug) error {
	if sc.Change.Op != OpSet {
		return p.badOp(sc)
	}
	switch sc.Change.Value.Kind {
	case ValueSlug:
		*target = sc.Change.Value.Slug
	case ValueString:
		*target = rules.Slug(sc.Change.Value.Str)
	default:
		return types.NewValidationError("event %d: %q needs a slug", sc.Seq, sc.Change.Path)
	}
	return nil
}

func setBool(p *projector, sc seqChange, target *bool) error {
	if sc.Change.Op != OpSet {
		return p.badOp(sc)
	}
	if sc.Change.Value.Kind != ValueBool {
		return types.NewValidationError("event %d: %q needs a boolean", sc.Seq, sc.Change.Path)
	}
	*target = sc.Change.Value.Bool
	return nil
}

func changeStrings(p *projector, sc seqChange, target *[]string) error {
	if sc.Change.Value.Kind != ValueString {
		return types.NewValidationError("event %d: %q needs a string", sc.Seq, sc.Change.Path)
	}
	switch sc.Change.Op {
	case OpSet:
		*target = []string{sc.Change.Value.Str}
	case OpAdd:
		*target = append(*target, sc.Change.Value.Str)
	case OpRemove:
		*target = slices.DeleteFunc(*target, func(s string) bool { return s == sc.Change.Value.Str })
	default:
		return p.badOp(sc)
	}
	return nil
}

// valueKindName is for error messages only.
func valueKindName(k ValueKind) string {
	switch k {
	case ValueNone:
		return "nothing"
	case ValueInt:
		return "an integer"
	case ValueString:
		return "a string"
	case ValueBool:
		return "a boolean"
	case ValueSlug:
		return "a slug"
	case ValueSlugList:
		return "a slug list"
	case ValueDice:
		return "dice"
	}
	return "an unknown value"
}
