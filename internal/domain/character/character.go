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

// Log is the ordered, append-only history of a character.
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
// step impossible; it is *append, or drop a suffix -- never edit the middle*.
// That is what docs/dnd.md means when it says event sourcing is what makes
// level-up reversible: reversible means the log can shrink. Rewriting an
// event in place is what stays forbidden, because a stored answer's meaning
// depends on the events before it.
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

	// Delete removes a character. Implementations report a
	// *types.NotFoundError when it does not exist.
	Delete(ctx context.Context, id ID) error
}
