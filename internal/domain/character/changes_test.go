package character

import (
	"testing"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

// skills.* and savingThrows.* exist so that a sheet imported from another tool
// can state its proficiencies as facts -- there is no answer to a prompt to
// record, because a foreign sheet does not say which prompt granted what.
//
// Both are override tier, which is the trap these tests exist for. Overrides
// land after deriveStatus has already computed every bonus, so a handler that
// sets only the training level leaves the sheet reading "Expertise" beside the
// number for plain proficiency. That failure is invisible unless something
// checks the bonus, so these do.

func skillLog(t *testing.T, changes ...Change) State {
	t.Helper()
	var log Log
	err := log.Append(
		Event{Type: EventInit, Changes: append([]Change{
			{Path: "identity.name", Op: OpSet, Value: StringValue("Test")},
			{Path: "abilities.dex", Op: OpSet, Value: IntValue(16)},
			{Path: "abilities.wis", Op: OpSet, Value: IntValue(12)},
		}, changes...)},
		Event{Type: EventClass, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 1},
		Event{Type: EventLevel, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 2},
		Event{Type: EventLevel, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 3},
	)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	state, err := Project(log, LoadCatalog(t))
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	return state
}

func TestChangeSkillSetsLevelAndRecomputesBonus(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		wantLevel rules.Proficiency
		wantBonus int // Dexterity 16 is +3; the proficiency bonus at level 3 is 2.
	}{
		{"proficient", "proficient", rules.Proficient, 5},
		{"expertise", "expertise", rules.Expertise, 7},
		{"half", "half", rules.HalfProficient, 4},
		{"none", "none", rules.NotProficient, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := skillLog(t, Change{
				Path: "skills.stealth", Op: OpSet, Value: StringValue(tt.level),
			})
			got := s.Skills.BySkill["stealth"]
			if got.Proficiency != tt.wantLevel {
				t.Errorf("proficiency = %s, want %s", got.Proficiency, tt.wantLevel)
			}
			if got.Bonus != tt.wantBonus {
				t.Errorf("bonus = %d, want %d", got.Bonus, tt.wantBonus)
			}
		})
	}
}

// Passive Perception reads the Perception bonus, so a change to that skill has
// to carry it along or the two disagree on the same sheet.
func TestChangeSkillRecomputesPassivePerception(t *testing.T) {
	s := skillLog(t, Change{
		Path: "skills.perception", Op: OpSet, Value: StringValue("expertise"),
	})
	// Wisdom 12 is +1, Expertise at proficiency bonus 2 adds 4.
	if got := s.Skills.BySkill["perception"].Bonus; got != 5 {
		t.Fatalf("perception bonus = %d, want 5", got)
	}
	if s.Status.PassivePerception != 15 {
		t.Errorf("passive Perception = %d, want 15", s.Status.PassivePerception)
	}
}

// Experience is recorded, not acted on: a level comes from a level event, so
// crossing a threshold advances nobody. The test is here rather than in
// project_test.go because a change event is the only thing that sets it.
func TestChangeExperienceRecordsWithoutAdvancing(t *testing.T) {
	s := skillLog(t, Change{
		Path: "identity.experience", Op: OpSet, Value: IntValue(900),
	})

	if s.Identity.Experience != 900 {
		t.Errorf("experience = %d, want 900", s.Identity.Experience)
	}
	// 900 XP is third level in the 2014 rules, and this character is third
	// level because the log says so three times, not because of the number.
	if got := s.Identity.Level(); got != 3 {
		t.Errorf("level = %d, want 3: the log decides the level, not the XP", got)
	}

	// It increments, which is how a session's award is recorded.
	s = skillLog(t,
		Change{Path: "identity.experience", Op: OpSet, Value: IntValue(900)},
		Change{Path: "identity.experience", Op: OpIncrement, Value: IntValue(350)},
	)
	if s.Identity.Experience != 1250 {
		t.Errorf("experience = %d, want 1250", s.Identity.Experience)
	}
}

func TestChangeSavingThrow(t *testing.T) {
	// The rogue already has a Dexterity save, so setting Charisma is the case
	// that proves a change can add one the class does not grant.
	s := skillLog(t, Change{
		Path: "savingThrows.cha", Op: OpSet, Value: BoolValue(true),
	})
	got := s.SavingThrows.ByAbility[rules.Charisma]
	if !got.Proficient {
		t.Error("charisma save should be proficient")
	}
	// Charisma is unset, so 10, so +0; the proficiency bonus at level 3 is 2.
	if got.Bonus != 2 {
		t.Errorf("charisma save = %d, want 2", got.Bonus)
	}
}

// Taking a proficiency away has to work too, or a change event cannot undo a
// DM's earlier ruling.
func TestChangeSavingThrowCanRemove(t *testing.T) {
	s := skillLog(t, Change{
		Path: "savingThrows.dex", Op: OpSet, Value: BoolValue(false),
	})
	got := s.SavingThrows.ByAbility[rules.Dexterity]
	if got.Proficient {
		t.Error("dexterity save should have been taken away")
	}
	if got.Bonus != 3 {
		t.Errorf("dexterity save = %d, want 3 (the bare modifier)", got.Bonus)
	}
}

func TestChangeRejectsBadSkillPaths(t *testing.T) {
	tests := []struct {
		name   string
		change Change
	}{
		{"unknown skill", Change{Path: "skills.jousting", Op: OpSet, Value: StringValue("proficient")}},
		{"unknown level", Change{Path: "skills.stealth", Op: OpSet, Value: StringValue("very")}},
		{"wrong value kind", Change{Path: "skills.stealth", Op: OpSet, Value: BoolValue(true)}},
		{"wrong operator", Change{Path: "skills.stealth", Op: OpAdd, Value: StringValue("proficient")}},
		{"no skill named", Change{Path: "skills", Op: OpSet, Value: StringValue("proficient")}},
		{"unknown ability", Change{Path: "savingThrows.luck", Op: OpSet, Value: BoolValue(true)}},
		{"save wants a bool", Change{Path: "savingThrows.dex", Op: OpSet, Value: IntValue(1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var log Log
			if err := log.Append(Event{Type: EventInit, Changes: []Change{tt.change}}); err != nil {
				t.Fatalf("Append() error = %v", err)
			}
			if _, err := Project(log, LoadCatalog(t)); err == nil {
				t.Error("Project() should have refused this change")
			}
		})
	}
}
