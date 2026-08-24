package character

import (
	"strings"
	"time"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

// EventType discriminates a log entry.
//
// DND.md fixes the first six; level, feat and note extend the "TODO" it left
// open. They are the minimum a level-up tracker needs, and every one of them
// could equally have been expressed as a change -- they exist because a typed
// event can be replayed against a regenerated catalogue, and an opaque change
// cannot.
type EventType uint8

// The event types.
const (
	EventNone EventType = iota
	// EventInit seeds a new character: name, ability scores, starting level.
	// It is always the first event and may appear only once.
	EventInit
	// EventChange is an arbitrary, user-authored mutation. It is the escape
	// hatch for anything the typed events do not cover -- a DM's ruling, a
	// homebrew adjustment -- and its payload is Changes.
	EventChange
	// EventRace, EventSubrace, EventBackground, EventClass and EventSubclass
	// record a character-building choice. Ref names the catalogue entry and
	// Choices carries the answers to its prompts.
	EventRace
	EventSubrace
	EventBackground
	EventClass
	EventSubclass
	// EventLevel records gaining a level in a class. Ref is the class and
	// Level is the new class level.
	EventLevel
	// EventFeat records taking a feat.
	EventFeat
	// EventNote records a player's annotation and changes nothing.
	EventNote
)

var eventTypeNames = map[EventType]string{
	EventNone:       "none",
	EventInit:       "init",
	EventChange:     "change",
	EventRace:       "race",
	EventSubrace:    "subrace",
	EventBackground: "background",
	EventClass:      "class",
	EventSubclass:   "subclass",
	EventLevel:      "level",
	EventFeat:       "feat",
	EventNote:       "note",
}

// String returns the type's wire name, or "unknown" outside the enumeration.
func (t EventType) String() string {
	if name, ok := eventTypeNames[t]; ok {
		return name
	}
	return "unknown"
}

// ParseEventType maps a wire name to its EventType. The second result reports
// whether the name was recognised.
func ParseEventType(s string) (EventType, bool) {
	for t, name := range eventTypeNames {
		if name == s && t != EventNone {
			return t, true
		}
	}
	return EventNone, false
}

// Event is one entry in a character's log.
//
// Every event has the same field structure, as DND.md requires: one struct
// with a Type discriminator rather than a sealed interface. Fields not used by
// a given type are zero. That uniformity is what lets the whole log be one
// JSON array in one database record.
type Event struct {
	// Seq is the event's 1-based position in the log. It doubles as the
	// optimistic-concurrency token: an append states the sequence it expects
	// to follow, so two clients editing the same character cannot silently
	// interleave.
	Seq int

	Type EventType

	// Source is the group of the prompt this entry answers: the race prompt
	// makes a race entry, the class prompt a class entry. It is what lets a
	// client group the log by the question each entry settles instead of
	// inferring the grouping from the event's type -- an inference that has
	// no answer for a change event carrying six ability scores.
	//
	// It is written by the server, from the prompt the event was matched
	// against, and never read off a request. A client-supplied source would
	// be a second vocabulary for the same fact, unverified and free to
	// disagree with the one the rules produce.
	//
	// Zero where the server cannot attribute the entry: an imported log
	// answers no prompt, and a DM's change answers whatever the DM had in
	// mind. Those entries are still in the log; they simply belong to no
	// question.
	Source PromptGroup

	// At is when the event was recorded, not when it happened in the game.
	At time.Time

	// Ref names the catalogue entry this event selects: the race, the class,
	// the feat. Zero for init, change and note.
	Ref rules.Ref

	// Level is the class level this event applies at. Zero when the event is
	// not level-bound.
	Level int

	// Choices are the player's answers to the prompts on the referenced
	// entry -- which two skills, which fighting style.
	Choices []Answer

	// Changes are addressed mutations. Only init and change use them.
	Changes []Change

	// Note is free-form text the player wrote. It is never interpreted.
	Note string
}

// Answer records the player's response to one catalogue prompt.
//
// Prompt matches rules.Choice.Prompt, which is why that slug has to stay
// stable across catalogue regenerations: an answer whose prompt no longer
// resolves is a choice the character silently loses.
type Answer struct {
	Prompt rules.Slug
	Picks  []rules.Slug
}

// Op is what a Change does to the value at its path.
type Op uint8

// The mutation operators.
const (
	OpNone Op = iota
	// OpSet replaces the value.
	OpSet
	// OpIncrement adds to a numeric value, and is how a +1 is recorded so
	// that it survives an earlier value changing.
	OpIncrement
	// OpAdd appends to a collection.
	OpAdd
	// OpRemove deletes from a collection.
	OpRemove
)

var opNames = map[Op]string{
	OpNone:      "none",
	OpSet:       "set",
	OpIncrement: "increment",
	OpAdd:       "add",
	OpRemove:    "remove",
}

// String returns the operator's wire name, or "unknown" outside the
// enumeration.
func (o Op) String() string {
	if name, ok := opNames[o]; ok {
		return name
	}
	return "unknown"
}

// ParseOp maps a wire name to its Op. The second result reports whether the
// name was recognised.
func ParseOp(s string) (Op, bool) {
	for op, name := range opNames {
		if name == s && op != OpNone {
			return op, true
		}
	}
	return OpNone, false
}

// Path addresses a field of the projected State in dotted form:
// "abilities.dex", "hitPoints.max", "conditions".
//
// It is a string rather than a typed accessor because change events must be
// able to reach anything, including fields added after the event was written.
// A path that no longer resolves is reported by the projector, not silently
// dropped.
type Path string

// Segments splits the path on dots.
func (p Path) Segments() []string {
	if p == "" {
		return nil
	}
	return strings.Split(string(p), ".")
}

// String returns the path text.
func (p Path) String() string { return string(p) }

// Change is one addressed mutation of the projected state.
type Change struct {
	Path  Path
	Op    Op
	Value Value
}

// ValueKind discriminates a Value's payload.
type ValueKind uint8

// The value kinds a change can carry.
const (
	ValueNone ValueKind = iota
	ValueInt
	ValueString
	ValueBool
	ValueSlug
	ValueSlugList
	ValueDice
)

// Value is the payload of a Change.
//
// It is a tagged struct rather than an `any`, because the domain must stay
// free of the JSON round-tripping that an interface payload would force on
// every adapter -- and because an unhandled kind should be a compile-time
// switch to extend, not a runtime type assertion that panics.
type Value struct {
	Kind  ValueKind
	Int   int
	Str   string
	Bool  bool
	Slug  rules.Slug
	Slugs []rules.Slug
	Dice  rules.Dice
}

// IntValue builds an integer Value.
func IntValue(n int) Value { return Value{Kind: ValueInt, Int: n} }

// StringValue builds a string Value.
func StringValue(s string) Value { return Value{Kind: ValueString, Str: s} }

// BoolValue builds a boolean Value.
func BoolValue(b bool) Value { return Value{Kind: ValueBool, Bool: b} }

// SlugValue builds a single-slug Value.
func SlugValue(s rules.Slug) Value { return Value{Kind: ValueSlug, Slug: s} }

// SlugListValue builds a slug-list Value.
func SlugListValue(s []rules.Slug) Value { return Value{Kind: ValueSlugList, Slugs: s} }

// DiceValue builds a dice Value.
func DiceValue(d rules.Dice) Value { return Value{Kind: ValueDice, Dice: d} }
