package character_test

import (
	"context"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// Two ability bonuses that look identical and are not the same rule.
//
// A level's Ability Score Improvement is "+2 to one ability, or +1 to two", so
// spending both points on Dexterity is a legal answer. A half-elf's is "+1 to
// two *different* scores", and it is the same choice kind over the same
// ability-bonus options. Both the validator and the build screen used to key
// the rule on that kind, which meant a half-elf could take +2 to one score.
func TestOnlyTheImprovementTakesBothPointsInOneScore(t *testing.T) {
	ctx := context.Background()

	// The half-elf's, which must refuse.
	elf := build(t)
	_, err := elf.s.Apply(ctx, testOwner, elf.id, rules.DefaultLocale, elf.seq,
		domain.Event{Type: domain.EventRace, Ref: ref(rules.RefRace, "half-elf"),
			Choices: []domain.Answer{answer("half-elf/ability-bonus/0", "dex", "dex")}})
	if err == nil {
		t.Error("a half-elf put both bonuses into one score, which is not two different scores")
	}

	// The improvement's, which must accept -- and land as +2.
	rogue := build(t).
		add("class", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "rogue"), Level: 1}).
		add("desired level", domain.Event{Type: domain.EventChange, Changes: []domain.Change{
			{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(4)},
		}})

	before := scoreOf(t, rogue, rules.Dexterity)
	// Both answers in one event, parent then branch, which is what the build
	// screen posts now that a branch is answered in the card that offered it.
	rogue.add("improvement", domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "rogue"), Level: 4,
		Choices: []domain.Answer{
			answer("rogue/ability-score-improvement/4", "rogue/ability-score-improvement/4/0"),
			answer("rogue/ability-score-improvement/4/0", "dex", "dex"),
		}})

	if got := scoreOf(t, rogue, rules.Dexterity); got != before+2 {
		t.Errorf("dexterity = %d, want %d: two points into one score is +2", got, before+2)
	}
}

func scoreOf(t *testing.T, b *builder, ability rules.Ability) int {
	t.Helper()
	cat, err := b.s.Catalog(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	state, err := domain.Project(b.log(), cat)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	return state.Abilities.Score(ability)
}
