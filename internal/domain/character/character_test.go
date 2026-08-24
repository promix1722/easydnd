package character_test

import (
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

func TestRogueLogValidates(t *testing.T) {
	log := domain.RogueLog(t)

	if log.Len() != 8 {
		t.Fatalf("log length = %d, want 8", log.Len())
	}
	if err := log.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
	if log.LastSeq() != 8 {
		t.Errorf("LastSeq() = %d, want 8", log.LastSeq())
	}
	for i, e := range log.Events {
		if e.Seq != i+1 {
			t.Errorf("event %d sequence = %d, want %d", i, e.Seq, i+1)
		}
	}
}

// Every answer must name a prompt, because that slug is the only link back to
// the catalogue question it answers.
func TestRogueChoicesCarryPrompts(t *testing.T) {
	for _, e := range domain.RogueLog(t).Events {
		for _, answer := range e.Choices {
			if answer.Prompt.IsZero() {
				t.Errorf("event %d (%s) has an answer with no prompt", e.Seq, e.Type)
			}
			if len(answer.Picks) == 0 {
				t.Errorf("event %d answer %q has no picks", e.Seq, answer.Prompt)
			}
		}
	}
}

func TestAppendRejectsAnUntypedEvent(t *testing.T) {
	var log domain.Log
	if err := log.Append(domain.Event{}); err == nil {
		t.Error("Append() accepted an event with no type")
	}
	if log.Len() != 0 {
		t.Errorf("log length = %d, want 0: a rejected batch must not be written", log.Len())
	}
}

func TestAppendRejectsAStaleSequence(t *testing.T) {
	var log domain.Log
	if err := log.Append(domain.Event{Type: domain.EventInit}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	// Seq 1 is already taken; re-stating it would renumber history.
	if err := log.Append(domain.Event{Type: domain.EventRace, Seq: 1}); err == nil {
		t.Error("Append() accepted an event with a stale sequence")
	}
}

func TestValidateRequiresInitFirst(t *testing.T) {
	log := domain.Log{Events: []domain.Event{
		{Seq: 1, Type: domain.EventRace},
		{Seq: 2, Type: domain.EventInit},
	}}
	if err := log.Validate(); err == nil {
		t.Error("Validate() accepted an init event that is not first")
	}

	noInit := domain.Log{Events: []domain.Event{{Seq: 1, Type: domain.EventRace}}}
	if err := noInit.Validate(); err == nil {
		t.Error("Validate() accepted a log with no init event")
	}
}

func TestEventTypeRoundTrips(t *testing.T) {
	for _, want := range []domain.EventType{
		domain.EventInit, domain.EventChange, domain.EventRace, domain.EventSubrace,
		domain.EventBackground, domain.EventClass, domain.EventSubclass,
		domain.EventLevel, domain.EventFeat, domain.EventNote,
	} {
		got, ok := domain.ParseEventType(want.String())
		if !ok || got != want {
			t.Errorf("ParseEventType(%q) = %v, %v; want %v, true", want, got, ok, want)
		}
	}
}

func TestIdentityLevelSumsClasses(t *testing.T) {
	id := domain.Identity{Classes: []domain.ClassLevel{
		{Class: "cleric", Level: 2},
		{Class: "wizard", Level: 1},
	}}
	if got := id.Level(); got != 3 {
		t.Errorf("Level() = %d, want 3", got)
	}
}

func TestAbilitiesDefaultToTen(t *testing.T) {
	a := domain.Abilities{Scores: map[rules.Ability]int{rules.Dexterity: 16}}
	if got := a.Score(rules.Dexterity); got != 16 {
		t.Errorf("Score(DEX) = %d, want 16", got)
	}
	if got := a.Modifier(rules.Dexterity); got != 3 {
		t.Errorf("Modifier(DEX) = %d, want 3", got)
	}
	// An unset score is 10, the SRD's default, not zero -- which would give a
	// -5 modifier to every ability the character has not rolled yet.
	if got := a.Score(rules.Strength); got != 10 {
		t.Errorf("Score(STR) = %d, want 10", got)
	}
}
