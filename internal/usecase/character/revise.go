package character

import (
	"context"
	"fmt"
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// DropReason says what a replacement cost one entry of the suffix.
//
// Three words, and they are different events for a player. not-offered means
// the entry itself is gone -- a subrace nothing offers any more. empty means
// the entry had nothing left in it once its answers went. answers-dropped is
// the one that is *not* a deletion: the entry stands, minus some picks, and
// the questions behind those picks come back outstanding.
type DropReason string

// The reasons an entry does not survive a replacement unchanged.
const (
	DropNotOffered     DropReason = "not-offered"
	DropAnswersDropped DropReason = "answers-dropped"
	DropEmpty          DropReason = "empty"
)

// LostAnswer is one answer a replacement invalidated.
//
// Rule is the vocabulary validateAnswer already speaks -- unknown, option,
// choose, duplicate, held, not-held -- so the client renders this with the
// words it already renders a rejected append with. Where an answer failed
// several ways the first is reported, because the client draws one line per
// lost answer and the first is the one that killed it.
type LostAnswer struct {
	Prompt  rules.Slug
	Picks   []rules.Slug
	Rule    string
	Message string
}

// Dropped is one entry a replacement did not leave alone.
//
// Seq is the entry's position in the log as it stood *before* the
// replacement, not after: the whole point of the report is to name entries
// the player can still see on screen, and after the rebuild half of them have
// moved and some do not exist.
type Dropped struct {
	Seq    int
	Type   domain.EventType
	Ref    rules.Ref
	Level  int
	Source domain.PromptGroup
	Reason DropReason
	Lost   []LostAnswer
}

// Revision is the outcome of replacing or removing one entry.
type Revision struct {
	// Seq is where the log ends afterwards -- or would end, on a dry run.
	Seq     int
	Dropped []Dropped

	// Sheet is the character as the rebuilt log projects. On a dry run it is
	// the sheet the player would get, computed by exactly the code that would
	// have produced it, because it *is* that code.
	Sheet domain.State
}

// Revise replaces the entry at targetSeq -- or removes it, when replacement
// is nil -- and revalidates everything after it.
//
// Two properties make this safe to reason about, and both are worth stating
// because both are easy to lose in a refactor.
//
// The suffix is judged against the log rebuilt *so far*, before each entry is
// applied. Not against the old log, which is gone, and not against the
// finished new one, which does not exist yet and would make an entry's
// legality depend on entries that come after it. Walking forwards means every
// entry is checked against exactly the prefix it will actually sit on, which
// is the same thing an append checks against -- so revalidation is never
// stricter than the predicate that accepted the entry in the first place,
// except where that predicate was wrong.
//
// An entry carrying a Ref is never dropped merely because its answers died.
// A rogue whose race change invalidated one of their four class skills is
// still a rogue: the class entry stands, keeps every other answer it carries
// -- the Expertise, the starting weapon -- and the invalidated question comes
// back outstanding under its own group. Deleting the entry would take the
// class, the answers that were still fine and every level built on it, and a
// revalidation that silently eats a player's choices is worse than the
// truncation it replaces.
//
// The granularity is one *answer*, not one pick: an answer is what a prompt
// was asked for, so half of one is not an answer to anything. Four skills
// picked together stand or fall together, and the prompt is asked again.
func Revise(
	log domain.Log, cat *catalog.Catalog, targetSeq int, replacement *domain.Event,
) (domain.Log, []Dropped, error) {
	if targetSeq < 1 || targetSeq > log.LastSeq() {
		return domain.Log{}, nil, seqError(fmt.Sprintf(
			"the log has no entry at %d; it runs 1 to %d", targetSeq, log.LastSeq()))
	}
	// Seq 1 is replaceable, but only by another init event -- which is how a
	// name is changed. Log.Validate already requires init to be first and to
	// appear exactly once, so this is a type check rather than a prohibition:
	// what it refuses is a log that could not be read back.
	if targetSeq == 1 && (replacement == nil || replacement.Type != domain.EventInit) {
		return domain.Log{}, nil, seqError(
			"the opening entry can only be replaced by another init event")
	}
	if targetSeq != 1 && replacement != nil && replacement.Type == domain.EventInit {
		return domain.Log{}, nil, seqError(
			"an init event can only be the opening entry")
	}

	// The prefix is a Log rather than a bare slice, and each entry is staged
	// through Append, because the replay projects the prefix at every step
	// and Project refuses a log whose sequence numbers do not run 1..n. The
	// numbering therefore has to be right *during* the rebuild, not only at
	// the end of it.
	rebuilt := domain.Log{Events: slices.Clone(log.Events[:targetSeq-1])}
	stage := func(event domain.Event) error {
		event.Seq = 0
		return rebuilt.Append(event)
	}

	if replacement != nil {
		open, err := domain.Prompts(rebuilt, cat)
		if err != nil {
			return domain.Log{}, nil, err
		}
		// Strict, exactly as an append is: a replacement is something the
		// player is choosing right now. A rejection here writes nothing at
		// all, so the stored log is byte-identical afterwards.
		staged := *replacement
		staged.Seq = 0
		if err := validateEvent(rebuilt, cat, open, staged, 0); err != nil {
			return domain.Log{}, nil, err
		}
		staged.Source = sourceOf(rebuilt, cat, open, staged)
		if err := stage(staged); err != nil {
			return domain.Log{}, nil, err
		}
	}

	var dropped []Dropped
	for _, event := range log.Events[targetSeq:] {
		open, err := domain.Prompts(rebuilt, cat)
		if err != nil {
			return domain.Log{}, nil, err
		}

		if requiredRef(event) {
			if _, ok := answersAnOpenPrompt(rebuilt, cat, open, event); !ok {
				dropped = append(dropped, droppedEntry(event, DropNotOffered, nil))
				continue
			}
		}

		kept, lost, err := surviving(rebuilt, cat, event, 0)
		if err != nil {
			return domain.Log{}, nil, err
		}
		staged := event
		staged.Choices = kept
		switch {
		case saysNothing(staged):
			// An entry that was nothing but answers, all of which died. Its
			// own reason rather than answers-dropped, because the row it
			// names has gone from the screen rather than got shorter -- and
			// it still carries the lost answers, which are what the player
			// will be asked again.
			dropped = append(dropped, droppedEntry(event, DropEmpty, lost))
			continue
		case len(lost) > 0:
			dropped = append(dropped, droppedEntry(event, DropAnswersDropped, lost))
		}
		staged.Source = sourceOf(rebuilt, cat, open, staged)
		if err := stage(staged); err != nil {
			return domain.Log{}, nil, err
		}
	}

	// Rebuild is still what returns the log, though staging has already
	// numbered it: renumbering is the domain's invariant to state, and a
	// caller that has kept it by hand still has to be told so.
	out, err := domain.Rebuild(rebuilt.Events)
	if err != nil {
		return domain.Log{}, nil, err
	}
	return out, dropped, nil
}

// saysNothing reports that an entry has nothing left to say: no catalogue
// entry, no answers, no changes, no note.
//
// An entry with a Ref never says nothing, which is the guard behind the
// second property in Revise's comment -- a race entry that lost every answer
// is still the entry that sets the race.
func saysNothing(event domain.Event) bool {
	return !requiredRef(event) &&
		len(event.Choices) == 0 && len(event.Changes) == 0 && event.Note == ""
}

func droppedEntry(event domain.Event, reason DropReason, lost []answerLoss) Dropped {
	out := Dropped{
		Seq:    event.Seq,
		Type:   event.Type,
		Ref:    event.Ref,
		Level:  event.Level,
		Source: event.Source,
		Reason: reason,
	}
	for _, l := range lost {
		entry := LostAnswer{Prompt: l.Answer.Prompt, Picks: l.Answer.Picks}
		if len(l.Errors) > 0 {
			entry.Rule = l.Errors[0].Rule
			entry.Message = l.Errors[0].Message
		}
		out.Lost = append(out.Lost, entry)
	}
	return out
}

// seqError reports a target that is not a replaceable position. It names seq
// rather than the body, because the position is in the path.
func seqError(message string) error {
	return types.NewFieldValidationError("the entry cannot be replaced", types.FieldError{
		Field: "seq", Rule: "range", Message: message,
	})
}

// Revise replaces one entry of a character's log and revalidates the rest,
// returning what the change cost and what the character becomes.
//
// commit is the whole difference between a preview and the change itself. A
// dry run loads, validates, replays, renumbers, validates the rebuilt log and
// projects it -- and then skips exactly one line, the write. A separate
// preview route would be two paths to drift, and a preview that disagrees
// with its commit is worse than no preview at all.
//
// A stale preview cannot be committed silently, and that costs nothing extra:
// the commit re-runs the replay, and if the log moved in between, expectedSeq
// makes it the ordinary sequence conflict that every other write already
// reports.
func (s *Service) Revise(
	ctx context.Context,
	owner domain.OwnerID,
	id domain.ID,
	locale rules.Locale,
	expectedSeq, targetSeq int,
	replacement *domain.Event,
	commit bool,
) (Revision, error) {
	character, cat, err := s.load(ctx, owner, id, locale)
	if err != nil {
		return Revision{}, err
	}
	if got := character.Log.LastSeq(); got != expectedSeq {
		return Revision{}, types.NewValidationError(
			"character %q is at sequence %d, not %d", id, got, expectedSeq)
	}

	rebuilt, dropped, err := Revise(character.Log, cat, targetSeq, replacement)
	if err != nil {
		return Revision{}, err
	}
	// Projected before the write, not after, so a rebuilt log that cannot be
	// read back is a rejected request rather than a character nobody can open.
	sheet, err := domain.Project(rebuilt, cat)
	if err != nil {
		return Revision{}, err
	}
	if commit {
		if err := s.repo.Rewrite(ctx, id, expectedSeq, rebuilt); err != nil {
			return Revision{}, err
		}
	}
	return Revision{Seq: rebuilt.LastSeq(), Dropped: dropped, Sheet: sheet}, nil
}
