package character

import (
	"fmt"
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// validateEvents checks a batch of events against what the character can
// actually answer right now.
//
// The batch is validated as a whole, event by event, against a log that grows
// as it goes -- because a batch that chooses a race and answers that race's
// prompt in one request is a perfectly reasonable thing for a client to send,
// and the second half is only legal because of the first.
func validateEvents(log domain.Log, cat *catalog.Catalog, events []domain.Event) error {
	working := domain.Log{Events: slices.Clone(log.Events)}

	for i, event := range events {
		if err := validateEvent(working, cat, event, i); err != nil {
			return err
		}
		if err := working.Append(event); err != nil {
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
func validateEvent(log domain.Log, cat *catalog.Catalog, event domain.Event, index int) error {
	if requiredRef(event) {
		if err := validateRef(cat, event, index); err != nil {
			return err
		}
	}
	if len(event.Choices) == 0 {
		return nil
	}

	structural := event
	structural.Choices = nil
	structural.Seq = 0
	opened := domain.Log{Events: slices.Clone(log.Events)}
	if err := opened.Append(structural); err != nil {
		return err
	}

	// Answers are checked one at a time, against a log that grows as each is
	// accepted, because an answer can open the prompt the next one answers.
	// The rogue's Expertise is the case: choosing the "two skills" branch is
	// what brings the two-skill prompt into existence, and a client naturally
	// sends both in the same event. Checking them all against one snapshot
	// would reject the second every time.
	var fields []types.FieldError
	for _, answer := range event.Choices {
		open, err := domain.Prompts(opened, cat)
		if err != nil {
			return err
		}
		errs := validateAnswer(open, answer, index)
		fields = append(fields, errs...)
		if len(errs) > 0 {
			continue
		}
		if err := opened.Append(domain.Event{
			Type:    domain.EventChange,
			Choices: []domain.Answer{answer},
		}); err != nil {
			return err
		}
	}
	if len(fields) > 0 {
		return types.NewFieldValidationError("some answers are not valid", fields...)
	}
	return nil
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
			Message: fmt.Sprintf("a %s event must name a catalogue entry", event.Type),
		})
	}
	if !exists(cat, event.Ref) {
		return types.NewFieldValidationError("some answers are not valid", types.FieldError{
			Field: field, Rule: "unknown",
			Message: fmt.Sprintf("%s is not in the compendium", event.Ref),
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

// validateAnswer checks one answer against the prompts currently open.
func validateAnswer(open []domain.Prompt, answer domain.Answer, index int) []types.FieldError {
	field := fmt.Sprintf("events[%d].choices.%s", index, answer.Prompt)

	prompt, found := findPrompt(open, answer.Prompt)
	if !found {
		return []types.FieldError{{
			Field: field, Rule: "unknown",
			Message: "this character has no such prompt open",
		}}
	}

	var fields []types.FieldError
	if len(answer.Picks) > prompt.Choice.Choose {
		fields = append(fields, types.FieldError{
			Field: field, Rule: "choose",
			Message: fmt.Sprintf("choose %d, got %d", prompt.Choice.Choose, len(answer.Picks)),
		})
	}

	legal := rules.OptionKeys(prompt.Choice.From)
	seen := make(map[rules.Slug]bool, len(answer.Picks))
	for _, pick := range answer.Picks {
		// A set drawn from a collection has no inline options; the pick is
		// the entry's own slug, and the reference check above covers it.
		if legal != nil && !slices.Contains(legal, pick) {
			fields = append(fields, types.FieldError{
				Field: field, Rule: "option",
				Message: fmt.Sprintf("%q is not one of this prompt's options", pick),
			})
			continue
		}
		// Duplicates are rejected, except where the prompt is adding a
		// number: two points into Dexterity is how "+2 to one ability" is
		// written, whereas being proficient in Stealth twice is not a thing.
		if seen[pick] && prompt.Choice.Kind != rules.ChooseAbilityBonus {
			fields = append(fields, types.FieldError{
				Field: field, Rule: "duplicate",
				Message: fmt.Sprintf("%q is picked more than once", pick),
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
				Field: field, Rule: "not-held",
				Message: fmt.Sprintf("the character is not proficient in %q", pick),
			})
		case !prompt.HeldOnly && held:
			fields = append(fields, types.FieldError{
				Field: field, Rule: "held",
				Message: fmt.Sprintf("the character already has %q from another source", pick),
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
