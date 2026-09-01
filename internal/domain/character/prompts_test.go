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

// required is every prompt a character cannot be finished without.
func required(prompts []Prompt) []rules.Slug {
	var out []rules.Slug
	for _, p := range prompts {
		if !p.Optional {
			out = append(out, p.Choice.Prompt)
		}
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
	// The identity stage is three questions: who they are, under which rules,
	// and to what level. All three are required, and they come in that order.
	if got := firstRequired(promptsFor(t, log)); got != "character/ruleset" {
		t.Fatalf("after init, first required = %q, want character/ruleset", got)
	}

	mustAppend(Event{Type: EventChange, At: at, Changes: []Change{
		{Path: "identity.ruleset", Op: OpSet, Value: SlugValue("2014")},
	}})
	if got := firstRequired(promptsFor(t, log)); got != "character/desired-level" {
		t.Fatalf("after the ruleset, first required = %q, want character/desired-level", got)
	}

	mustAppend(Event{Type: EventChange, At: at, Changes: []Change{
		{Path: "identity.desiredLevel", Op: OpSet, Value: IntValue(1)},
	}})
	if got := firstRequired(promptsFor(t, log)); got != "character/abilities" {
		t.Fatalf("after the desired level, first required = %q, want character/abilities", got)
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

// A finished character is complete, and levelling up is one declaration: the
// desired level *is* the level, and what those levels open is what is asked.
// That is what makes creation and level-up one flow.
func TestDeclaringALevelIsTakingIt(t *testing.T) {
	at := time.Date(2026, time.August, 23, 0, 0, 0, 0, time.UTC)
	log := RogueLog(t)
	// RogueLog predates both identity questions, as an imported log does.
	// Answering the ruleset leaves the desired level as the only thing
	// outstanding, which is what the rest of this walks.
	if err := log.Append(Event{Type: EventChange, At: at, Changes: []Change{
		{Path: "identity.ruleset", Op: OpSet, Value: SlugValue("2014")},
	}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	got := promptsFor(t, log)

	if want := []rules.Slug{"character/desired-level"}; !slices.Equal(required(got), want) {
		t.Fatalf("still required: %v, want %v", required(got), want)
	}
	// Nothing anywhere asks which class a level goes into.
	if has(got, "character/level") {
		t.Error("something still offers a level to take")
	}

	declare := func(level int) []Prompt {
		t.Helper()
		if err := log.Append(Event{Type: EventChange, At: at, Changes: []Change{
			{Path: "identity.desiredLevel", Op: OpSet, Value: IntValue(level)},
		}}); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
		return promptsFor(t, log)
	}

	// The rogue's log already takes three levels the old way, so declaring
	// three changes nothing and the character reads as finished.
	atGoal := declare(3)
	if !Complete(atGoal) {
		t.Errorf("a character at their desired level is not complete; required: %v", required(atGoal))
	}

	// Declaring four *is* the fourth level, and what it opens is the Ability
	// Score Improvement it grants -- required, and asked for that level.
	got = declare(4)
	state, err := Project(log, LoadCatalog(t))
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if state.Identity.Level() != 4 {
		t.Errorf("level = %d, want 4: the declaration is the level", state.Identity.Level())
	}
	if has(got, "character/level") {
		t.Error("declaring a level asked which class it went into")
	}
	improvement := find(t, got, "rogue/ability-score-improvement/4")
	if improvement.Optional {
		t.Error("the improvement a level grants is optional")
	}
	if improvement.Level != 4 {
		t.Errorf("improvement level = %d, want 4", improvement.Level)
	}
	if Complete(got) {
		t.Error("a character owing their new level's improvement reads as complete")
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

// Expertise offers only the skills the character is trained in.
//
// HeldOnly inverts what Held means -- those are the only legal answers rather
// than the illegal ones -- so Held is what the Expertise prompt is made of.
// The projector now puts all eighteen skills on the sheet, untrained ones
// included, and holds() has to keep reading the training level rather than
// mere presence: if it ever stopped, this prompt would quietly offer every
// skill in the game and Expertise would become free.
func TestExpertiseOffersOnlyTheSkillsAlreadyTrained(t *testing.T) {
	at := time.Date(2026, time.August, 23, 0, 0, 0, 0, time.UTC)
	var log Log
	if err := log.Append(
		Event{Type: EventInit, At: at},
		Event{Type: EventClass, At: at, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 1,
			Choices: []Answer{
				{Prompt: "rogue/proficiency/0", Picks: []rules.Slug{
					"skill-deception", "skill-persuasion", "skill-sleight-of-hand", "skill-stealth",
				}},
				{Prompt: "rogue-expertise-1/expertise/0", Picks: []rules.Slug{
					"rogue-expertise-1/expertise/0/0",
				}},
			}},
	); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	prompt := find(t, promptsFor(t, log), "rogue-expertise-1/expertise/0/0")
	if !prompt.HeldOnly {
		t.Fatal("the expertise prompt is not heldOnly; Held would read as the illegal answers")
	}
	for _, want := range []rules.Slug{"skill-stealth", "skill-persuasion"} {
		if !slices.Contains(prompt.Held, want) {
			t.Errorf("held = %v, want it to include %q", prompt.Held, want)
		}
	}
	// Nothing trained these, so doubling them is not on offer.
	for _, notHeld := range []rules.Slug{"skill-arcana", "skill-athletics", "skill-perception"} {
		if slices.Contains(prompt.Held, notHeld) {
			t.Errorf("held = %v, want it to exclude %q: nothing trained it", prompt.Held, notHeld)
		}
	}
	// The four the class granted, and not one row per skill in the game.
	if len(prompt.Held) != 4 {
		t.Errorf("held %d options, want the 4 trained skills: %v", len(prompt.Held), prompt.Held)
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

// Twentieth level is where the 2014 rules stop, and declaring it must produce
// a twentieth-level character rather than run off the end of the class table.
func TestDeclaringTheMaximumLevel(t *testing.T) {
	at := time.Date(2026, time.August, 23, 0, 0, 0, 0, time.UTC)
	var log Log
	if err := log.Append(
		Event{Type: EventInit, At: at},
		Event{Type: EventClass, At: at, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 1},
		Event{Type: EventChange, At: at, Changes: []Change{
			{Path: "identity.desiredLevel", Op: OpSet, Value: IntValue(MaxCharacterLevel)},
		}},
	); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	state, err := Project(log, LoadCatalog(t))
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if state.Identity.Level() != MaxCharacterLevel {
		t.Errorf("level = %d, want %d", state.Identity.Level(), MaxCharacterLevel)
	}
	if has(promptsFor(t, log), "character/level") {
		t.Error("a level-20 character was offered another level")
	}
}

// The four roleplaying questions are asked as text, in their own group.
//
// They used to be the SRD's d8 tables offered as options, and the state behind
// them was free text all along -- so the menu was the compendium answering a
// question that is nobody's but the player's. What this pins is that the
// prompt offers nothing to pick between, which is how a client tells a
// question it writes an answer to from a question it picks one for.
func TestRoleplayingPromptsAreTextInTheirOwnGroup(t *testing.T) {
	at := time.Date(2026, time.August, 26, 0, 0, 0, 0, time.UTC)
	var log Log
	if err := log.Append(
		Event{Type: EventInit, At: at},
		Event{Type: EventBackground, At: at, Ref: rules.NewRef(rules.RefBackground, "acolyte")},
	); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	got := promptsFor(t, log)
	for _, want := range []struct {
		id     rules.Slug
		kind   rules.ChoiceKind
		choose int
	}{
		{"character/personality-trait", rules.ChoosePersonality, 1},
		{"character/ideal", rules.ChooseIdeal, 1},
		{"character/bond", rules.ChooseBond, 1},
		{"character/flaw", rules.ChooseFlaw, 1},
	} {
		p := find(t, got, want.id)
		if p.Choice.Kind != want.kind {
			t.Errorf("%s kind = %v, want %v", want.id, p.Choice.Kind, want.kind)
		}
		// One, whatever the background's table suggests: the count belonged to
		// a menu, and what is asked now is one answer in the player's words.
		if p.Choice.Choose != want.choose {
			t.Errorf("%s chooses %d, want %d", want.id, p.Choice.Choose, want.choose)
		}
		if len(p.Choice.From.Options) != 0 {
			t.Errorf("%s offers %d options; it is written, not picked",
				want.id, len(p.Choice.From.Options))
		}
		if p.Group != GroupPersonality {
			t.Errorf("%s is in group %v, want personality", want.id, p.Group)
		}
		if !p.Optional {
			t.Errorf("%s is required; a character is complete without one", want.id)
		}
	}

	// The alignment moved with them: it is who the character is, not what
	// their background was.
	if p := find(t, got, "character/alignment"); p.Group != GroupPersonality {
		t.Errorf("alignment is in group %v, want personality", p.Group)
	}

	// Acolyte's own questions stayed where they were.
	if p := find(t, got, "acolyte/language/0"); p.Group != GroupBackground {
		t.Errorf("acolyte's language prompt is in group %v, want background", p.Group)
	}
}

// Writing the answer is what closes the question, exactly as it is for the six
// ability scores -- there are no picks to compare against an option set, so a
// prompt that stayed open after being answered would be asked forever.
func TestWritingATraitClosesItsPrompt(t *testing.T) {
	at := time.Date(2026, time.August, 26, 0, 0, 0, 0, time.UTC)
	var log Log
	if err := log.Append(
		Event{Type: EventInit, At: at},
		Event{Type: EventBackground, At: at, Ref: rules.NewRef(rules.RefBackground, "acolyte")},
		Event{Type: EventChange, At: at, Changes: []Change{
			{Path: "identity.bonds", Op: OpSet, Value: StringValue("I owe the temple everything.")},
		}},
	); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	got := promptsFor(t, log)
	if has(got, "character/bond") {
		t.Error("the bond is still being asked for after being written")
	}
	// And nothing else went with it.
	if !has(got, "character/flaw") {
		t.Errorf("the flaw stopped being asked too; got %v", promptIDs(got))
	}

	state, err := Project(log, LoadCatalog(t))
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if got, want := state.Identity.Bonds, []string{"I owe the temple everything."}; len(got) != 1 || got[0] != want[0] {
		t.Errorf("bonds = %v, want %v", got, want)
	}
}

// A background chosen after the words were written does not wipe them. The
// projector used to assign all four from the picked suggestion, which meant
// applyBackground overwrote whatever had been typed.
func TestChoosingABackgroundKeepsWhatWasWritten(t *testing.T) {
	at := time.Date(2026, time.August, 26, 0, 0, 0, 0, time.UTC)
	var log Log
	if err := log.Append(
		Event{Type: EventInit, At: at},
		Event{Type: EventChange, At: at, Changes: []Change{
			{Path: "identity.flaws", Op: OpSet, Value: StringValue("I cannot let a bet go.")},
		}},
		Event{Type: EventBackground, At: at, Ref: rules.NewRef(rules.RefBackground, "acolyte")},
	); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	state, err := Project(log, LoadCatalog(t))
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if got := state.Identity.Flaws; len(got) != 1 || got[0] != "I cannot let a bet go." {
		t.Errorf("flaws = %v, want the one that was written", got)
	}
}
