package character

import (
	"fmt"
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// validateAndAttribute checks a batch of events against what the character
// can actually answer right now, and stamps each one with the group of the
// prompt it answers.
//
// The batch is validated as a whole, event by event, against a log that grows
// as it goes -- because a batch that chooses a race and answers that race's
// prompt in one request is a perfectly reasonable thing for a client to send,
// and the second half is only legal because of the first.
//
// Validating and attributing are one pass because they read the same thing:
// the prompts open before the event. Splitting them would mean projecting the
// log twice per event to reach the same answer, and would leave two places
// where "which prompt is this?" is decided.
func validateAndAttribute(log domain.Log, cat *catalog.Catalog, events []domain.Event) error {
	working := domain.Log{Events: slices.Clone(log.Events)}

	for i := range events {
		open, err := domain.Prompts(working, cat)
		if err != nil {
			return err
		}
		if err := validateEvent(working, cat, open, events[i], i); err != nil {
			return err
		}
		events[i].Source = sourceOf(working, cat, open, events[i])
		if err := working.Append(events[i]); err != nil {
			return err
		}
	}
	return nil
}

// validateEvent checks one event against the prompts open once its structural
// half has landed.
//
// The two halves matter. An event both selects a catalogue entry and answers
// prompts, and the entry it selects is usually what *opens* those prompts:
// {type: race, ref: half-elf, choices: [half-elf/ability-bonus/0 ...]} is the
// ordinary shape a build step posts. Validating the answers against the
// prompts open before the event would reject it, so the reference is applied
// first, on a copy with the answers stripped, and the answers are checked
// against what that opened.
//
// The structural half is checked twice over, and the two checks are different
// questions. validateRef asks whether the entry exists in the compendium;
// answersAnOpenPrompt asks whether the character was offered it. Only the
// first of those used to be asked, which is how a half-elf could be given a
// hill dwarf's subrace: subrace:hill-dwarf resolves perfectly well, and
// nothing looked at whether anything had asked for a subrace at all.
func validateEvent(
	log domain.Log, cat *catalog.Catalog, open []domain.Prompt, event domain.Event, index int,
) error {
	if requiredRef(event) {
		if err := validateRef(cat, event, index); err != nil {
			return err
		}
		if _, ok := answersAnOpenPrompt(open, event); !ok {
			return types.NewFieldValidationError("some answers are not valid", types.FieldError{
				Field: fmt.Sprintf("events[%d].ref", index), Rule: "not-offered",
				Reason: "field.answer.notAsked",
			})
		}
	}
	fields := validateChanges(cat, event, index)

	_, lost, err := surviving(log, cat, event, index)
	if err != nil {
		return err
	}
	// Append's policy on a lost answer: refuse the write. The player is
	// answering right now, so an answer that is not legal is a mistake they
	// can still correct -- unlike a replay, where nobody is at the keyboard.
	for _, l := range lost {
		fields = append(fields, l.Errors...)
	}
	if len(fields) > 0 {
		return types.NewFieldValidationError("some answers are not valid", fields...)
	}
	return nil
}

// answerLoss is one answer that did not survive, and why.
type answerLoss struct {
	Answer domain.Answer
	Errors []types.FieldError
}

// surviving splits an event's answers into the ones still legal against log
// and the ones that are not.
//
// One loop, two policies. validateEvent turns any loss into a rejected write
// because the player is answering right now; Revise keeps the entry and
// reports the loss because they are not. Both read the same predicate, so
// append's strictness and replay's leniency can never disagree about what
// "legal" means -- which they would within a week if this existed twice.
//
// Answers are checked one at a time, against a log that grows as each is
// accepted, because an answer can open the prompt the next one answers. The
// rogue's Expertise is the case: choosing the "two skills" branch is what
// brings the two-skill prompt into existence, and a client naturally sends
// both in the same event. Checking them all against one snapshot would reject
// the second every time.
func surviving(
	log domain.Log, cat *catalog.Catalog, event domain.Event, index int,
) (kept []domain.Answer, lost []answerLoss, err error) {
	if len(event.Choices) == 0 {
		return nil, nil, nil
	}

	structural := event
	structural.Choices = nil
	structural.Seq = 0
	opened := domain.Log{Events: slices.Clone(log.Events)}
	if err := opened.Append(structural); err != nil {
		return nil, nil, err
	}

	for _, answer := range event.Choices {
		open, err := domain.Prompts(opened, cat)
		if err != nil {
			return nil, nil, err
		}
		if errs := validateAnswer(open, answer, index); len(errs) > 0 {
			lost = append(lost, answerLoss{Answer: answer, Errors: errs})
			continue
		}
		kept = append(kept, answer)
		if err := opened.Append(domain.Event{
			Type:    domain.EventChange,
			Choices: []domain.Answer{answer},
		}); err != nil {
			return nil, nil, err
		}
	}
	return kept, lost, nil
}

// answersAnOpenPrompt reports whether a structural event is one the character
// is *currently being asked for*, and returns the prompt it answers.
//
// Two shapes count, and the second is what keeps a race's own follow-up
// entries alive:
//
//   - the prompt selects the entry itself -- "which race?", "which subrace?",
//     "which class?" -- and the event names one of the options it offers;
//   - the prompt hangs off an entry the character already holds, in which case
//     the prompt states the event it must be posted as, Ref and all, and the
//     event matching that Ref is what makes it the entry those answers belong
//     to.
//
// Level is compared only when both sides state one, so a client that omits the
// level is answering the prompt it was handed either way.
//
// The first match in prompt order wins, and prompt order is the order a build
// flow asks in.
func answersAnOpenPrompt(open []domain.Prompt, event domain.Event) (domain.Prompt, bool) {
	for _, p := range open {
		if p.Event.Type != event.Type {
			continue
		}
		if p.Event.Level != 0 && event.Level != 0 && p.Event.Level != event.Level {
			continue
		}
		if p.Event.Ref.IsZero() && !offers(p.Choice.From, event.Ref) {
			continue
		}
		if !p.Event.Ref.IsZero() && p.Event.Ref != event.Ref {
			continue
		}
		return p, true
	}
	return domain.Prompt{}, false
}

// offers reports whether an option set contains a catalogue entry.
func offers(from rules.OptionSet, ref rules.Ref) bool {
	keys := rules.OptionKeys(from)
	if keys == nil {
		// A set drawn from a collection does not list its members: the
		// collection *is* the option set, and validateRef has already
		// established that the entry is in it. What is left to check is that
		// it is the right collection -- offering a race is not offering a
		// class.
		return from.Collection == ref.Kind
	}
	if !slices.Contains(keys, ref.Slug) {
		return false
	}
	// An option key is a slug and carries no kind, so a slug published under
	// two collections would match on the wrong one. The options themselves
	// carry the kind, which is what settles it.
	for _, option := range from.Options {
		if opt, ok := option.(rules.RefOption); ok && opt.Ref == ref {
			return true
		}
	}
	return false
}

// sourceOf names the group of the prompt an event answers.
//
// It is what the server writes into Event.Source, and it is derived rather
// than accepted: the client posts what a prompt told it to post, and the
// server already knows which prompt that was.
//
// The three shapes an answer arrives in, in the order they are tried:
//
//   - an init event is the identity entry, always. It is the one event no
//     prompt hands out on a character that already exists, because the way to
//     change a name is to replace it;
//   - a structural event is attributed to the prompt that offered it, which is
//     the same match that decided whether to accept it at all;
//   - an event carrying answers is attributed to the first open prompt one of
//     them names.
//
// Failing all three, an event carrying only changes is attributed to whatever
// prompt it *closes*: the six ability scores are answered as changes and name
// no prompt, so the only way to know they settled character/abilities is to
// see that prompt stop being open. Everything left -- a note, a DM's ruling --
// answers nothing and is attributed to nothing.
func sourceOf(
	log domain.Log, cat *catalog.Catalog, open []domain.Prompt, event domain.Event,
) domain.PromptGroup {
	if event.Type == domain.EventInit {
		return domain.GroupIdentity
	}
	if requiredRef(event) {
		if p, ok := answersAnOpenPrompt(open, event); ok {
			return p.Group
		}
		return domain.PromptGroupNone
	}
	for _, answer := range event.Choices {
		if p, found := findPrompt(open, answer.Prompt); found {
			return p.Group
		}
	}
	if len(event.Changes) > 0 {
		return closedGroup(log, cat, open, event)
	}
	return domain.PromptGroupNone
}

// closedGroup returns the group of the first prompt that was open before the
// event and is not open after it.
//
// This costs a second projection, and it is the only way to attribute a
// change event without a table mapping paths to groups -- a table that would
// be a second statement of what Prompts already says, and would be wrong the
// first time a prompt moved between groups.
func closedGroup(
	log domain.Log, cat *catalog.Catalog, open []domain.Prompt, event domain.Event,
) domain.PromptGroup {
	after := domain.Log{Events: slices.Clone(log.Events)}
	staged := event
	staged.Seq = 0
	if err := after.Append(staged); err != nil {
		return domain.PromptGroupNone
	}
	remaining, err := domain.Prompts(after, cat)
	if err != nil {
		return domain.PromptGroupNone
	}
	for _, p := range open {
		if _, still := findPrompt(remaining, p.Choice.Prompt); !still {
			return p.Group
		}
	}
	return domain.PromptGroupNone
}

// requiredRef reports whether an event type must name a catalogue entry.
func requiredRef(event domain.Event) bool {
	switch event.Type {
	case domain.EventRace, domain.EventSubrace, domain.EventBackground,
		domain.EventClass, domain.EventSubclass, domain.EventLevel, domain.EventFeat:
		return true
	}
	return false
}

// validateRef checks that an event names an entry the compendium actually
// has, and of the right kind. A dangling reference would project as a
// character who simply has no race, with nothing anywhere saying why.
func validateRef(cat *catalog.Catalog, event domain.Event, index int) error {
	field := fmt.Sprintf("events[%d].ref", index)
	if event.Ref.IsZero() {
		return types.NewFieldValidationError("some answers are not valid", types.FieldError{
			Field: field, Rule: "required",
		})
	}
	if !exists(cat, event.Ref) {
		return types.NewFieldValidationError("some answers are not valid", types.FieldError{
			Field: field, Rule: "unknown", Reason: "field.answer.notInCompendium",
		})
	}
	return nil
}

func exists(cat *catalog.Catalog, ref rules.Ref) bool {
	switch ref.Kind {
	case rules.RefRace:
		return cat.Races.Has(ref.Slug)
	case rules.RefSubrace:
		return cat.Subraces.Has(ref.Slug)
	case rules.RefBackground:
		return cat.Backgrounds.Has(ref.Slug)
	case rules.RefClass:
		return cat.Classes.Has(ref.Slug)
	case rules.RefSubclass:
		return cat.Subclasses.Has(ref.Slug)
	case rules.RefFeat:
		return cat.Feats.Has(ref.Slug)
	case rules.RefSpell:
		return cat.Spells.Has(ref.Slug)
	case rules.RefItem:
		return cat.Items.Has(ref.Slug)
	}
	// A kind with no collection to check against is accepted rather than
	// rejected: refusing what this function has not learned about yet would
	// make adding a collection a breaking change.
	return true
}

// minScore and maxScore bound an ability score.
//
// The bounds are deliberately wide. Point buy and the standard array are far
// narrower, but validating them here would make a legitimate DM ruling --
// a boon, a cursed item, a homebrew race -- impossible to record. The client
// enforces the generation method it is offering; the server enforces only
// what no rule can produce.
const (
	minScore = 1
	maxScore = 30
)

// validateChanges checks the addressed mutations an event carries.
//
// Only what no rule can produce is checked. The six ability scores used to
// arrive with the create call and were bounded there; they arrive as an answer
// now, so the bound moved here with them rather than being quietly dropped
// along the way. The desired level is bounded by where the 2014 rules stop,
// and the ruleset must be the compendium's own -- which is what makes the
// rules selection final: the only value a change can ever set is the one
// already in effect.
func validateChanges(cat *catalog.Catalog, event domain.Event, index int) []types.FieldError {
	var fields []types.FieldError
	for i, change := range event.Changes {
		if change.Path == "identity.desiredLevel" {
			if change.Value.Kind == domain.ValueInt &&
				(change.Value.Int < 1 || change.Value.Int > domain.MaxCharacterLevel) {
				fields = append(fields, types.FieldError{
					Field:  fmt.Sprintf("events[%d].changes[%d].value", index, i),
					Rule:   "range",
					Reason: "field.level.range",
				})
			}
			continue
		}
		if change.Path == "identity.ruleset" {
			value := change.Value.Str
			if change.Value.Kind == domain.ValueSlug {
				value = change.Value.Slug.String()
			}
			if value != cat.Ruleset {
				fields = append(fields, types.FieldError{
					Field:  fmt.Sprintf("events[%d].changes[%d].value", index, i),
					Rule:   "unsupported",
					Reason: "field.ruleset.unsupported",
					Args:   types.Args{"ruleset": cat.Ruleset},
				})
			}
			continue
		}
		segments := change.Path.Segments()
		if len(segments) != 2 || segments[0] != "abilities" {
			continue
		}
		if _, ok := rules.ParseAbility(segments[1]); !ok {
			continue
		}
		if change.Op != domain.OpSet || change.Value.Kind != domain.ValueInt {
			continue
		}
		if change.Value.Int < minScore || change.Value.Int > maxScore {
			fields = append(fields, types.FieldError{
				Field:  fmt.Sprintf("events[%d].changes[%d].value", index, i),
				Rule:   "range",
				Reason: "field.ability.range",
			})
		}
	}
	return fields
}

// validateAnswer checks one answer against the prompts currently open.
func validateAnswer(open []domain.Prompt, answer domain.Answer, index int) []types.FieldError {
	field := fmt.Sprintf("events[%d].choices.%s", index, answer.Prompt)

	prompt, found := findPrompt(open, answer.Prompt)
	if !found {
		return []types.FieldError{{
			Field: field, Rule: "unknown",
			Reason: "field.answer.promptClosed",
		}}
	}

	var fields []types.FieldError
	if len(answer.Picks) > prompt.Choice.Choose {
		fields = append(fields, types.FieldError{
			Field: field, Rule: "choose",
			Reason: "field.answer.chooseCount",
			Args:   types.Args{"want": prompt.Choice.Choose, "got": len(answer.Picks)},
		})
	}

	legal := rules.OptionKeys(prompt.Choice.From)
	seen := make(map[rules.Slug]bool, len(answer.Picks))
	for _, pick := range answer.Picks {
		// A set drawn from a collection has no inline options; the pick is
		// the entry's own slug, and the reference check above covers it.
		if legal != nil && !slices.Contains(legal, pick) {
			fields = append(fields, types.FieldError{
				Field: field, Rule: "option", Reason: "field.answer.notAnOption",
			})
			continue
		}
		// Duplicates are rejected unless the prompt says its picks are points
		// being spent: two into Dexterity is how "+2 to one ability" is
		// written, whereas being proficient in Stealth twice is not a thing.
		//
		// The prompt says it, rather than the kind implying it. A half-elf's
		// two ability bonuses are the same kind over the same options and are
		// "two *different* scores", so keying this on the kind let a half-elf
		// put both of theirs into one.
		if seen[pick] && !prompt.Choice.Repeatable {
			fields = append(fields, types.FieldError{
				Field: field, Rule: "duplicate", Reason: "field.answer.duplicate",
			})
		}
		seen[pick] = true

		// Held options stay in the option set on purpose -- removing them
		// would make the prompt depend on the order it was answered in -- so
		// the rejection happens here instead.
		//
		// HeldOnly inverts the test. Expertise doubles a proficiency the
		// character already has, so there holding a skill is what makes it
		// pickable rather than what rules it out.
		held := slices.Contains(prompt.Held, pick)
		switch {
		case prompt.HeldOnly && !held:
			fields = append(fields, types.FieldError{
				Field: field, Rule: "not-held", Reason: "field.answer.notProficient",
			})
		case !prompt.HeldOnly && held:
			fields = append(fields, types.FieldError{
				Field: field, Rule: "held", Reason: "field.answer.alreadyHeld",
			})
		}
	}
	return fields
}

func findPrompt(open []domain.Prompt, id rules.Slug) (domain.Prompt, bool) {
	for _, p := range open {
		if p.Choice.Prompt == id {
			return p, true
		}
	}
	return domain.Prompt{}, false
}
