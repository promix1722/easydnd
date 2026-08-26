package character_test

import (
	"context"
	"slices"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// The stub is the one character this project builds from a hard-coded list of
// selections, which makes these tests the only thing standing between a
// compendium regeneration and a button that silently stops working. They are
// written against the real data in data/srd_5.1 for that reason: a hand-built
// catalogue would only prove the list agrees with itself, and the prompt ids
// the stub answers -- rogue-expertise-1/expertise/0/0 and the rest -- are
// exactly what a regeneration could rename.

func mustCreateStub(t *testing.T, s *charuc.Service) domain.Character {
	t.Helper()
	c, err := s.CreateStub(context.Background(), testOwner, "", rules.DefaultLocale)
	if err != nil {
		t.Fatalf("CreateStub() error = %v", err)
	}
	return c
}

// TestCreateStubBuildsTheReferenceCharacter is the substantive one: every
// selection in stubEvents has to be one the character was actually being
// offered at the point it arrives, or Apply refuses the write.
func TestCreateStubBuildsTheReferenceCharacter(t *testing.T) {
	s := newService(t)
	c := mustCreateStub(t, s)

	if got, want := c.Log.Len(), 13; got != want {
		t.Fatalf("log length = %d, want %d", got, want)
	}
	if got := c.Log.Events[0].Type; got != domain.EventInit {
		t.Errorf("first entry = %v, want init", got)
	}

	sheet, err := s.Sheet(context.Background(), testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Sheet() error = %v", err)
	}

	if got, want := sheet.Identity.Name, charuc.StubName; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
	if got, want := sheet.Identity.Level(), 3; got != want {
		t.Errorf("level = %d, want %d", got, want)
	}
	if got, want := len(sheet.Identity.Classes), 1; got != want {
		t.Fatalf("classes = %d, want %d", got, want)
	}
	if got, want := sheet.Identity.Classes[0].Class, rules.Slug("rogue"); got != want {
		t.Errorf("class = %q, want %q", got, want)
	}
	if got, want := sheet.Identity.Classes[0].Subclass, rules.Slug("thief"); got != want {
		t.Errorf("subclass = %q, want %q", got, want)
	}
	if got, want := sheet.Identity.Race, rules.Slug("half-elf"); got != want {
		t.Errorf("race = %q, want %q", got, want)
	}
}

// TestStubScoresCountTheRaceExactlyOnce is the check the transcription got
// wrong before. The entry records the base array; Project adds the half-elf's
// fixed +2 Charisma and the two chosen +1s. Storing the sheet's final numbers
// instead would read correctly here and double every racial bonus the moment
// the race were changed.
func TestStubScoresCountTheRaceExactlyOnce(t *testing.T) {
	s := newService(t)
	c := mustCreateStub(t, s)

	sheet, err := s.Sheet(context.Background(), testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Sheet() error = %v", err)
	}

	// 10/15/13/10/12/12 as generated, plus dex+1, con+1 and the fixed cha+2.
	want := map[rules.Ability]int{
		rules.Strength:     10,
		rules.Dexterity:    16,
		rules.Constitution: 14,
		rules.Intelligence: 10,
		rules.Wisdom:       12,
		rules.Charisma:     14,
	}
	for ability, score := range want {
		if got := sheet.Abilities.Score(ability); got != score {
			t.Errorf("%s = %d, want %d", ability, got, score)
		}
	}
	if got, want := sheet.Abilities.Method, rules.Slug("point-buy"); got != want {
		t.Errorf("method = %q, want %q", got, want)
	}
}

// TestStubHasItsExpertise checks the one nested answer in the log: choosing the
// "two skills" branch is what brings the two-skill prompt into existence, and
// both arrive in the same entry.
func TestStubHasItsExpertise(t *testing.T) {
	s := newService(t)
	c := mustCreateStub(t, s)

	sheet, err := s.Sheet(context.Background(), testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Sheet() error = %v", err)
	}

	// Keyed by the bare skill slug. Every skill in the compendium is on the
	// sheet, an untrained one as a real entry at "none" rather than a missing
	// key, so what this pins is which eight are *trained* and that nothing
	// else is -- athletics being the one checked from the other direction.
	want := map[rules.Slug]rules.Proficiency{
		"stealth":         rules.Expertise,
		"persuasion":      rules.Expertise,
		"deception":       rules.Proficient,
		"sleight-of-hand": rules.Proficient,
		// From the trait Skill Versatility, not from the race: half-elf poses
		// no proficiency prompt of its own.
		"perception": rules.Proficient,
		"acrobatics": rules.Proficient,
		// From the background. The real character's Urchin would have granted a
		// disguise kit instead, but SRD 5.1 publishes only acolyte.
		"insight":  rules.Proficient,
		"religion": rules.Proficient,
	}
	for skill, level := range want {
		if got := sheet.Skills.BySkill[skill].Proficiency; got != level {
			t.Errorf("%s = %v, want %v", skill, got, level)
		}
	}
	if got := sheet.Skills.BySkill["athletics"].Proficiency; got != rules.NotProficient {
		t.Errorf("athletics = %v, want none: nothing in the log grants it", got)
	}
	trained := 0
	for _, state := range sheet.Skills.BySkill {
		if state.Proficiency != rules.NotProficient {
			trained++
		}
	}
	if trained != len(want) {
		t.Errorf("trained skills = %d, want %d", trained, len(want))
	}
}

// TestStubDerivedNumbers pins the headline block against the exported sheet.
// Armor class is the reason the log equips the leather armor by hand: a rogue's
// kit contains it, but no rule says it is worn.
func TestStubDerivedNumbers(t *testing.T) {
	s := newService(t)
	c := mustCreateStub(t, s)

	sheet, err := s.Sheet(context.Background(), testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Sheet() error = %v", err)
	}

	if got, want := sheet.Status.ArmorClass, 14; got != want {
		t.Errorf("armor class = %d, want %d", got, want)
	}
	if got, want := sheet.Status.ProficiencyBonus, 2; got != want {
		t.Errorf("proficiency bonus = %d, want %d", got, want)
	}
	if got, want := sheet.Base.HitPoints.Max, 24; got != want {
		t.Errorf("hit point maximum = %d, want %d", got, want)
	}
}

// TestStubLeavesOnlyTheLevelOffer states what a stub is *for*: nothing is left
// to fill in.
//
// "Complete" is the weaker property and the stub had it long before this test
// did: seven of its prompts are optional -- the half-elf's third language and
// all six acolyte poses -- so the character counted as complete while the build
// screen still showed seven rows under "still to choose". Answering them is the
// difference between a character the rules call finished and one a person would.
//
// `character/level` is the exception and stays open on purpose. It is the
// standing offer every complete character carries, and answering it would not
// fill anything in -- it would make the stub a level 4 rogue.
func TestStubLeavesOnlyTheLevelOffer(t *testing.T) {
	s := newService(t)
	c := mustCreateStub(t, s)

	prompts, err := s.Prompts(context.Background(), testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Prompts() error = %v", err)
	}
	if !domain.Complete(prompts) {
		t.Error("stub is not complete")
	}

	var open []rules.Slug
	for _, p := range prompts {
		open = append(open, p.Choice.Prompt)
	}
	if len(open) != 1 || open[0] != "character/level" {
		t.Errorf("open prompts = %v, want only character/level", open)
	}
}

// The answers to those optional prompts have to actually land on the sheet, not
// merely be accepted: an answer the projector ignores would leave the build
// screen showing the row as settled and the sheet showing nothing.
func TestStubOptionalAnswersReachTheSheet(t *testing.T) {
	s := newService(t)
	c := mustCreateStub(t, s)

	sheet, err := s.Sheet(context.Background(), testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Sheet() error = %v", err)
	}

	// Common and Elvish from the race, then the three that were chosen.
	wantLanguages := []rules.Slug{"celestial", "common", "draconic", "elvish", "undercommon"}
	got := slices.Clone(sheet.Base.Languages)
	slices.Sort(got)
	if !slices.Equal(got, wantLanguages) {
		t.Errorf("languages = %v, want %v", got, wantLanguages)
	}

	if len(sheet.Identity.PersonalityTraits) != 2 {
		t.Errorf("personality traits = %v, want two", sheet.Identity.PersonalityTraits)
	}
	for _, field := range []struct {
		name  string
		value []string
	}{
		{"ideals", sheet.Identity.Ideals},
		{"bonds", sheet.Identity.Bonds},
		{"flaws", sheet.Identity.Flaws},
	} {
		if len(field.value) != 1 {
			t.Errorf("%s = %v, want one", field.name, field.value)
		}
	}

	// Acolyte's fixed kit does arrive.
	backpack := make(map[rules.Slug]bool, len(sheet.Equipment.Backpack))
	for _, stack := range sheet.Equipment.Backpack {
		backpack[stack.Item] = true
	}
	for _, item := range []rules.Slug{"clothes-common", "pouch"} {
		if !backpack[item] {
			t.Errorf("%s is not in the backpack", item)
		}
	}

	// The amulet chosen for acolyte/starting-equipment/0 is *not*, and this
	// asserts the gap rather than the feature so that closing it fails here
	// and gets noticed.
	//
	// That prompt draws its options from an equipment *category*
	// (rules.OptionsFromEquipmentCategory, "any item in holy-symbols"), and
	// nothing outside the catalogue DTO reads that kind: rules.OptionKeys
	// returns no keys for it, so validateAnswer's membership check is skipped
	// and any slug at all is accepted -- "not-a-real-item" and "rapier"
	// included -- and the projector materialises none of them. The answer is
	// still recorded and still closes the prompt, which is why the stub has
	// nothing outstanding; it just buys no equipment.
	//
	// Pre-existing, and not about the stub: every category-drawn equipment
	// prompt in the compendium behaves this way. Delete this and assert the
	// amulet when that is fixed.
	for _, stack := range append(slices.Clone(sheet.Equipment.Backpack), sheet.Equipment.Equipped...) {
		if stack.Item == "amulet" {
			t.Error("the amulet now reaches the sheet -- equipment-category choices " +
				"have been implemented, so assert it properly and drop this")
		}
	}
}

// TestStubEntriesAreAttributed is the check that keeps the stub a log the build
// screen could have written. Every entry is stamped by the server with the
// group of the prompt it answers, and the build screen files entries into tabs
// by exactly that -- so an unattributed entry is one nobody can find.
//
// The leather-armor adjustment is the deliberate exception. It answers no
// prompt and closes none, which is what a DM's ruling looks like, so it carries
// no source and belongs to no tab.
func TestStubEntriesAreAttributed(t *testing.T) {
	s := newService(t)
	c := mustCreateStub(t, s)

	for _, event := range c.Log.Events {
		equipping := event.Type == domain.EventChange &&
			len(event.Changes) == 1 &&
			event.Changes[0].Path == "equipment.equipped"

		switch {
		case equipping && event.Source != domain.PromptGroupNone:
			t.Errorf("entry %d equips armor and answers nothing, but is attributed to %v",
				event.Seq, event.Source)
		case !equipping && event.Source == domain.PromptGroupNone:
			t.Errorf("entry %d (%v) carries no source", event.Seq, event.Type)
		}
	}

	// The four roleplaying lines in particular. They are changes carrying no
	// answer at all, so the only thing that can attribute them is the prompt
	// they close -- and getting that wrong is silent: the entry saves, and the
	// build screen simply never shows it anywhere.
	written := map[domain.Path]bool{
		"identity.personalityTraits": false,
		"identity.ideals":            false,
		"identity.bonds":             false,
		"identity.flaws":             false,
	}
	for _, event := range c.Log.Events {
		for _, change := range event.Changes {
			if _, roleplaying := written[change.Path]; !roleplaying {
				continue
			}
			written[change.Path] = true
			if event.Source != domain.GroupPersonality {
				t.Errorf("entry %d writes %s but is attributed to %v, want personality",
					event.Seq, change.Path, event.Source)
			}
		}
	}
	for path, found := range written {
		if !found {
			t.Errorf("the stub writes no %s", path)
		}
	}
}

// TestCreateStubFilesIntoAFolder: the stub is reached from the party list,
// which may be filtered, and a character that ignored the filter would land
// somewhere the player is not looking.
func TestCreateStubFilesIntoAFolder(t *testing.T) {
	s := newService(t)
	folder, err := s.CreateFolder(context.Background(), testOwner, "Stubs")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	c, err := s.CreateStub(context.Background(), testOwner, folder.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("CreateStub() error = %v", err)
	}
	if got, want := c.Folder, folder.ID; got != want {
		t.Errorf("folder = %q, want %q", got, want)
	}
}
