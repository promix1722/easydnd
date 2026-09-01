package character

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// The fixtures below are exported so that both this package's internal tests
// and the external character_test package can share one definition. A second
// transcription of the same character is a second chance to get it wrong, and
// the first one already was wrong in four different ways.

// One Source for the whole test binary. Source.Load caches per locale, so
// this turns the two dozen reads of the 1.55 MB compendium these tests used to
// make -- one of them inside a seven-case subtest loop -- into a single one. A
// fresh Source per call threw that cache away. Sharing is safe for the reason
// the cache is: a Catalog is immutable, and Load is mutex-guarded.
var catalogSource = catalogfile.NewSource(filepath.Join("..", "..", "..", "data", "srd_5.1"))

// LoadCatalog loads the committed compendium.
//
// The tests load the real data rather than a hand-built stub because a stub
// would only prove the code agrees with itself, and every number these tests
// check comes from the compendium. The import of the file adapter is
// test-only: `make lint/layers` runs `go list -deps` without -test, so the
// domain's stdlib-only rule is untouched.
func LoadCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalogSource.Load(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatalf("loading the compendium: %v", err)
	}
	return c
}

// RogueLog builds the level-3 half-elf rogue from
// docs/reference_hexsheet/rouge_3_level.json as a sequence of events.
//
// Using a real exported sheet rather than an invented one is the point: it is
// what proves the event vocabulary can express a character somebody actually
// plays, instead of only the ones the model was designed around.
//
// Three things about this transcription are worth knowing, because the first
// draft of it got all three wrong and nothing caught them -- the log
// validated, and validating a log says nothing about whether it means what it
// claims to.
//
// The scores below are the *base* array, not the sheet's. An init event
// records what the player generated; Project applies the racial bonuses. So
// 10/15/13/10/12/12 plus the half-elf's fixed +2 Charisma and two chosen +1s
// projects to the exported 10/16/14/10/12/14. Recording the final numbers
// instead would double-count the race and would mean going back a step could
// not change it.
//
// The two extra skills come from the trait skill-versatility, not from the
// race: half-elf has no proficiency prompt of its own. And the background is
// acolyte because SRD 5.1 publishes exactly one background -- the real
// character's Urchin does not exist here, which is why the projection has
// Insight and Religion where the export has a disguise kit.
func RogueLog(t *testing.T) Log {
	t.Helper()
	at := time.Date(2026, time.August, 23, 14, 27, 51, 0, time.UTC)

	var log Log
	err := log.Append(
		Event{
			Type: EventInit,
			At:   at,
			Changes: []Change{
				{Path: "identity.name", Op: OpSet, Value: StringValue("Сахарок")},
				{Path: "identity.alignment", Op: OpSet, Value: SlugValue("neutral")},
				{Path: "abilities.method", Op: OpSet, Value: SlugValue("point-buy")},
				{Path: "abilities.str", Op: OpSet, Value: IntValue(10)},
				{Path: "abilities.dex", Op: OpSet, Value: IntValue(15)},
				{Path: "abilities.con", Op: OpSet, Value: IntValue(13)},
				{Path: "abilities.int", Op: OpSet, Value: IntValue(10)},
				{Path: "abilities.wis", Op: OpSet, Value: IntValue(12)},
				{Path: "abilities.cha", Op: OpSet, Value: IntValue(12)},
			},
		},
		Event{
			Type: EventRace,
			At:   at,
			Ref:  rules.NewRef(rules.RefRace, "half-elf"),
			Choices: []Answer{
				// Charisma is the half-elf's fixed +2 and is not on offer
				// here; the prompt chooses two of the other five.
				{Prompt: "half-elf/ability-bonus/0", Picks: []rules.Slug{"dex", "con"}},
				// Skill Versatility is a trait, so its prompt only exists
				// once the race has been chosen -- which is why an answer to
				// it may arrive in this event or any later one.
				{Prompt: "skill-versatility/proficiency/0", Picks: []rules.Slug{
					"skill-perception", "skill-acrobatics",
				}},
			},
			// half-elf/language/0 is deliberately unanswered: the export
			// reads "Common, Elvish, One language of your choice", so the
			// third language was never picked. A partial answer set has to
			// project cleanly.
		},
		Event{
			Type: EventBackground,
			At:   at,
			Ref:  rules.NewRef(rules.RefBackground, "acolyte"),
		},
		Event{
			Type:  EventClass,
			At:    at,
			Ref:   rules.NewRef(rules.RefClass, "rogue"),
			Level: 1,
			Choices: []Answer{
				{Prompt: "rogue/proficiency/0", Picks: []rules.Slug{
					"skill-deception", "skill-persuasion", "skill-sleight-of-hand", "skill-stealth",
				}},
				// Expertise: two of the skills the rogue is proficient in,
				// asked as one list by oneList.
				{Prompt: "rogue-expertise-1/expertise/0", Picks: []rules.Slug{
					"skill-persuasion", "skill-stealth",
				}},
				{Prompt: "rogue/starting-equipment/0", Picks: []rules.Slug{"rapier"}},
				// The shortbow-and-arrows bundle has no slug of its own, so
				// it is named by what is in it.
				{Prompt: "rogue/starting-equipment/1", Picks: []rules.Slug{"shortbow+arrow"}},
				{Prompt: "rogue/starting-equipment/2", Picks: []rules.Slug{"burglars-pack"}},
			},
		},
		// A rogue's kit includes leather armor, but wearing it is a decision
		// no rule makes for you -- and armor class depends on it.
		Event{
			Type: EventChange,
			At:   at,
			Changes: []Change{
				{Path: "equipment.equipped", Op: OpAdd, Value: SlugValue("leather-armor")},
			},
		},
		// Level 2 grants Cunning Action: the bonus-action Dash, Disengage and
		// Hide that DND.md described as "bonus hide action from rouge".
		Event{Type: EventLevel, At: at, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 2},
		Event{
			Type:  EventSubclass,
			At:    at,
			Ref:   rules.NewRef(rules.RefSubclass, "thief"),
			Level: 3,
		},
		Event{Type: EventLevel, At: at, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 3},
	)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	return log
}
