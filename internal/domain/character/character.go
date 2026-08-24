// Package character holds the character aggregate.
//
// The character is event-sourced, as DND.md specifies: the Log is the source
// of truth and everything a player sees is a projection of it. That choice is
// what makes level-up reversible, makes "why do I have this proficiency?"
// answerable, and lets a character survive a catalogue regeneration -- the
// events record what was chosen, not what it evaluated to.
//
// This is the innermost layer. It imports the standard library,
// internal/domain/rules and internal/domain/catalog, and nothing else: no gin,
// no net/http, no database/sql, and no JSON or database struct tags.
// Serialization and persistence details belong to the adapters, so that
// changing either one cannot ripple inward.
//
// The dependency on catalog is sideways within layer 1 and points one way
// only: a character reads the compendium, the compendium knows nothing of
// characters.
package character

import (
	"context"

	"github.com/promix1722/easydnd/internal/types"
)

// ID identifies a character.
type ID string

// String returns the identifier's text.
func (id ID) String() string { return string(id) }

// IsZero reports whether the identifier is unset.
func (id ID) IsZero() bool { return id == "" }

// OwnerID identifies whoever the character belongs to.
type OwnerID string

// String returns the owner's text.
func (o OwnerID) String() string { return string(o) }

// Character is a player character.
//
// It holds no state fields of its own: everything about the character lives in
// the Log, and the readable picture comes from Project. Adding a field here
// that is not derivable from the log would create a second source of truth.
type Character struct {
	ID    ID
	Owner OwnerID
	Log   Log
}

// Summary is the short form used for listings, where projecting every
// character's full state would be wasteful.
type Summary struct {
	ID    ID
	Owner OwnerID
	Name  string
	Level int

	// Classes is the class line, e.g. "Rogue 3" or "Cleric 2 / Wizard 1".
	Classes []ClassLevel
}

// Log is the ordered history of a character: one entry per selection, in the
// order the selections were made.
//
// "Append-only" is not the invariant, and never quite was. It is *append, drop
// a suffix, or replace one entry and revalidate what follows* -- see Truncate
// and Rebuild for the two shrinking halves. What holds throughout is that a
// stored answer's meaning depends only on the entries *before* it, which is
// why replacing one entry is safe to reason about and editing one in the
// middle without revalidating what follows is not.
//
// DND.md fixes the storage shape: a character's log is small, so it is stored
// as a single database record holding a JSON array. That is what makes the
// optimistic-concurrency check in Repository.Append both necessary and cheap.
type Log struct {
	Events []Event
}

// Len returns the number of events.
func (l Log) Len() int { return len(l.Events) }

// LastSeq returns the sequence number of the final event, or 0 for an empty
// log. It is the value a caller passes to Repository.Append as the expected
// sequence.
func (l Log) LastSeq() int {
	if len(l.Events) == 0 {
		return 0
	}
	return l.Events[len(l.Events)-1].Seq
}

// Append adds events to the log, numbering each one in turn.
//
// It rejects an event carrying a Seq that does not match its position, so a
// caller cannot append a stale event that would renumber the history.
func (l *Log) Append(events ...Event) error {
	next := l.LastSeq() + 1
	staged := make([]Event, 0, len(events))
	for _, e := range events {
		if e.Seq != 0 && e.Seq != next {
			return types.NewValidationError("event %s has sequence %d, expected %d", e.Type, e.Seq, next)
		}
		if e.Type == EventNone {
			return types.NewValidationError("event at sequence %d has no type", next)
		}
		e.Seq = next
		staged = append(staged, e)
		next++
	}
	l.Events = append(l.Events, staged...)
	return nil
}

// Truncate drops every event after afterSeq.
//
// The log's invariant is not "append-only", which would make going back a
// step impossible; it is *append, drop a suffix, or replace one entry and
// revalidate what follows*. That is what docs/dnd.md means when it says event
// sourcing is what makes level-up reversible: reversible means the log can
// shrink.
//
// The reason the third of those is safe is the same reason an earlier draft
// of this comment gave for forbidding it: a stored answer's meaning depends
// on the entries *before* it. A replace leaves that prefix untouched, so
// every earlier entry means exactly what it did; what it can invalidate is
// the suffix, and the suffix is therefore re-checked entry by entry against
// the log rebuilt so far. Rewriting an entry in place *without* that replay
// is what stays forbidden, and it is forbidden for the original reason --
// it would leave answers standing that the new prefix never offered.
//
// The init event can never be dropped. A character with no opening state is
// not an earlier version of itself, it is an unreadable record; removing a
// character is Repository.Delete.
func (l *Log) Truncate(afterSeq int) error {
	if afterSeq < 1 {
		return types.NewValidationError(
			"cannot truncate to sequence %d: the init event must remain", afterSeq)
	}
	if afterSeq > l.LastSeq() {
		return types.NewValidationError(
			"cannot truncate to sequence %d: the log ends at %d", afterSeq, l.LastSeq())
	}
	l.Events = l.Events[:afterSeq]
	return nil
}

// Rebuild renumbers a slice of events into a log running 1..n and validates
// the result.
//
// Renumbering belongs to the domain because the numbering is a domain
// invariant: Validate requires an event's Seq to be its position, so any
// caller that drops or replaces an entry has to restate every sequence after
// it. Leaving that to the usecase would put the invariant in two places, and
// the copy that drifts is the one that renumbers.
//
// The incoming Seq values are discarded rather than checked. A caller
// rebuilding a log is holding events that were numbered for the log they came
// out of, and insisting those still line up would mean the caller renumbering
// before calling the function whose job is to renumber.
func Rebuild(events []Event) (Log, error) {
	staged := make([]Event, len(events))
	for i, e := range events {
		e.Seq = 0
		staged[i] = e
	}
	var out Log
	if err := out.Append(staged...); err != nil {
		return Log{}, err
	}
	if err := out.Validate(); err != nil {
		return Log{}, err
	}
	return out, nil
}

// Validate reports whether the log is well formed: sequence numbers run 1..n
// without gaps, and an init event appears exactly once, first.
func (l Log) Validate() error {
	initSeen := false
	for i, e := range l.Events {
		if e.Seq != i+1 {
			return types.NewValidationError("event at index %d has sequence %d, expected %d", i, e.Seq, i+1)
		}
		if e.Type == EventInit {
			if i != 0 {
				return types.NewValidationError("init event must be first, found at sequence %d", e.Seq)
			}
			initSeen = true
		}
	}
	if len(l.Events) > 0 && !initSeen {
		return types.NewValidationError("log does not begin with an init event")
	}
	return nil
}

// Repository is the persistence port for characters. Implementations live
// under internal/adapter/repository; internal/app picks the concrete one, and
// that assignment is what proves conformance at compile time.
type Repository interface {
	// Create stores a new character owned by owner and returns it with its
	// assigned ID and an empty log.
	Create(ctx context.Context, owner OwnerID) (Character, error)

	// Get returns the character with the given ID. Implementations report a
	// *types.NotFoundError when it does not exist.
	Get(ctx context.Context, id ID) (Character, error)

	// List returns every character owned by owner, in a stable order.
	//
	// It returns whole characters rather than Summary values because a
	// Summary carries a name, a level and a class line, and none of those
	// are stored -- they are projections of the log against a catalogue,
	// which a repository has neither access to nor any business holding.
	// The application layer summarises; see Summarize.
	List(ctx context.Context, owner OwnerID) ([]Character, error)

	// Append adds events to a character's log, but only if the stored log
	// still ends at expectedSeq. Implementations report a
	// *types.ValidationError when it does not.
	//
	// The check is what makes a whole-log-in-one-record store safe: two
	// clients editing the same character would otherwise read, modify and
	// write the same blob, and the later write would silently discard the
	// earlier one.
	Append(ctx context.Context, id ID, expectedSeq int, events ...Event) error

	// Truncate drops every event after afterSeq, but only if the stored log
	// still ends at expectedSeq. It is the undo primitive: a build flow's
	// Back button, and un-taking a level.
	//
	// Implementations report a *types.ValidationError for a stale
	// expectedSeq, exactly as Append does, and for an afterSeq that would
	// drop the init event or that is not actually in the past.
	Truncate(ctx context.Context, id ID, expectedSeq, afterSeq int) error

	// Rewrite replaces a character's whole log, but only if the stored log
	// still ends at expectedSeq.
	//
	// It exists because replacing one entry can change every entry after it
	// -- an answer the new prefix no longer offers is dropped, and the
	// sequence numbers close up behind it -- so the write is not an append
	// and not a truncation. The caller has already rebuilt and validated the
	// log; implementations check the sequence, check Validate, and store.
	//
	// The concurrency check matters more here than anywhere else, because
	// the write being discarded by a stale one is the entire history.
	// Implementations report a *types.ValidationError for a stale
	// expectedSeq or a log that does not validate.
	Rewrite(ctx context.Context, id ID, expectedSeq int, log Log) error

	// Delete removes a character. Implementations report a
	// *types.NotFoundError when it does not exist.
	Delete(ctx context.Context, id ID) error
}
