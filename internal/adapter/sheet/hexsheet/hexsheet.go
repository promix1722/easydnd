// Package hexsheet imports a character sheet exported from HexSheet.
//
// # What an import can and cannot know
//
// A HexSheet export records what a character *is*, not what was *chosen*. It
// says the character is proficient in Stealth; it does not say whether that
// came from the class, the background or a racial trait. easydnd's log is the
// opposite: it stores the choices and derives the sheet from them.
//
// Bridging that gap by reconstructing the choices would mean solving a
// matching problem -- six proficient skills across a class prompt that takes
// four from a restricted list and a trait prompt that takes two from all
// eighteen -- and then presenting one of several answers that fit as though it
// were the one the player made. That is inference dressed as fact, and a sheet
// is the wrong place for it.
//
// So this importer does none of it. The export's final state becomes the
// character's *opening* state: an init event carrying the numbers, plus typed
// events naming the race, class, subclass and levels the export states
// outright. No prompt is answered. Every choice the character has stays open,
// and the build screen asks for them exactly as it would for a new character.
//
// Anything that does not map is named in the Report rather than dropped, so
// the player learns what did not come across here instead of from a wrong
// number weeks later.
package hexsheet

import (
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// Entry is one line of a Report: a field of the export, and what became of it.
type Entry struct {
	// Field is the export's own path, e.g. "character.background".
	Field string

	// Detail says what was there and why it did not survive.
	Detail string
}

// Report is everything the import could not carry across.
//
// It is not an error list. An import that reports twenty entries has still
// produced a usable character -- SRD 5.1 publishes one background and one
// feat, so a sheet from a tool with the full rules will always leave some
// behind. The report exists so that is visible rather than silent.
type Report struct {
	// Unresolved names something SRD 5.1 does not publish.
	Unresolved []Entry

	// Skipped is real data the model has no home for.
	Skipped []Entry

	// Open lists the prompts the import deliberately left unanswered.
	Open []rules.Slug
}

// Import reads a HexSheet export and returns the log it maps to.
//
// The time is a parameter rather than a clock read so that Import is pure:
// same export, same catalogue, same time, same log. Project is written the
// same way and for the same reason.
func Import(r io.Reader, cat *catalog.Catalog, at time.Time) (character.Log, Report, error) {
	if cat == nil {
		return character.Log{}, Report{}, types.NewValidationError(
			"importing against a nil catalogue")
	}

	var doc export
	dec := json.NewDecoder(r)
	if err := dec.Decode(&doc); err != nil {
		var syntax *json.SyntaxError
		var mismatch *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntax), errors.As(err, &mismatch):
			return character.Log{}, Report{}, types.NewFieldValidationError(
				"this file is not a HexSheet export", types.FieldError{
					Field: "file", Rule: "malformed",
				})
		}
		return character.Log{}, Report{}, types.NewFieldValidationError(
			"the export could not be read", types.FieldError{
				Field: "file", Rule: "unreadable",
			})
	}

	if err := validate(doc); err != nil {
		return character.Log{}, Report{}, err
	}

	c := newConverter(doc.Character, cat)
	log, err := c.build(at)
	if err != nil {
		return character.Log{}, Report{}, err
	}
	return log, c.report, nil
}

// validate rejects an export this importer cannot honestly read.
//
// The checks are deliberately few. A wrong game system or ruleset produces a
// character that is wrong in ways no report could usefully list, so those are
// refused; everything else is imported as far as it goes and reported where it
// does not. Refusing more would mean refusing exports that mostly work.
func validate(doc export) error {
	var fields []types.FieldError

	if doc.ExportedFrom != "" && doc.ExportedFrom != "HexSheet" {
		fields = append(fields, types.FieldError{
			Field: "exportedFrom", Rule: "unsupported",
			Reason: "field.import.wrongSource", Args: types.Args{"source": doc.ExportedFrom},
		})
	}
	if system := doc.Character.GameSystem; system != "" && system != "dnd5e" {
		fields = append(fields, types.FieldError{
			Field: "character.gameSystem", Rule: "unsupported",
			Reason: "field.import.wrongSystem", Args: types.Args{"system": system},
		})
	}
	if ruleset := doc.Character.Ruleset; ruleset != "" && ruleset != "2014" {
		fields = append(fields, types.FieldError{
			Field: "character.ruleset", Rule: "unsupported",
			Reason: "field.import.wrongRuleset", Args: types.Args{"ruleset": ruleset},
		})
	}
	if doc.Character.Name == "" {
		fields = append(fields, types.FieldError{
			Field: "character.name", Rule: "required",
			Reason: "field.import.noName",
		})
	}

	if len(fields) > 0 {
		return types.NewFieldValidationError("this export cannot be imported", fields...)
	}
	return nil
}
