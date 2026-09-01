package character_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// The fixtures below are *captured*, not transcribed: every log a test
// revises was written by driving the ordinary build flow through Apply, one
// entry per selection, exactly as a client does.
//
// That is the whole point. A hand-written log is a log nothing ever produced,
// and a replay test on one proves only that the replay agrees with whoever
// typed the fixture. Capturing means these tests fail if the build flow and
// the replay ever stop agreeing about what an entry means -- which is the
// failure this feature exists to avoid.

type builder struct {
	t   *testing.T
	s   *charuc.Service
	id  domain.ID
	seq int
}

func build(t *testing.T) *builder {
	t.Helper()
	s := newService(t)
	c := mustCreateScored(t, s)
	return &builder{t: t, s: s, id: c.ID, seq: c.Log.LastSeq()}
}

// add appends one entry, failing the test if the build flow would not have
// accepted it. A fixture that cannot be built is not a fixture.
func (b *builder) add(label string, event domain.Event) *builder {
	b.t.Helper()
	seq, err := b.s.Apply(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq, event)
	if err != nil {
		b.t.Fatalf("building %s: %v", label, err)
	}
	b.seq = seq
	return b
}

func (b *builder) log() domain.Log {
	b.t.Helper()
	c, err := b.s.Get(context.Background(), testOwner, b.id)
	if err != nil {
		b.t.Fatalf("Get() error = %v", err)
	}
	return c.Log
}

func ref(kind rules.RefKind, slug rules.Slug) rules.Ref { return rules.NewRef(kind, slug) }

func answer(prompt rules.Slug, picks ...rules.Slug) domain.Answer {
	return domain.Answer{Prompt: prompt, Picks: picks}
}

// rogue3 is the level-3 half-elf rogue, one entry per selection:
//
//	1 init        the name
//	2 change      the six scores and the method
//	3 race        half-elf, and the two ability bonuses it offers
//	4 race        the two skills Skill Versatility grants
//	5 background  acolyte
//	6 class       rogue, and its four skill proficiencies
//	7 level       the level-1 Expertise, doubling two of the acolyte's skills
//	8 class       the rapier
//	9 change      the declared level, which is what makes them third
//	10 subclass   thief, due at third
func rogue3(t *testing.T) *builder {
	t.Helper()
	return build(t).
		add("race", domain.Event{Type: domain.EventRace, Ref: ref(rules.RefRace, "half-elf"),
			Choices: []domain.Answer{answer("half-elf/ability-bonus/0", "dex", "con")}}).
		add("skill versatility", domain.Event{Type: domain.EventRace, Ref: ref(rules.RefRace, "half-elf"),
			Choices: []domain.Answer{
				answer("skill-versatility/proficiency/0", "skill-acrobatics", "skill-investigation")}}).
		add("background", domain.Event{Type: domain.EventBackground, Ref: ref(rules.RefBackground, "acolyte")}).
		add("class", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "rogue"), Level: 1,
			Choices: []domain.Answer{answer("rogue/proficiency/0",
				"skill-perception", "skill-stealth", "skill-deception", "skill-persuasion")}}).
		add("expertise", domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "rogue"), Level: 1,
			Choices: []domain.Answer{
				answer("rogue-expertise-1/expertise/0", "skill-insight", "skill-religion"),
			}}).
		add("equipment", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "rogue"), Level: 1,
			Choices: []domain.Answer{answer("rogue/starting-equipment/0", "rapier")}}).
		add("level 3", domain.Event{Type: domain.EventChange, Changes: []domain.Change{
			{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(3)}}}).
		add("subclass", domain.Event{Type: domain.EventSubclass, Ref: ref(rules.RefSubclass, "thief"), Level: 3})
}

// Assertion helpers.

// revised runs Revise and insists the rebuilt log is one a store would accept.
// Every case below goes through here, because a rebuild that does not
// validate is a character nobody can open again.
func revised(
	t *testing.T, b *builder, targetSeq int, replacement *domain.Event,
) (domain.Log, []charuc.Dropped) {
	t.Helper()
	cat, err := b.s.Catalog(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	out, dropped, err := charuc.Revise(b.log(), cat, targetSeq, replacement)
	if err != nil {
		t.Fatalf("Revise() error = %v", err)
	}
	if err := out.Validate(); err != nil {
		t.Fatalf("the rebuilt log does not validate: %v", err)
	}
	return out, dropped
}

func droppedAt(dropped []charuc.Dropped, seq int) (charuc.Dropped, bool) {
	for _, d := range dropped {
		if d.Seq == seq {
			return d, true
		}
	}
	return charuc.Dropped{}, false
}

func hasAnswer(log domain.Log, prompt rules.Slug) bool {
	for _, e := range log.Events {
		for _, a := range e.Choices {
			if a.Prompt == prompt {
				return true
			}
		}
	}
	return false
}

func openIn(prompts []domain.Prompt, id rules.Slug) (domain.Prompt, bool) {
	for _, p := range prompts {
		if p.Choice.Prompt == id {
			return p, true
		}
	}
	return domain.Prompt{}, false
}

// The plain case: an entry nothing else depends on, swapped for another.
func TestReviseReplacesAnEntryWithNoDependants(t *testing.T) {
	b := rogue3(t)
	before := b.log()

	out, dropped := revised(t, b, 8, &domain.Event{
		Type: domain.EventClass, Ref: ref(rules.RefClass, "rogue"), Level: 1,
		Choices: []domain.Answer{answer("rogue/starting-equipment/0", "shortsword")},
	})

	if len(dropped) != 0 {
		t.Errorf("dropped = %+v, want nothing: nothing depends on which blade it is", dropped)
	}
	if out.Len() != before.Len() {
		t.Errorf("log length = %d, want %d", out.Len(), before.Len())
	}
	got := out.Events[7].Choices
	if len(got) != 1 || !slices.Contains(got[0].Picks, "shortsword") {
		t.Errorf("entry 8 = %+v, want the shortsword", got)
	}
	// And the entries around it are untouched, seq and all.
	if out.Events[6].Seq != 7 || out.Events[8].Seq != 9 {
		t.Errorf("neighbours renumbered: %d, %d", out.Events[6].Seq, out.Events[8].Seq)
	}
}

// The regression test for the bug this closes. subrace:hill-dwarf resolves
// perfectly well in the compendium; what makes it wrong on a half-elf is that
// nothing is asking. Before answersAnOpenPrompt existed, the append that put
// it there was accepted, and a replay had no way to notice.
func TestReviseDropsASubraceTheNewRaceDoesNotOffer(t *testing.T) {
	b := build(t).
		add("race", domain.Event{Type: domain.EventRace, Ref: ref(rules.RefRace, "dwarf")}).
		add("subrace", domain.Event{Type: domain.EventSubrace, Ref: ref(rules.RefSubrace, "hill-dwarf")}).
		add("tools", domain.Event{Type: domain.EventRace, Ref: ref(rules.RefRace, "dwarf"),
			Choices: []domain.Answer{answer("tool-proficiency/proficiency/0", "smiths-tools")}}).
		add("background", domain.Event{Type: domain.EventBackground, Ref: ref(rules.RefBackground, "acolyte")})

	out, dropped := revised(t, b, 3, &domain.Event{
		Type: domain.EventRace, Ref: ref(rules.RefRace, "half-elf"),
	})

	subrace, ok := droppedAt(dropped, 4)
	if !ok {
		t.Fatalf("dropped = %+v, want the orphaned subrace at 4", dropped)
	}
	if subrace.Reason != charuc.DropNotOffered {
		t.Errorf("reason = %q, want not-offered", subrace.Reason)
	}
	if subrace.Source != domain.GroupRace {
		t.Errorf("source = %q, want race: the report groups by the question, not the type", subrace.Source)
	}
	// The dwarf's own tool proficiency goes with it, for the same reason.
	if tools, ok := droppedAt(dropped, 5); !ok || tools.Reason != charuc.DropNotOffered {
		t.Errorf("dropped = %+v, want the dwarf's tool entry gone too", dropped)
	}
	// The background survived: it never depended on the race.
	if out.Len() != 4 {
		t.Fatalf("log = %d entries, want 4", out.Len())
	}
	if out.Events[3].Type != domain.EventBackground {
		t.Errorf("last entry = %s, want the background", out.Events[3].Type)
	}
}

// What a dropped answer becomes. Not nothing: the question it answered is
// asked again, in the group it belongs to, which is what makes changing a
// race a thing a player can recover from rather than a thing that quietly
// costs them a spell.
func TestReviseReturnsDroppedChoicesOutstandingUnderTheirGroup(t *testing.T) {
	b := build(t).
		add("race", domain.Event{Type: domain.EventRace, Ref: ref(rules.RefRace, "elf")}).
		add("subrace", domain.Event{Type: domain.EventSubrace, Ref: ref(rules.RefSubrace, "high-elf")}).
		add("cantrip", domain.Event{Type: domain.EventRace, Ref: ref(rules.RefRace, "elf"),
			Choices: []domain.Answer{answer("high-elf-cantrip/spell/0", "light")}}).
		add("class", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "rogue"), Level: 1,
			Choices: []domain.Answer{answer("rogue/proficiency/0",
				"skill-acrobatics", "skill-stealth", "skill-deception", "skill-persuasion")}})

	cantripSeq := 5
	if !hasAnswer(b.log(), "high-elf-cantrip/spell/0") {
		t.Fatal("the fixture never recorded the cantrip")
	}

	out, dropped := revised(t, b, 3, &domain.Event{
		Type: domain.EventRace, Ref: ref(rules.RefRace, "half-elf"),
		Choices: []domain.Answer{answer("half-elf/ability-bonus/0", "dex", "con")},
	})

	cantrip, ok := droppedAt(dropped, cantripSeq)
	if !ok {
		t.Fatalf("dropped = %+v, want the cantrip entry at %d", dropped, cantripSeq)
	}
	if cantrip.Source != domain.GroupRace {
		t.Errorf("cantrip source = %q, want race", cantrip.Source)
	}
	if hasAnswer(out, "high-elf-cantrip/spell/0") {
		t.Error("the rebuilt log still answers a prompt no race poses")
	}

	// And the new race's own questions are open, under race, waiting.
	prompts := promptsFor(t, b, out)
	skills, ok := openIn(prompts, "skill-versatility/proficiency/0")
	if !ok {
		t.Fatalf("the half-elf's own skill choice is not outstanding: %v", promptIDs(prompts))
	}
	if skills.Group != domain.GroupRace {
		t.Errorf("group = %s, want race", skills.Group)
	}
	// The class survived the whole thing, picks and all.
	if !hasAnswer(out, "rogue/proficiency/0") {
		t.Error("the rogue lost its skills to a race change")
	}
}

// promptsFor asks what a rebuilt log still leaves open, without storing it.
func promptsFor(t *testing.T, b *builder, log domain.Log) []domain.Prompt {
	t.Helper()
	cat, err := b.s.Catalog(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	prompts, err := domain.Prompts(log, cat)
	if err != nil {
		t.Fatalf("Prompts() error = %v", err)
	}
	return prompts
}

func promptIDs(prompts []domain.Prompt) []rules.Slug {
	out := make([]rules.Slug, 0, len(prompts))
	for _, p := range prompts {
		out = append(out, p.Choice.Prompt)
	}
	return out
}

// The most important test in the change.
//
// An elf grants Perception outright, so a half-elf rogue who picked Perception
// as one of their four class skills has an illegal answer the moment the race
// becomes elf. What must NOT happen is the class entry going with it: that
// would take the class, the other three skills, the Expertise and every level
// built on top, and a revalidation that silently eats a player's choices is
// worse than the truncation it replaces.
func TestReviseKeepsAnEntryThatLostAnAnswer(t *testing.T) {
	b := rogue3(t)
	before := b.log()

	out, dropped := revised(t, b, 3, &domain.Event{
		Type: domain.EventRace, Ref: ref(rules.RefRace, "elf"),
	})

	class, ok := droppedAt(dropped, 6)
	if !ok {
		t.Fatalf("dropped = %+v, want the class entry's lost answer reported", dropped)
	}
	if class.Reason != charuc.DropAnswersDropped {
		t.Fatalf("reason = %q, want answers-dropped: the entry is not gone", class.Reason)
	}
	if len(class.Lost) != 1 || class.Lost[0].Prompt != "rogue/proficiency/0" {
		t.Errorf("lost = %+v, want the four-skill answer", class.Lost)
	}
	if class.Lost[0].Rule != "held" {
		t.Errorf("rule = %q, want held: an elf already has Perception", class.Lost[0].Rule)
	}

	// The entry itself is alive, and so is everything hanging off it.
	stillARogue := false
	for _, e := range out.Events {
		if e.Type == domain.EventClass && e.Ref == ref(rules.RefClass, "rogue") {
			stillARogue = true
		}
	}
	if !stillARogue {
		t.Fatal("the class entry was deleted because one of its picks died")
	}
	if !hasAnswer(out, "rogue-expertise-1/expertise/0") {
		t.Error("Expertise went with it")
	}
	if !hasAnswer(out, "rogue/starting-equipment/0") {
		t.Error("the rapier went with it")
	}

	cat, _ := b.s.Catalog(context.Background(), rules.DefaultLocale)
	state, err := domain.Project(out, cat)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if len(state.Identity.Classes) != 1 || state.Identity.Classes[0].Level != 3 {
		t.Errorf("classes = %+v, want a level-3 rogue", state.Identity.Classes)
	}
	if state.Identity.Classes[0].Subclass != "thief" {
		t.Errorf("subclass = %q, want thief", state.Identity.Classes[0].Subclass)
	}

	// And the question comes back rather than vanishing.
	prompts := promptsFor(t, b, out)
	if skills, ok := openIn(prompts, "rogue/proficiency/0"); !ok {
		t.Errorf("the rogue's skills are not outstanding again: %v", promptIDs(prompts))
	} else if skills.Group != domain.GroupClass {
		t.Errorf("group = %s, want class", skills.Group)
	}

	// One entry did go, and it is the right one: the half-elf's own trait
	// answer, which an elf poses no prompt for. Exactly one -- the class,
	// the levels, the subclass and the background all stand.
	if orphan, ok := droppedAt(dropped, 4); !ok || orphan.Reason != charuc.DropNotOffered {
		t.Errorf("dropped = %+v, want the half-elf trait entry at 4 as not-offered", dropped)
	}
	if out.Len() != before.Len()-1 {
		t.Errorf("log length = %d, want %d", out.Len(), before.Len()-1)
	}
}

// A preview the player sat and thought about, while another tab moved the
// log. It must not commit against a log it never saw -- and it does not,
// because expectedSeq makes it the ordinary sequence conflict.
func TestAStalePreviewCannotBeCommitted(t *testing.T) {
	ctx := context.Background()
	b := rogue3(t)
	at := b.log().LastSeq()
	replacement := &domain.Event{Type: domain.EventRace, Ref: ref(rules.RefRace, "elf")}

	if _, err := b.s.Revise(ctx, testOwner, b.id, rules.DefaultLocale, at, 3, replacement, false); err != nil {
		t.Fatalf("dry run error = %v", err)
	}
	b.add("a note from the other tab", domain.Event{Type: domain.EventNote, Note: "meanwhile"})

	_, err := b.s.Revise(ctx, testOwner, b.id, rules.DefaultLocale, at, 3, replacement, true)
	var validation *types.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("commit error = %v, want a ValidationError", err)
	}
}

// Taking levels back, which is revising the one entry that declares them.
// Everything those levels bought goes with them: a thief was due at third,
// and a second-level rogue is not owed one.
func TestReviseLowersTheDeclaredLevel(t *testing.T) {
	b := rogue3(t)

	out, dropped := revised(t, b, 9, &domain.Event{
		Type:    domain.EventChange,
		Changes: []domain.Change{{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(2)}},
	})

	if d, ok := droppedAt(dropped, 10); !ok || d.Reason != charuc.DropNotOffered {
		t.Errorf("dropped = %+v, want the subclass gone as not-offered", dropped)
	}

	cat, _ := b.s.Catalog(context.Background(), rules.DefaultLocale)
	state, err := domain.Project(out, cat)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if len(state.Identity.Classes) != 1 || state.Identity.Classes[0].Level != 2 {
		t.Errorf("classes = %+v, want a level-2 rogue", state.Identity.Classes)
	}
	if !state.Identity.Classes[0].Subclass.IsZero() {
		t.Errorf("subclass = %q, want none: it was due at a level the character no longer has",
			state.Identity.Classes[0].Subclass)
	}
	// Everything before the revised entry is untouched and renumbered
	// contiguously, which is what Rebuild is for.
	if out.Len() != 9 {
		t.Errorf("log = %d entries, want 9", out.Len())
	}
}

// Source is written on append and written again on replay, from the same
// function -- so an entry that still means what it meant still says so.
func TestSourceSurvivesAReplace(t *testing.T) {
	b := rogue3(t)
	before := b.log()

	out, _ := revised(t, b, 1, &domain.Event{
		Type:    domain.EventInit,
		Changes: []domain.Change{{Path: "identity.name", Op: domain.OpSet, Value: domain.StringValue("Рурик")}},
	})

	if out.Len() != before.Len() {
		t.Fatalf("log length = %d, want %d", out.Len(), before.Len())
	}
	for i := range out.Events {
		if out.Events[i].Source != before.Events[i].Source {
			t.Errorf("entry %d source = %s, was %s",
				i+1, out.Events[i].Source, before.Events[i].Source)
		}
	}
	// And the values are the ones a client groups tabs by, not "none".
	want := []domain.PromptGroup{
		domain.GroupIdentity, domain.GroupAbilities, domain.GroupRace, domain.GroupRace,
		domain.GroupBackground, domain.GroupClass, domain.GroupClass, domain.GroupClass,
		domain.GroupIdentity, domain.GroupClass,
	}
	for i, group := range want {
		if out.Events[i].Source != group {
			t.Errorf("entry %d source = %s, want %s", i+1, out.Events[i].Source, group)
		}
	}
}
