package character_test

import (
	"context"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// Declaring a level is taking it: no entry names a class level, and the
// character comes out of the projection at the level they asked for.
func TestDeclaringALevelIsTakingIt(t *testing.T) {
	b := build(t).
		add("class", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "rogue"), Level: 1}).
		add("desired level", domain.Event{Type: domain.EventChange, Changes: []domain.Change{
			{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(3)},
		}})

	log := b.log()
	for _, e := range log.Events {
		if e.Type == domain.EventLevel && len(e.Choices) == 0 {
			t.Errorf("entry %d takes a level, which nothing asks for any more", e.Seq)
		}
	}

	cat, err := b.s.Catalog(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	state, err := domain.Project(log, cat)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if state.Identity.Level() != 3 {
		t.Errorf("level = %d, want 3", state.Identity.Level())
	}
	// And a level event taking a level is refused, not quietly recorded.
	if _, err := b.s.Apply(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq,
		domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "rogue"), Level: 4},
	); err == nil {
		t.Error("a level event was accepted, so there are two ways to advance")
	}
}

// The desired level is bounded by where the 2014 rules stop.
func TestDesiredLevelIsBounded(t *testing.T) {
	for _, level := range []int{0, 21} {
		b := build(t)
		_, err := b.s.Apply(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq,
			domain.Event{Type: domain.EventChange, Changes: []domain.Change{
				{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(level)},
			}})
		if err == nil {
			t.Errorf("a desired level of %d was accepted", level)
		}
	}
}

// The only ruleset a change can set is the compendium's own, which is what
// makes the rules selection final.
func TestRulesetMustBeTheCompendiums(t *testing.T) {
	b := build(t)
	_, err := b.s.Apply(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq,
		domain.Event{Type: domain.EventChange, Changes: []domain.Change{
			{Path: "identity.ruleset", Op: domain.OpSet, Value: domain.SlugValue("2024")},
		}})
	if err == nil {
		t.Error("a ruleset the compendium does not serve was accepted")
	}

	b.add("ruleset", domain.Event{Type: domain.EventChange, Changes: []domain.Change{
		{Path: "identity.ruleset", Op: domain.OpSet, Value: domain.SlugValue("2014")},
	}})
}

// Answering a class's own level-1 grant after the levels have been declared.
//
// A rogue built towards 5 is level 5 the moment they say so, which is before
// their four class skills are chosen. Those answers are still owed and must
// still land: the prompt that asks for them belongs to the level that granted
// them, not to the level the character has reached.
func TestClassGrantIsAnswerableAfterTheLevelsAreTaken(t *testing.T) {
	b := build(t).
		add("class", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "rogue"), Level: 1}).
		add("desired level", domain.Event{Type: domain.EventChange, Changes: []domain.Change{
			{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(5)},
		}})

	cat, err := b.s.Catalog(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	state, err := domain.Project(b.log(), cat)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if state.Identity.Level() != 5 {
		t.Fatalf("level = %d, want 5", state.Identity.Level())
	}

	// The skills the class grants on entry, answered now.
	b.add("skills", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "rogue"), Level: 1,
		Choices: []domain.Answer{answer("rogue/proficiency/0",
			"skill-perception", "skill-stealth", "skill-deception", "skill-persuasion")}})

	after, err := domain.Project(b.log(), cat)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if got := after.Skills.BySkill["stealth"].Proficiency; got == 0 {
		t.Error("the rogue is not proficient in Stealth, so the answer did not land")
	}

	// And Expertise, which doubles a proficiency the character holds, now
	// offers the skills that answer just granted.
	prompts, err := b.s.Prompts(context.Background(), testOwner, b.id, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Prompts() error = %v", err)
	}
	// And it offers the skills that answer just granted. Expertise doubles
	// what the character holds, so an empty offer here is the visible symptom
	// of an answer that went nowhere.
	for _, p := range prompts {
		if p.Choice.Prompt != "rogue-expertise-1/expertise/0" {
			continue
		}
		if len(p.Held) == 0 {
			t.Error("Expertise offers nothing to double, though four skills were just granted")
		}
		return
	}
	t.Error("Expertise never opened, so its skills cannot be doubled at all")
}
