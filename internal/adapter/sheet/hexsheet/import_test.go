package hexsheet_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/sheet/hexsheet"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// This is the importer's golden test. It reads the same export that
// internal/domain/character/project_test.go checks a hand-written log against,
// imports it, projects the result, and asserts the numbers come back.
//
// The fixture is read from docs/ rather than a testdata directory, which is a
// departure from the rest of the suite. It is deliberate: the export is
// vendored reference material described in docs/licensing.md, and a copy under
// testdata/ would be a second copy to keep in step with it. The file is only
// ever read.
//
// Two of the export's fields are hand-entered and contradict the rules, and
// the assertions below say so where they land:
//
//   - initiative reads 1 where Dexterity 16 gives +3. The import pins it, so
//     the sheet reproduces the export rather than correcting it.
//   - the background is Urchin, which SRD 5.1 does not publish, so it is
//     reported rather than substituted.

// One Source for this test package: Source.Load caches per locale, so the
// compendium is read once rather than once per test. Safe because a Catalog is
// immutable and Load is mutex-guarded (see catalog/file/source.go).
var catalogSource = catalogfile.NewSource(filepath.Join("..", "..", "..", "..", "data", "srd_5.1"))

func loadCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalogSource.Load(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatalf("loading the compendium: %v", err)
	}
	return c
}

func importReference(t *testing.T) (character.State, hexsheet.Report) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..",
		"docs", "reference_hexsheet", "rouge_3_level.json")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening the reference export: %v", err)
	}
	defer f.Close()

	cat := loadCatalog(t)
	at := time.Date(2026, time.August, 23, 14, 27, 51, 0, time.UTC)

	log, report, err := hexsheet.Import(f, cat, at)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	state, err := character.Project(log, cat)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	return state, report
}

func TestImportIdentity(t *testing.T) {
	s, _ := importReference(t)

	if s.Identity.Name != "Сахарок" {
		t.Errorf("name = %q, want Сахарок", s.Identity.Name)
	}
	if s.Identity.Alignment != "neutral" {
		t.Errorf("alignment = %q, want neutral", s.Identity.Alignment)
	}
	if s.Identity.Race != "half-elf" {
		t.Errorf("race = %q, want half-elf", s.Identity.Race)
	}
	if got := s.Identity.Level(); got != 3 {
		t.Errorf("level = %d, want 3", got)
	}
	if len(s.Identity.Classes) != 1 {
		t.Fatalf("classes = %d, want 1", len(s.Identity.Classes))
	}
	if c := s.Identity.Classes[0]; c.Class != "rogue" || c.Level != 3 || c.Subclass != "thief" {
		t.Errorf("class = %+v, want rogue 3 (thief)", c)
	}
	// Urchin is not in SRD 5.1, so no background is set and the report says so.
	if s.Identity.Background != "" {
		t.Errorf("background = %q, want none", s.Identity.Background)
	}
}

// TestImportAbilities is the one that catches the double count. The export
// records final scores; the projection re-applies the half-elf's fixed +2
// Charisma, so the import has to record 12 for Charisma to read 14.
func TestImportAbilities(t *testing.T) {
	s, _ := importReference(t)

	want := map[rules.Ability]int{
		rules.Strength: 10, rules.Dexterity: 16, rules.Constitution: 14,
		rules.Intelligence: 10, rules.Wisdom: 12, rules.Charisma: 14,
	}
	for ability, score := range want {
		if got := s.Abilities.Score(ability); got != score {
			t.Errorf("%s = %d, want %d", ability, got, score)
		}
	}
}

func TestImportVitals(t *testing.T) {
	s, _ := importReference(t)

	if s.Base.HitPoints.Max != 24 {
		t.Errorf("hit point maximum = %d, want 24", s.Base.HitPoints.Max)
	}
	if s.Base.HitPoints.Current != 24 {
		t.Errorf("current hit points = %d, want 24", s.Base.HitPoints.Current)
	}
	if s.Status.ArmorClass != 14 {
		t.Errorf("armor class = %d, want 14", s.Status.ArmorClass)
	}
	if s.Status.ProficiencyBonus != 2 {
		t.Errorf("proficiency bonus = %d, want 2", s.Status.ProficiencyBonus)
	}
	// Pinned from the export, not derived: the export's own initiative is 1
	// where Dexterity 16 would give +3. Importing means reproducing the sheet.
	if s.Status.Initiative != 1 {
		t.Errorf("initiative = %d, want the export's 1", s.Status.Initiative)
	}
}

func TestImportSkillsAndSavingThrows(t *testing.T) {
	s, _ := importReference(t)

	want := map[rules.Slug]rules.Proficiency{
		"deception": rules.Proficient, "acrobatics": rules.Proficient,
		"perception": rules.Proficient, "sleight-of-hand": rules.Proficient,
		"persuasion": rules.Expertise, "stealth": rules.Expertise,
	}
	for skill, level := range want {
		got, ok := s.Skills.BySkill[skill]
		if !ok {
			t.Errorf("%s is missing", skill)
			continue
		}
		if got.Proficiency != level {
			t.Errorf("%s = %s, want %s", skill, got.Proficiency, level)
		}
	}

	// The bonus has to be recomputed by the change, not left at whatever
	// deriveStatus produced before the override landed.
	if got := s.Skills.BySkill["stealth"].Bonus; got != 7 {
		t.Errorf("stealth bonus = %d, want 7 (DEX +3, Expertise +4)", got)
	}
	if got := s.Skills.BySkill["perception"].Bonus; got != 3 {
		t.Errorf("perception bonus = %d, want 3 (WIS +1, proficient +2)", got)
	}
	if s.Status.PassivePerception != 13 {
		t.Errorf("passive Perception = %d, want 13", s.Status.PassivePerception)
	}

	for _, ability := range []rules.Ability{rules.Dexterity, rules.Intelligence} {
		if !s.SavingThrows.ByAbility[ability].Proficient {
			t.Errorf("%s save should be proficient", ability)
		}
	}
	for _, ability := range []rules.Ability{
		rules.Strength, rules.Constitution, rules.Wisdom, rules.Charisma,
	} {
		if s.SavingThrows.ByAbility[ability].Proficient {
			t.Errorf("%s save should not be proficient", ability)
		}
	}
	if got := s.SavingThrows.ByAbility[rules.Dexterity].Bonus; got != 5 {
		t.Errorf("dex save = %d, want 5", got)
	}
}

func TestImportEquipmentAndProficiencies(t *testing.T) {
	s, _ := importReference(t)

	if len(s.Equipment.Equipped) != 4 {
		t.Errorf("equipped = %d items, want 4", len(s.Equipment.Equipped))
	}
	if !hasItem(s.Equipment.Equipped, "leather-armor") {
		t.Error("leather armor should be worn -- armor class depends on it")
	}
	// The three names that only resolve through comma inversion and the
	// singular rule.
	for _, slug := range []rules.Slug{"lantern-hooded", "rope-hempen-50-feet", "arrow"} {
		if !hasItem(s.Equipment.Backpack, slug) {
			t.Errorf("backpack is missing %s", slug)
		}
	}
	for _, slug := range []rules.Slug{"thieves-tools", "disguise-kit"} {
		if !contains(s.Proficiencies, slug) {
			t.Errorf("proficiencies missing %s", slug)
		}
	}
	if len(s.Base.Languages) != 2 {
		t.Errorf("languages = %v, want common and elvish", s.Base.Languages)
	}
}

func TestImportTraitsAndFeatures(t *testing.T) {
	s, _ := importReference(t)

	for _, slug := range []rules.Slug{"darkvision", "fey-ancestry", "skill-versatility"} {
		if !contains(s.Traits, slug) {
			t.Errorf("traits missing %s", slug)
		}
	}
	for _, slug := range []rules.Slug{"sneak-attack", "cunning-action", "fast-hands"} {
		if !contains(s.Features, slug) {
			t.Errorf("features missing %s", slug)
		}
	}
}

// TestImportReport pins the promise that nothing vanishes silently.
func TestImportReport(t *testing.T) {
	_, report := importReference(t)

	if !reported(report.Unresolved, "Urchin") {
		t.Errorf("the report should name Urchin: %+v", report.Unresolved)
	}
	if !reported(report.Unresolved, "One language of your choice") {
		t.Errorf("the report should name the unpicked language: %+v", report.Unresolved)
	}
	if !reported(report.Skipped, "gp") {
		t.Errorf("the report should name the purse: %+v", report.Skipped)
	}
	if !reported(report.Skipped, "Sneak Attack") {
		t.Errorf("the report should name the class resource: %+v", report.Skipped)
	}
	// Two daggers arrive as one, and that is said out loud.
	if !reported(report.Skipped, "Dagger") {
		t.Errorf("the report should name the lost dagger count: %+v", report.Skipped)
	}

	// Nothing was answered, so the race's ability bonuses are still open.
	if !slices(report.Open, "half-elf/ability-bonus/0") {
		t.Errorf("half-elf/ability-bonus/0 should be open: %v", report.Open)
	}
	if !slices(report.Open, "rogue/proficiency/0") {
		t.Errorf("rogue/proficiency/0 should be open: %v", report.Open)
	}
}

func TestImportRejectsAnotherTool(t *testing.T) {
	body := `{"exportedFrom":"Roll20","character":{"name":"x","gameSystem":"dnd5e"}}`
	if _, _, err := hexsheet.Import(strings.NewReader(body), loadCatalog(t), time.Now()); err == nil {
		t.Error("importing another tool's export should fail")
	}
}

func TestImportRejectsMalformedJSON(t *testing.T) {
	if _, _, err := hexsheet.Import(strings.NewReader("{nope"), loadCatalog(t), time.Now()); err == nil {
		t.Error("importing malformed JSON should fail")
	}
}

func hasItem(stacks []character.ItemStack, slug rules.Slug) bool {
	for _, s := range stacks {
		if s.Item == slug {
			return true
		}
	}
	return false
}

func contains(list []rules.Slug, slug rules.Slug) bool {
	for _, s := range list {
		if s == slug {
			return true
		}
	}
	return false
}

func slices(list []rules.Slug, want string) bool { return contains(list, rules.Slug(want)) }

func reported(entries []hexsheet.Entry, substring string) bool {
	for _, e := range entries {
		if strings.Contains(e.Detail, substring) {
			return true
		}
	}
	return false
}
