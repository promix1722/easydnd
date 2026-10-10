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
	"github.com/promix1722/easydnd/internal/domain/pack"
	"slices"
	"strings"

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
//
// Owner and Folder are not exceptions to that, because neither is a fact about
// the character. They are facts about the *record* -- who it belongs to, and
// where its owner filed it -- so neither is projected onto a sheet and neither
// belongs in the log. Moving a character to another folder is not something
// that happened to the character in the fiction, and it has no business
// appearing in their history.
type Character struct {
	Revision int

	ID     ID
	Owner  OwnerID
	Folder FolderID
	Log    Log

	// Public opens the character to anybody signed in who has its link: a
	// read of the sheet, never a write. False -- the default -- leaves it to
	// its owner and to the groups it is shared with. It is the owner's switch
	// and not part of the log: who may look at a character is not a fact
	// about the character.
	Public bool
}

// Summary is the short form used for listings, where projecting every
// character's full state would be wasteful.
type Summary struct {
	ID     ID
	Owner  OwnerID
	Folder FolderID
	Image  string
	Name   string
	Level  int

	// Public is whether its owner opened it. Only a table's roster fills it:
	// that is the one listing with readers who are not the owner.
	Public bool

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
// optimistic-concurrency check in Repository.Commit both necessary and cheap.
type Log struct {
	Events []Event
}

// Len returns the number of events.
func (l Log) Len() int { return len(l.Events) }

// LastSeq returns the sequence number of the final event, or 0 for an empty
// log.
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
	ids := map[string]bool{}
	for i, e := range l.Events {
		if e.ID != "" {
			if ids[e.ID] {
				return types.NewValidationError("duplicate event identity")
			}
			ids[e.ID] = true
		}
		if !e.RulesLock.IsZero() {
			if err := e.RulesLock.Validate(); err != nil {
				return err
			}
		}
		if e.SchemaVersion < 0 || e.SchemaVersion > 1 {
			return types.NewValidationError("unsupported event schema %d", e.SchemaVersion)
		}
		if i > 0 && !e.RulesLock.IsZero() {
			return types.NewValidationError("rules lock belongs to init event")
		}
		if e.Seq != i+1 {
			return types.NewValidationError("event at index %d has sequence %d, expected %d", i, e.Seq, i+1)
		}
		if e.Type == EventInit {
			if i != 0 {
				return types.NewValidationError("init event must be first, found at sequence %d", e.Seq)
			}
			initSeen = true
		}
		// One entry, one question. A branch and the picks made inside it are
		// one question -- the nested prompt's id is under its parent's -- and
		// anything else in the same entry is a second decision nobody can
		// point at or change apart from the first. The service refuses such
		// an entry with a field error (see oneSelection there); this is the
		// same rule for every writer that does not go through it -- an
		// import, a migration, a repository handed a whole log -- so that no
		// stored log can hold one.
		for at := 1; at < len(e.Choices); at++ {
			if !strings.HasPrefix(string(e.Choices[at].Prompt), string(e.Choices[0].Prompt)+"/") {
				return types.NewValidationError("event %d answers both %s and %s: one entry answers one question", e.Seq, e.Choices[0].Prompt, e.Choices[at].Prompt)
			}
		}
	}
	if len(l.Events) > 0 && !initSeen {
		return types.NewValidationError("log does not begin with an init event")
	}
	return nil
}

// Query narrows and pages a listing of every stored character, whoever owns
// it. The zero value of each filter means "do not filter on this".
type Query struct {
	Owners []OwnerID
	// ID matches anywhere in the character id.
	ID     string
	Public *bool
	Limit  int
	Offset int
}

// Repository is the persistence port for characters. Implementations live
// under internal/adapter/repository; internal/app picks the concrete one, and
// that assignment is what proves conformance at compile time.
type Repository interface {
	// Commit replaces a character's whole log under its revision, which is
	// the write every application mutation goes through. See
	// Character.Commit for what it checks and how the revision advances.
	Commit(ctx context.Context, id ID, expectedRevision int, log Log) error

	// CreateWithLog stores a new character together with its first log in
	// one write, so that a failure cannot leave an empty character behind
	// the way Create followed by Commit can. See NewWithLog.
	CreateWithLog(ctx context.Context, owner OwnerID, folder FolderID, log Log) (Character, error)

	// Create stores a new character owned by owner, filed in folder, and
	// returns it with its assigned ID and an empty log.
	//
	// The folder is a parameter rather than something set afterwards
	// because a character is never in no folder: the application layer
	// resolves the owner's default when the caller named none, and passing
	// the zero value here would store a character nothing can list.
	Create(ctx context.Context, owner OwnerID, folder FolderID) (Character, error)

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

	// Search lists characters matching q across every owner, newest first,
	// and reports how many match in all. Nothing here asks who is calling:
	// the one caller is the superadmin listing.
	Search(ctx context.Context, q Query) ([]Character, int, error)

	// SetFolder files a character in another folder. Implementations report
	// a *types.NotFoundError when the character does not exist.
	//
	// It does not verify the folder: whether it exists and whether the
	// caller owns it are authorization questions, and those are settled in
	// the application layer where every other one is.
	SetFolder(ctx context.Context, id ID, folder FolderID) error

	// SetPublic opens or hides a character. Like SetFolder it changes
	// nothing in the log and verifies nothing about the caller.
	SetPublic(ctx context.Context, id ID, public bool) error

	// Delete removes a character. Implementations report a
	// *types.NotFoundError when it does not exist.
	Delete(ctx context.Context, id ID) error
}

func (l Log) RulesLock() pack.Lock {
	if len(l.Events) == 0 {
		return pack.Lock{}
	}
	return l.Events[0].RulesLock.Clone()
}
func (l Log) Clone() Log {
	out := Log{Events: slices.Clone(l.Events)}
	for i := range out.Events {
		e := &out.Events[i]
		if e.Custom != nil {
			c := *e.Custom
			if c.Level != nil {
				n := *c.Level
				c.Level = &n
			}
			if c.HitDie != nil {
				n := *c.HitDie
				c.HitDie = &n
			}
			if c.Speed != nil {
				n := *c.Speed
				c.Speed = &n
			}
			c.Item = c.Item.clone()
			e.Custom = &c
		}
		e.RulesLock = e.RulesLock.Clone()
		e.Choices = slices.Clone(e.Choices)
		for j := range e.Choices {
			e.Choices[j].Picks = slices.Clone(e.Choices[j].Picks)
		}
		e.Changes = slices.Clone(e.Changes)
		for j := range e.Changes {
			e.Changes[j].Value.Slugs = slices.Clone(e.Changes[j].Value.Slugs)
			e.Changes[j].Value.Dice.Terms = slices.Clone(e.Changes[j].Value.Dice.Terms)
		}
	}
	return out
}
