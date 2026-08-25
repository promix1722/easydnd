package character

import (
	"strconv"
	"time"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// Outbound: domain to response.

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

// sourceString names the prompt group an entry answers, or nothing at all
// where the server could not attribute it -- an imported log, a DM's change.
// PromptGroupNone stringifies as "none", which would read on the wire as a
// group rather than as an absence.
func sourceString(group domain.PromptGroup) string {
	if group == domain.PromptGroupNone {
		return ""
	}
	return group.String()
}

func refString(ref rules.Ref) string {
	if ref.IsZero() {
		return ""
	}
	return ref.String()
}

func classLevels(classes []domain.ClassLevel) []ClassLevel {
	if len(classes) == 0 {
		return nil
	}
	out := make([]ClassLevel, 0, len(classes))
	for _, c := range classes {
		out = append(out, ClassLevel{
			Class: c.Class.String(), Subclass: c.Subclass.String(), Level: c.Level,
		})
	}
	return out
}

func summaryOf(s domain.Summary) Summary {
	return Summary{
		ID:      s.ID.String(),
		Folder:  s.Folder.String(),
		Name:    s.Name,
		Level:   s.Level,
		Classes: classLevels(s.Classes),
	}
}

func characterOf(c domain.Character) Character {
	events := make([]Event, 0, c.Log.Len())
	for _, e := range c.Log.Events {
		events = append(events, eventOf(e))
	}
	return Character{ID: c.ID.String(), Seq: c.Log.LastSeq(), Events: events}
}

func eventOf(e domain.Event) Event {
	out := Event{
		Seq:    e.Seq,
		Type:   e.Type.String(),
		Source: sourceString(e.Source),
		Ref:    refString(e.Ref),
		Level:  e.Level,
		Note:   e.Note,
	}
	if !e.At.IsZero() {
		out.At = e.At.UTC().Format(time.RFC3339)
	}
	for _, a := range e.Choices {
		out.Choices = append(out.Choices, Answer{
			Prompt: a.Prompt.String(), Picks: slugStrings(a.Picks),
		})
	}
	for _, ch := range e.Changes {
		out.Changes = append(out.Changes, Change{
			Path: ch.Path.String(), Op: ch.Op.String(), Value: valueOf(ch.Value),
		})
	}
	return out
}

func valueOf(v domain.Value) Value {
	out := Value{Kind: valueKindName(v.Kind)}
	switch v.Kind {
	case domain.ValueInt:
		out.Int = v.Int
	case domain.ValueString:
		out.String = v.Str
	case domain.ValueBool:
		out.Bool = v.Bool
	case domain.ValueSlug:
		out.Slug = v.Slug.String()
	case domain.ValueSlugList:
		out.Slugs = slugStrings(v.Slugs)
	case domain.ValueDice:
		out.Dice = v.Dice.String()
	}
	return out
}

func valueKindName(k domain.ValueKind) string {
	switch k {
	case domain.ValueInt:
		return "int"
	case domain.ValueString:
		return "string"
	case domain.ValueBool:
		return "bool"
	case domain.ValueSlug:
		return "slug"
	case domain.ValueSlugList:
		return "slugs"
	case domain.ValueDice:
		return "dice"
	}
	return "none"
}

func sheetOf(s domain.State) Sheet {
	out := Sheet{
		Identity:      identityOf(s.Identity),
		Base:          baseOf(s.Base),
		Abilities:     abilitiesOf(s.Abilities),
		Skills:        skillsOf(s.Skills),
		SavingThrows:  savesOf(s.SavingThrows),
		Status:        statusOf(s.Status),
		Equipment:     equipmentOf(s.Equipment),
		Resources:     resourcesOf(s.Resources),
		Spells:        spellbookOf(s.Spells),
		Actions:       actionsOf(s.Actions),
		Feats:         slugStrings(s.Feats),
		Traits:        slugStrings(s.Traits),
		Features:      slugStrings(s.Features),
		Conditions:    slugStrings(s.Conditions),
		Proficiencies: slugStrings(s.Proficiencies),
	}
	return out
}

func identityOf(i domain.Identity) Identity {
	return Identity{
		Name:              i.Name,
		Alignment:         i.Alignment.String(),
		Race:              i.Race.String(),
		Subrace:           i.Subrace.String(),
		Background:        i.Background.String(),
		Classes:           classLevels(i.Classes),
		Level:             i.Level(),
		Experience:        i.Experience,
		PersonalityTraits: i.PersonalityTraits,
		Ideals:            i.Ideals,
		Bonds:             i.Bonds,
		Flaws:             i.Flaws,
	}
}

func baseOf(b domain.Base) Base {
	out := Base{
		HitPoints: HitPoints{
			Current: b.HitPoints.Current, Max: b.HitPoints.Max, Temporary: b.HitPoints.Temporary,
		},
		Size:        b.Size.String(),
		Languages:   slugStrings(b.Languages),
		Exhaustion:  b.Exhaustion,
		DeathSaves:  DeathSaves{Successes: b.DeathSaves.Successes, Failures: b.DeathSaves.Failures},
		Inspiration: b.Inspiration,
	}
	for _, s := range b.Speeds {
		out.Speeds = append(out.Speeds, Speed{Kind: s.Kind.String(), Distance: int(s.Distance)})
	}
	for _, s := range b.Senses {
		out.Senses = append(out.Senses, Sense{Kind: s.Kind.String(), Distance: int(s.Distance)})
	}
	return out
}

// abilitiesOf serves the modifiers alongside the scores. The domain never
// stores a modifier -- it is a pure function of the score -- but every client
// would otherwise reimplement it, and one of them would round negatives the
// wrong way.
func abilitiesOf(a domain.Abilities) Abilities {
	scores := make(map[string]int, len(rules.Abilities()))
	modifiers := make(map[string]int, len(rules.Abilities()))
	for _, ability := range rules.Abilities() {
		key := ability.Slug().String()
		scores[key] = a.Score(ability)
		modifiers[key] = a.Modifier(ability)
	}
	return Abilities{Scores: scores, Modifiers: modifiers, Method: a.Method.String()}
}

func skillsOf(s domain.Skills) map[string]Skill {
	out := make(map[string]Skill, len(s.BySkill))
	for slug, state := range s.BySkill {
		out[slug.String()] = Skill{Proficiency: state.Proficiency.String(), Bonus: state.Bonus}
	}
	return out
}

func savesOf(s domain.SavingThrows) map[string]SavingThrow {
	out := make(map[string]SavingThrow, len(rules.Abilities()))
	for _, ability := range rules.Abilities() {
		state := s.ByAbility[ability]
		out[ability.Slug().String()] = SavingThrow{Proficient: state.Proficient, Bonus: state.Bonus}
	}
	return out
}

func statusOf(s domain.Status) Status {
	out := Status{
		ArmorClass:        s.ArmorClass,
		Initiative:        s.Initiative,
		ProficiencyBonus:  s.ProficiencyBonus,
		PassivePerception: s.PassivePerception,
	}
	for _, c := range s.Spellcasting {
		out.Spellcasting = append(out.Spellcasting, Spellcasting{
			Class:       c.Class.String(),
			Ability:     c.Ability.Slug().String(),
			SaveDC:      c.SaveDC,
			AttackBonus: c.AttackBonus,
		})
	}
	return out
}

func equipmentOf(e domain.Equipment) Equipment {
	out := Equipment{
		Equipped: stacksOf(e.Equipped),
		Backpack: stacksOf(e.Backpack),
		Loot:     stacksOf(e.Loot),
	}
	if len(e.Purse) > 0 {
		out.Purse = make(map[string]int, len(e.Purse))
		for unit, amount := range e.Purse {
			out.Purse[string(unit)] = amount
		}
	}
	return out
}

func stacksOf(stacks []domain.ItemStack) []ItemStack {
	out := make([]ItemStack, 0, len(stacks))
	for _, s := range stacks {
		stack := ItemStack{Item: s.Item.String(), Count: s.Count}
		if s.Custom != nil {
			stack.Custom = &CustomItem{
				Name: s.Custom.Name, Description: s.Custom.Description, Weight: s.Custom.Weight,
			}
		}
		out = append(out, stack)
	}
	return out
}

func resourcesOf(r domain.Resources) Resources {
	out := Resources{}
	for level := 1; level <= domain.MaxSpellLevel; level++ {
		if r.SpellSlots[level].Max > 0 {
			if out.SpellSlots == nil {
				out.SpellSlots = make(map[string]Pool)
			}
			out.SpellSlots[strconv.Itoa(level)] = poolOf(r.SpellSlots[level])
		}
	}
	for _, p := range r.HitDice {
		out.HitDice = append(out.HitDice, poolOf(p))
	}
	for _, p := range r.Class {
		out.Class = append(out.Class, poolOf(p))
	}
	return out
}

func poolOf(p domain.Pool) Pool {
	out := Pool{Key: p.Key.String(), Max: p.Max, Used: p.Used, Recharge: p.Recharge.String()}
	if p.Dice != nil {
		out.Dice = p.Dice.String()
	}
	return out
}

func spellbookOf(s domain.Spellbook) Spellbook {
	out := Spellbook{
		Cantrips: slugStrings(s.Cantrips),
		Known:    slugStrings(s.Known),
		Prepared: slugStrings(s.Prepared),
	}
	if s.Ability != rules.AbilityNone {
		out.Ability = s.Ability.Slug().String()
	}
	return out
}

func actionsOf(actions []domain.Action) []Action {
	out := make([]Action, 0, len(actions))
	for _, a := range actions {
		action := Action{
			Source: a.Source.String(),
			Origin: refString(a.Origin),
			Kind:   a.Kind.String(),
			Name:   a.Name,
			Range:  int(a.Range),
			ToHit:  a.ToHit,
			Uses:   a.Uses.String(),
			Notes:  a.Notes,
		}
		if a.Damage != nil {
			action.Damage = a.Damage.Dice.String()
		}
		out = append(out, action)
	}
	return out
}

// Inbound: request to domain.

// toEvents converts posted events, reporting every problem at once rather
// than the first: a client that sent three bad values deserves to hear about
// three, not to fix them one round trip at a time.
func toEvents(params []Event) ([]domain.Event, error) {
	var fields []types.FieldError
	out := make([]domain.Event, 0, len(params))

	for i, p := range params {
		event, errs := toEvent(p, i)
		fields = append(fields, errs...)
		out = append(out, event)
	}
	if len(fields) > 0 {
		return nil, types.NewFieldValidationError("the events could not be read", fields...)
	}
	return out, nil
}

func toEvent(p Event, index int) (domain.Event, []types.FieldError) {
	var fields []types.FieldError
	at := func(field string) string {
		return "events[" + strconv.Itoa(index) + "]." + field
	}

	eventType, ok := domain.ParseEventType(p.Type)
	if !ok {
		fields = append(fields, types.FieldError{
			Field: at("type"), Rule: "unknown", Message: "no such event type",
		})
	}

	// Source is deliberately not read. The server writes it, from the prompt
	// the event turns out to answer; taking it from the body would let a
	// client file its own answer under whatever category suited it.
	out := domain.Event{Type: eventType, Level: p.Level, Note: p.Note}
	if p.Ref != "" {
		ref, ok := rules.ParseRef(p.Ref)
		if !ok {
			fields = append(fields, types.FieldError{
				Field: at("ref"), Rule: "format", Message: `a reference reads "kind:slug"`,
			})
		}
		out.Ref = ref
	}

	for j, a := range p.Choices {
		if a.Prompt == "" {
			fields = append(fields, types.FieldError{
				Field:   at("choices[" + strconv.Itoa(j) + "].prompt"),
				Rule:    "required",
				Message: "an answer must name the prompt it answers",
			})
			continue
		}
		picks := make([]rules.Slug, 0, len(a.Picks))
		for _, pick := range a.Picks {
			picks = append(picks, rules.Slug(pick))
		}
		out.Choices = append(out.Choices, domain.Answer{Prompt: rules.Slug(a.Prompt), Picks: picks})
	}

	for j, ch := range p.Changes {
		change, errs := toChange(ch, at("changes["+strconv.Itoa(j)+"]"))
		fields = append(fields, errs...)
		out.Changes = append(out.Changes, change)
	}
	return out, fields
}

func toChange(c Change, field string) (domain.Change, []types.FieldError) {
	var fields []types.FieldError

	op, ok := domain.ParseOp(c.Op)
	if !ok {
		fields = append(fields, types.FieldError{
			Field: field + ".op", Rule: "unknown",
			Message: "an operator is set, increment, add or remove",
		})
	}
	if c.Path == "" {
		fields = append(fields, types.FieldError{
			Field: field + ".path", Rule: "required", Message: "a change must address something",
		})
	}

	value, err := toValue(c.Value)
	if err != nil {
		fields = append(fields, types.FieldError{
			Field: field + ".value", Rule: "kind", Message: err.Error(),
		})
	}
	return domain.Change{Path: domain.Path(c.Path), Op: op, Value: value}, fields
}

func toValue(v Value) (domain.Value, error) {
	switch v.Kind {
	case "int":
		return domain.IntValue(v.Int), nil
	case "string":
		return domain.StringValue(v.String), nil
	case "bool":
		return domain.BoolValue(v.Bool), nil
	case "slug":
		return domain.SlugValue(rules.Slug(v.Slug)), nil
	case "slugs":
		slugs := make([]rules.Slug, 0, len(v.Slugs))
		for _, s := range v.Slugs {
			slugs = append(slugs, rules.Slug(s))
		}
		return domain.SlugListValue(slugs), nil
	case "dice":
		dice, err := rules.ParseDice(v.Dice)
		if err != nil {
			return domain.Value{}, err
		}
		return domain.DiceValue(dice), nil
	case "", "none":
		return domain.Value{}, nil
	}
	return domain.Value{}, types.NewValidationError("no such value kind %q", v.Kind)
}
