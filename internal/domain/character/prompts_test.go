package character

import (
	"slices"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

func promptsFor(t *testing.T, log Log) []Prompt {
	t.Helper()
	got, err := Prompts(log, LoadCatalog(t))
	if err != nil {
		t.Fatalf("Prompts() error = %v", err)
	}
	return got
}

func promptIDs(prompts []Prompt) []rules.Slug {
	out := make([]rules.Slug, 0, len(prompts))
	for _, p := range prompts {
		out = append(out, p.Choice.Prompt)
	}
	return out
}

func firstRequired(prompts []Prompt) rules.Slug {
	for _, p := range prompts {
		if !p.Optional {
			return p.Choice.Prompt
		}
	}
	return ""
}

func has(prompts []Prompt, id rules.Slug) bool {
	return slices.Contains(promptIDs(prompts), id)
}

func find(t *testing.T, prompts []Prompt, id rules.Slug) Prompt {
	t.Helper()
	for _, p := range prompts {
		if p.Choice.Prompt == id {
			return p
		}
	}
	t.Fatalf("prompt %q not found in %v", id, promptIDs(prompts))
	return Prompt{}
}

func TestEmptyLogAsksForIdentityFirst(t *testing.T) {
	got := promptsFor(t, Log{})
	if firstRequired(got) != "character/init" {
		t.Errorf("first required prompt = %q, want character/init", firstRequired(got))
	}
	if Complete(got) {
		t.Error("an empty log reads as complete")
	}
}

// The flow's spine: each answer opens the next question, and nothing is
// skipped. This walks a character from nothing to a level-1 rogue.
func TestPromptsAdvanceAsAnswersArrive(t *testing.T) {
	at := time.Date(2026, time.August, 23, 0, 0, 0, 0, time.UTC)
	var log Log

	mustAppend := func(events ...Event) {
		t.Helper()
		if err := log.Append(events...); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	mustAppend(Event{Type: EventInit, At: at})
	if got := firstRequired(promptsFor(t, log)); got != "character/abilities" {
		t.Fatalf("after init, first required = %q, want character/abilities", got)
	}

	mustAppend(Event{Type: EventChange, At: at, Changes: []Change{
		{Path: "abilities.str", Op: OpSet, Value: IntValue(10)},
		{Path: "abilities.dex", Op: OpSet, Value: IntValue(15)},
		{Path: "abilities.con", Op: OpSet, Value: IntValue(13)},
		{Path: "abilities.int", Op: OpSet, Value: IntValue(10)},
		{Path: "abilities.wis", Op: OpSet, Value: IntValue(12)},
		{Path: "abilities.cha", Op: OpSet, Value: IntValue(12)},
	}})
	if got := firstRequired(promptsFor(t, log)); got != "character/race" {
		t.Fatalf("after abilities, first required = %q, want character/race", got)
	}

	// Choosing a race opens the prompts that race poses -- including one
	// that belongs to a trait and therefore did not exist a moment ago.
	mustAppend(Event{Type: EventRace, At: at, Ref: rules.NewRef(rules.RefRace, "half-elf")})
	got := promptsFor(t, log)
	for _, want := range []rules.Slug{
		"half-elf/ability-bonus/0",
		"half-elf/language/0",
		"skill-versatility/proficiency/0",
	} {
		if !has(got, want) {
			t.Errorf("after choosing half-elf, %q is not open; got %v", want, promptIDs(got))
		}
	}
	// Half-elf has no subraces, so no subrace prompt.
	if has(got, "character/subrace") {
		t.Error("half-elf was offered a subrace")
	}

	mustAppend(Event{Type: EventChange, At: at, Choices: []Answer{
		{Prompt: "half-elf/ability-bonus/0", Picks: []rules.Slug{"dex", "con"}},
		{Prompt: "half-elf/language/0", Picks: []rules.Slug{"goblin"}},
		{Prompt: "skill-versatility/proficiency/0", Picks: []rules.Slug{
			"skill-perception", "skill-acrobatics",
		}},
	}})
	if got := firstRequired(promptsFor(t, log)); got != "character/background" {
		t.Fatalf("after the race prompts, first required = %q, want character/background", got)
	}

	mustAppend(Event{Type: EventBackground, At: at, Ref: rules.NewRef(rules.RefBackground, "acolyte")})
	if got := firstRequired(promptsFor(t, log)); got != "character/class" {
		t.Fatalf("after background, first required = %q, want character/class", got)
	}

	mustAppend(Event{Type: EventClass, At: at, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 1})
	got = promptsFor(t, log)
	if !has(got, "rogue/proficiency/0") {
		t.Errorf("rogue's skill prompt is not open; got %v", promptIDs(got))
	}
	if !has(got, "rogue-expertise-1/expertise/0") {
		t.Errorf("Expertise is not open; got %v", promptIDs(got))
	}
	// Not yet: the subclass is a 3rd-level decision.
	if has(got, "rogue/subclass") {
		t.Error("a level-1 rogue was offered a subclass")
	}
}

// The nested-prompt fixed point: a choice inside a choice does not exist
// until its parent is answered. This is what the rogue's Expertise needs, and
// what makes a step counter impossible.
func TestAnsweringAPromptOpensItsNestedPrompt(t *testing.T) {
	log := RogueLog(t)

	got := promptsFor(t, log)
	if has(got, "rogue-expertise-1/expertise/0") {
		t.Error("Expertise is still open on a log that answered it")
	}
	// Both halves were answered in the fixture, so neither should be open.
	if has(got, "rogue-expertise-1/expertise/0/0") {
		t.Error("the nested Expertise prompt is still open")
	}

	// Strip the inner answer and the inner prompt must reappear -- and only
	// the inner one, because the outer is still answered.
	stripped := Log{}
	for _, e := range log.Events {
		e.Choices = slices.DeleteFunc(slices.Clone(e.Choices), func(a Answer) bool {
			return a.Prompt == "rogue-expertise-1/expertise/0/0"
		})
		stripped.Events = append(stripped.Events, e)
	}
	got = promptsFor(t, stripped)
	if !has(got, "rogue-expertise-1/expertise/0/0") {
		t.Errorf("the nested Expertise prompt did not reopen; got %v", promptIDs(got))
	}
	if has(got, "rogue-expertise-1/expertise/0") {
		t.Error("the outer Expertise prompt reopened, though it is answered")
	}
}

// A finished character is complete, and its one remaining prompt is the one
// that advances it. That is what makes creation and level-up one flow.
func TestFinishedCharacterIsCompleteAndCanAdvance(t *testing.T) {
	got := promptsFor(t, RogueLog(t))

	if !Complete(got) {
		var open []rules.Slug
		for _, p := range got {
			if !p.Optional {
				open = append(open, p.Choice.Prompt)
			}
		}
		t.Fatalf("the rogue is not complete; still required: %v", open)
	}

	advance := find(t, got, "character/level")
	if !advance.Advances {
		t.Error("character/level does not report that it advances")
	}
	if !advance.Optional {
		t.Error("character/level is not optional, so a finished character reads as unfinished")
	}
	if advance.Group != GroupAdvance {
		t.Errorf("character/level group = %s, want advance", advance.Group)
	}
	if advance.Event.Type != EventLevel {
		t.Errorf("character/level posts a %s event, want level", advance.Event.Type)
	}
}

// Prompts must not depend on the order the player answered in. If it did,
// going back a step could change what had been legal a moment earlier -- the
// exact trap that narrowing a prompt's options by what is already held would
// have set.
func TestPromptsAreOrderIndependent(t *testing.T) {
	at := time.Date(2026, time.August, 23, 0, 0, 0, 0, time.UTC)
	init := Event{Type: EventInit, At: at, Changes: []Change{
		{Path: "abilities.dex", Op: OpSet, Value: IntValue(15)},
	}}
	race := Event{Type: EventRace, At: at, Ref: rules.NewRef(rules.RefRace, "half-elf")}
	class := Event{Type: EventClass, At: at, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 1}

	var raceFirst, classFirst Log
	if err := raceFirst.Append(init, race, class); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := classFirst.Append(init, class, race); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	a := promptIDs(promptsFor(t, raceFirst))
	b := promptIDs(promptsFor(t, classFirst))
	slices.Sort(a)
	slices.Sort(b)
	if !slices.Equal(a, b) {
		t.Errorf("race-first prompts %v differ from class-first %v", a, b)
	}
}

// Options a character already holds are reported, not removed. The rogue
// picked Acrobatics and Perception through Skill Versatility, so the class's
// four-skill prompt must still offer them -- greyed out by the client -- and
// must still be the same prompt it was before.
func TestHeldOptionsAreReportedNotRemoved(t *testing.T) {
	at := time.Date(2026, time.August, 23, 0, 0, 0, 0, time.UTC)
	var log Log
	if err := log.Append(
		Event{Type: EventInit, At: at},
		Event{Type: EventRace, At: at, Ref: rules.NewRef(rules.RefRace, "half-elf"), Choices: []Answer{
			{Prompt: "skill-versatility/proficiency/0", Picks: []rules.Slug{
				"skill-perception", "skill-acrobatics",
			}},
		}},
		Event{Type: EventClass, At: at, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 1},
	); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	prompt := find(t, promptsFor(t, log), "rogue/proficiency/0")
	keys := rules.OptionKeys(prompt.Choice.From)
	if !slices.Contains(keys, "skill-acrobatics") {
		t.Error("the class prompt no longer offers Acrobatics; it must be reported held, not removed")
	}
	for _, want := range []rules.Slug{"skill-acrobatics", "skill-perception"} {
		if !slices.Contains(prompt.Held, want) {
			t.Errorf("held = %v, want it to include %q", prompt.Held, want)
		}
	}
	if slices.Contains(prompt.Held, "skill-stealth") {
		t.Error("Stealth is reported held, but the character does not have it")
	}
}

// The subclass prompt appears at the level the compendium says, derived from
// where the subclass's own advancement rows begin.
func TestSubclassPromptAppearsWhenDue(t *testing.T) {
	at := time.Date(2026, time.August, 23, 0, 0, 0, 0, time.UTC)
	build := func(level int) Log {
		t.Helper()
		var log Log
		events := []Event{
			{Type: EventInit, At: at},
			{Type: EventClass, At: at, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 1},
		}
		for l := 2; l <= level; l++ {
			events = append(events, Event{
				Type: EventLevel, At: at, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: l,
			})
		}
		if err := log.Append(events...); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
		return log
	}

	if has(promptsFor(t, build(2)), "rogue/subclass") {
		t.Error("a level-2 rogue was offered a subclass; the thief's rows begin at 3")
	}
	if !has(promptsFor(t, build(3)), "rogue/subclass") {
		t.Error("a level-3 rogue was not offered a subclass")
	}
}

// The Ability Score Improvement is synthesised, because the SRD data marks it
// only with a cumulative counter. It must appear at 4 and not at 3, and the
// answer must actually move a score.
func TestAbilityScoreImprovementIsOfferedAndApplied(t *testing.T) {
	at := time.Date(2026, time.August, 23, 0, 0, 0, 0, time.UTC)
	log := RogueLog(t)

	if has(promptsFor(t, log), "rogue/ability-score-improvement/4") {
		t.Error("a level-3 rogue was offered an Ability Score Improvement")
	}

	if err := log.Append(Event{
		Type: EventLevel, At: at, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 4,
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	got := promptsFor(t, log)
	asi := find(t, got, "rogue/ability-score-improvement/4")
	if asi.Optional {
		t.Error("the Ability Score Improvement is optional; it is not")
	}
	if asi.Level != 4 {
		t.Errorf("improvement level = %d, want 4", asi.Level)
	}

	// Choosing the ability branch opens the two-pick prompt inside it.
	if err := log.Append(Event{Type: EventChange, At: at, Choices: []Answer{
		{Prompt: "rogue/ability-score-improvement/4", Picks: []rules.Slug{
			"rogue/ability-score-improvement/4/0",
		}},
	}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if !has(promptsFor(t, log), "rogue/ability-score-improvement/4/0") {
		t.Error("the ability branch did not open its own prompt")
	}

	// Both picks into Dexterity is how "+2 to one ability" is expressed.
	if err := log.Append(Event{Type: EventChange, At: at, Choices: []Answer{
		{Prompt: "rogue/ability-score-improvement/4/0", Picks: []rules.Slug{"dex", "dex"}},
	}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	state, err := Project(log, LoadCatalog(t))
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if got := state.Abilities.Score(rules.Dexterity); got != 18 {
		t.Errorf("Dexterity = %d, want 18 (16 plus the improvement)", got)
	}
	// And the sheet keeps up: +4 Dexterity raises armor class and initiative.
	if state.Status.Initiative != 4 {
		t.Errorf("initiative = %+d, want +4", state.Status.Initiative)
	}
}

// Multiclassing is gated on ability scores, and on both sides: the class
// being left as well as the one being entered.
func TestAdvanceOffersOnlyEligibleClasses(t *testing.T) {
	at := time.Date(2026, time.August, 23, 0, 0, 0, 0, time.UTC)
	build := func(scores map[rules.Ability]int) []Prompt {
		t.Helper()
		changes := make([]Change, 0, len(scores))
		for ability, score := range scores {
			changes = append(changes, Change{
				Path: Path("abilities." + ability.Slug().String()), Op: OpSet, Value: IntValue(score),
			})
		}
		slices.SortFunc(changes, func(a, b Change) int {
			if a.Path < b.Path {
				return -1
			}
			return 1
		})
		var log Log
		if err := log.Append(
			Event{Type: EventInit, At: at, Changes: changes},
			Event{Type: EventClass, At: at, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 1},
		); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
		return promptsFor(t, log)
	}

	// A rogue with Dexterity 8 may not leave, so no other class is on offer
	// -- but staying a rogue always is.
	weak := find(t, build(map[rules.Ability]int{rules.Dexterity: 8, rules.Strength: 16}), "character/level")
	weakKeys := rules.OptionKeys(weak.Choice.From)
	if !slices.Contains(weakKeys, "rogue") {
		t.Error("a rogue was not offered another rogue level")
	}
	if slices.Contains(weakKeys, "fighter") {
		t.Error("a rogue with Dexterity 8 was offered fighter; they cannot leave rogue")
	}

	// With Dexterity 15 they may leave, and Strength 16 lets them into
	// fighter -- but Intelligence 8 keeps them out of wizard.
	able := find(t, build(map[rules.Ability]int{
		rules.Dexterity: 15, rules.Strength: 16, rules.Intelligence: 8,
	}), "character/level")
	ableKeys := rules.OptionKeys(able.Choice.From)
	if !slices.Contains(ableKeys, "fighter") {
		t.Error("a rogue with Dexterity 15 and Strength 16 was not offered fighter")
	}
	if slices.Contains(ableKeys, "wizard") {
		t.Error("a rogue with Intelligence 8 was offered wizard")
	}
}

// At level 20 there is nowhere to go, and the prompt that would say otherwise
// must disappear rather than offer an illegal level.
func TestNoAdvancePromptAtMaximumLevel(t *testing.T) {
	at := time.Date(2026, time.August, 23, 0, 0, 0, 0, time.UTC)
	var log Log
	events := []Event{
		{Type: EventInit, At: at},
		{Type: EventClass, At: at, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 1},
	}
	for level := 2; level <= maxCharacterLevel; level++ {
		events = append(events, Event{
			Type: EventLevel, At: at, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: level,
		})
	}
	if err := log.Append(events...); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if has(promptsFor(t, log), "character/level") {
		t.Error("a level-20 character was offered another level")
	}
}
