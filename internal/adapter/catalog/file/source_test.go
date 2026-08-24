package file_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// The assignment is the conformance proof: if the loader ever drifts from the
// port, this fails to compile rather than failing at wiring time.
var _ catalog.Source = (*file.Source)(nil)

// dataDir is the committed compendium, four levels up from this package.
func dataDir() string { return filepath.Join("..", "..", "..", "..", "data", "srd_5.1") }

func load(t *testing.T, locale rules.Locale) *catalog.Catalog {
	t.Helper()
	c, err := file.NewSource(dataDir()).Load(context.Background(), locale)
	if err != nil {
		t.Fatalf("Load(%q) error = %v", locale, err)
	}
	return c
}

// The counts come from the upstream dump. They are pinned here because a
// generator that silently drops records is otherwise indistinguishable from
// one that works: nothing errors, a spell simply does not exist.
func TestLoadEntryCounts(t *testing.T) {
	c := load(t, rules.LocaleEN)

	tests := []struct {
		name string
		got  int
		want int
	}{
		{"abilities", c.Abilities.Len(), 6},
		{"skills", c.Skills.Len(), 18},
		{"alignments", c.Alignments.Len(), 9},
		{"languages", c.Languages.Len(), 16},
		{"conditions", c.Conditions.Len(), 15},
		{"damage types", c.DamageTypes.Len(), 13},
		{"magic schools", c.MagicSchools.Len(), 8},
		{"weapon properties", c.WeaponProperties.Len(), 11},
		{"proficiencies", c.Proficiencies.Len(), 117},
		{"equipment categories", c.EquipmentCategories.Len(), 39},
		{"races", c.Races.Len(), 9},
		{"subraces", c.Subraces.Len(), 4},
		{"traits", c.Traits.Len(), 38},
		{"classes", c.Classes.Len(), 12},
		{"subclasses", c.Subclasses.Len(), 12},
		{"features", c.Features.Len(), 407},
		{"backgrounds", c.Backgrounds.Len(), 1},
		{"feats", c.Feats.Len(), 1},
		{"items", c.Items.Len(), 237},
		{"magic items", c.MagicItems.Len(), 362},
		{"spells", c.Spells.Len(), 319},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
			}
		})
	}
}

// Rule strings are normalised by the generator rather than stored as prose.
// This pins one spell of each shape, so a parser regression is caught as a
// wrong structure and not as a range that quietly becomes zero feet.
func TestSpellRuleStringsAreStructured(t *testing.T) {
	c := load(t, rules.LocaleEN)

	spell, ok := c.Spells.Get("acid-arrow")
	if !ok {
		t.Fatal("acid-arrow not found")
	}
	if spell.CastingTime.Kind != catalog.CastAsAction {
		t.Errorf("casting time kind = %v, want CastAsAction", spell.CastingTime.Kind)
	}
	if spell.Range.Kind != catalog.RangeDistance || spell.Range.Distance != 90 {
		t.Errorf("range = %v/%d, want RangeDistance/90", spell.Range.Kind, spell.Range.Distance)
	}
	if spell.Duration.Kind != catalog.DurationInstantaneous {
		t.Errorf("duration kind = %v, want DurationInstantaneous", spell.Duration.Kind)
	}
	if spell.Damage == nil {
		t.Fatal("acid-arrow has no damage")
	}
	if got := spell.Damage.Scaling.AtSlotLevel[2]; got.String() != "4d4" {
		t.Errorf("damage at slot 2 = %q, want %q", got, "4d4")
	}

	// "Up to 1 minute" carries information a flat duration does not: the
	// caster may end it early.
	if bless, ok := c.Spells.Get("bless"); ok {
		if !bless.Duration.UpTo {
			t.Error("bless duration UpTo = false, want true")
		}
		if bless.Duration.Amount != 1 || bless.Duration.Unit != catalog.Minute {
			t.Errorf("bless duration = %d %v, want 1 minute", bless.Duration.Amount, bless.Duration.Unit)
		}
	}

	// A cantrip scales by character level, not slot level. Conflating the two
	// is how a cantrip ends up dealing no damage.
	fireBolt, ok := c.Spells.Get("fire-bolt")
	if !ok {
		t.Fatal("fire-bolt not found")
	}
	if !fireBolt.IsCantrip() {
		t.Error("fire-bolt IsCantrip() = false, want true")
	}
	if fireBolt.Damage == nil || len(fireBolt.Damage.Scaling.AtCharacterLevel) == 0 {
		t.Error("fire-bolt has no character-level scaling")
	}
	if fireBolt.Damage != nil && len(fireBolt.Damage.Scaling.AtSlotLevel) != 0 {
		t.Error("fire-bolt has slot-level scaling, want none")
	}
}

// Darkvision is a racial trait, not a class feature. The SRD keeps the two in
// separate collections and so must the catalogue, or a character cannot say
// which source granted it.
func TestTraitsAndFeaturesStaySeparate(t *testing.T) {
	c := load(t, rules.LocaleEN)

	if _, ok := c.Traits.Get("darkvision"); !ok {
		t.Error("darkvision is not a trait")
	}
	if _, ok := c.Features.Get("darkvision"); ok {
		t.Error("darkvision is a feature, want trait only")
	}
	if _, ok := c.Features.Get("cunning-action"); !ok {
		t.Error("cunning-action is not a feature")
	}
	if _, ok := c.Traits.Get("cunning-action"); ok {
		t.Error("cunning-action is a trait, want feature only")
	}
}

// The recursive option grammar is the hardest shape in the data. These are the
// two deepest real cases.
func TestChoiceTreeDecodes(t *testing.T) {
	c := load(t, rules.LocaleEN)

	fighter, ok := c.Classes.Get("fighter")
	if !ok {
		t.Fatal("fighter not found")
	}
	if len(fighter.StartingEquipmentOptions) == 0 {
		t.Fatal("fighter has no starting equipment options")
	}
	var nested int
	for _, choice := range fighter.StartingEquipmentOptions {
		if choice.Prompt.IsZero() {
			t.Error("choice has no prompt; a stored answer could not refer to it")
		}
		for _, opt := range choice.From.Options {
			switch opt.(type) {
			case rules.NestedOption, rules.BundleOption:
				nested++
			}
		}
	}
	if nested == 0 {
		t.Error("fighter starting equipment decoded no nested or bundled options")
	}

	halfElf, ok := c.Races.Get("half-elf")
	if !ok {
		t.Fatal("half-elf not found")
	}
	if halfElf.AbilityBonusOptions == nil {
		t.Fatal("half-elf has no ability bonus options")
	}
	if halfElf.AbilityBonusOptions.Choose != 2 {
		t.Errorf("half-elf chooses %d abilities, want 2", halfElf.AbilityBonusOptions.Choose)
	}
	for _, opt := range halfElf.AbilityBonusOptions.From.Options {
		if _, ok := opt.(rules.AbilityBonusOption); !ok {
			t.Errorf("half-elf ability option is %T, want rules.AbilityBonusOption", opt)
		}
	}
}

// Class resources are the thing DND.md called "slots". Sneak attack is a die,
// ki is a count, and both have to survive the same generic shape.
func TestClassLevelResources(t *testing.T) {
	c := load(t, rules.LocaleEN)

	rogue3, ok := c.ClassLevel("rogue", 3)
	if !ok {
		t.Fatal("rogue level 3 not found")
	}
	if rogue3.ProficiencyBonus != 2 {
		t.Errorf("rogue 3 proficiency bonus = %d, want 2", rogue3.ProficiencyBonus)
	}
	var sneak *rules.Dice
	for _, r := range rogue3.Resources {
		if r.Key == "sneak-attack" {
			sneak = r.Dice
		}
	}
	if sneak == nil {
		t.Fatal("rogue 3 has no sneak-attack dice")
	}
	if sneak.String() != "2d6" {
		t.Errorf("rogue 3 sneak attack = %q, want %q", sneak, "2d6")
	}

	wizard5, ok := c.ClassLevel("wizard", 5)
	if !ok {
		t.Fatal("wizard level 5 not found")
	}
	if wizard5.SpellSlots[3] != 2 {
		t.Errorf("wizard 5 third-level slots = %d, want 2", wizard5.SpellSlots[3])
	}
}

// A partially translated locale must fall back key by key, not entry by entry:
// a translated name with an untranslated description is the normal state of a
// growing locale.
func TestLocaleFallsBackPerKey(t *testing.T) {
	ru := load(t, rules.LocaleRU)

	if ru.Locale() != rules.LocaleRU {
		t.Errorf("Locale() = %q, want %q", ru.Locale(), rules.LocaleRU)
	}

	dwarf, ok := ru.Races.Get("dwarf")
	if !ok {
		t.Fatal("dwarf not found in ru")
	}
	if dwarf.Name != "Дварф" {
		t.Errorf("dwarf name = %q, want the Russian translation", dwarf.Name)
	}
	// The age paragraph has no Russian translation, so it must still be
	// present in English rather than empty.
	if len(dwarf.AgeDesc) == 0 {
		t.Error("dwarf age text is empty; an untranslated key must fall back to English")
	}

	// Spells have no Russian bundle at all, so the whole collection falls
	// back. That must not leave names blank.
	spell, ok := ru.Spells.Get("acid-arrow")
	if !ok {
		t.Fatal("acid-arrow not found in ru")
	}
	if spell.Name != "Acid Arrow" {
		t.Errorf("acid-arrow name = %q, want the English fallback", spell.Name)
	}

	// Counts must match the English catalogue: falling back must never drop
	// an entry.
	en := load(t, rules.LocaleEN)
	if ru.Spells.Len() != en.Spells.Len() {
		t.Errorf("ru spells = %d, en spells = %d; fallback must not drop entries", ru.Spells.Len(), en.Spells.Len())
	}
}

func TestLocalesListsWhatIsPresent(t *testing.T) {
	got, err := file.NewSource(dataDir()).Locales(context.Background())
	if err != nil {
		t.Fatalf("Locales() error = %v", err)
	}
	if len(got) != 2 || got[0] != rules.LocaleEN || got[1] != rules.LocaleRU {
		t.Errorf("Locales() = %v, want [en ru]", got)
	}
}

func TestLoadRejectsUnsupportedLocale(t *testing.T) {
	_, err := file.NewSource(dataDir()).Load(context.Background(), "xx")
	if err == nil {
		t.Fatal("Load() with an unsupported locale succeeded, want an error")
	}
}

// The manifest's counts must agree with what actually loads, which is what
// makes a truncated write detectable.
func TestManifestMatchesLoadedCounts(t *testing.T) {
	m, err := file.ReadManifest(dataDir())
	if err != nil {
		t.Fatalf("ReadManifest() error = %v", err)
	}
	if m.Ruleset != "2014" {
		t.Errorf("ruleset = %q, want %q", m.Ruleset, "2014")
	}
	c := load(t, rules.LocaleEN)
	if got := m.Counts[file.FileSpells]; got != c.Spells.Len() {
		t.Errorf("manifest spells = %d, loaded = %d", got, c.Spells.Len())
	}
	for _, name := range file.MechanicsFiles() {
		if _, ok := m.Counts[name]; !ok {
			t.Errorf("manifest has no count for %s", name)
		}
	}
}

// Terms are the prose behind rules.TextOption, DamageOption.Notes and
// ActionOption. They have no mechanics file, so nothing about the mechanics
// side of the loader would notice if they stopped being read -- the failure
// would surface as a background asking a player to choose between
// "i-idolize-a-particular-hero-of" and "i-can-find-common-ground-between".
func TestTermsResolveTextOptions(t *testing.T) {
	c := load(t, rules.LocaleEN)

	if got := c.Terms.Len(); got != 47 {
		t.Errorf("Terms.Len() = %d, want 47", got)
	}

	// The key comes from acolyte/ideal/0, which is a TextOption carrying
	// nothing but this slug.
	term, ok := c.Terms.Get("tradition-the-ancient-traditions-of-worship")
	if !ok {
		t.Fatal("Terms.Get(tradition-...) not found")
	}
	if term.Name == "" || term.Name == term.Slug.String() {
		t.Errorf("term name = %q, want the resolved prose", term.Name)
	}

	// Every text option the catalogue poses must resolve, or the prompt is
	// unanswerable.
	background, ok := c.Backgrounds.Get("acolyte")
	if !ok {
		t.Fatal("Backgrounds.Get(acolyte) not found")
	}
	for _, choice := range []rules.Choice{
		background.PersonalityTraits, background.Ideals, background.Bonds, background.Flaws,
	} {
		for _, option := range choice.From.Options {
			text, isText := option.(rules.TextOption)
			if !isText {
				continue
			}
			if !c.Terms.Has(text.Key) {
				t.Errorf("prompt %q offers term %q, which does not resolve", choice.Prompt, text.Key)
			}
		}
	}
}

// Equipment packs must resolve their contents.
//
// Upstream writes a pack's contents as {"item": ..., "quantity": n} but
// starting equipment as {"equipment": ..., "quantity": n}, and the generator
// read only the second key -- so all 66 pack contents were emitted with an
// empty slug. Nothing failed: srdgen's zero-warning gate does not see an empty
// string, and no test looked. A rogue's Burglar's Pack simply contained
// fourteen of nothing.
func TestEquipmentPackContentsResolve(t *testing.T) {
	c := load(t, rules.LocaleEN)

	pack, ok := c.Items.Get("burglars-pack")
	if !ok {
		t.Fatal("Items.Get(burglars-pack) not found")
	}
	if pack.Gear == nil {
		t.Fatal("burglars-pack has no gear")
	}
	if len(pack.Gear.Contents) != 14 {
		t.Errorf("burglars-pack contents = %d, want 14", len(pack.Gear.Contents))
	}

	// Every pack in the catalogue, not just this one: an empty slug here is
	// indistinguishable from a pack that legitimately holds nothing.
	for _, item := range c.Items.All() {
		if item.Gear == nil {
			continue
		}
		for i, stack := range item.Gear.Contents {
			if stack.Item.IsZero() {
				t.Errorf("%s contents[%d] has no item slug", item.Slug, i)
				continue
			}
			if !c.Items.Has(stack.Item) {
				t.Errorf("%s contents[%d] names %q, which is not in the catalogue",
					item.Slug, i, stack.Item)
			}
		}
	}
}

// The domain's enum names and this package's wire vocabulary are the same
// vocabulary written twice: the domain owns String, and kinds.go owns the
// tables the generator writes and the loader reads.
//
// Two spellings of one enum is how "martial" becomes "Martial" in one of
// them, and the failure is silent -- a weapon that simply has no category.
// This asserts they agree.
func TestDomainNamesMatchTheWireVocabulary(t *testing.T) {
	tests := []struct {
		wire   string
		domain string
	}{
		{file.PrerequisiteAbility, catalog.PrerequisiteAbility.String()},
		{file.PrerequisiteLevel, catalog.PrerequisiteLevel.String()},
		{file.PrerequisiteEntry, catalog.PrerequisiteEntry.String()},

		{file.CastAction, catalog.CastAsAction.String()},
		{file.CastBonusAction, catalog.CastAsBonusAction.String()},
		{file.CastReaction, catalog.CastAsReaction.String()},
		{file.CastOverTime, catalog.CastOverTime.String()},

		{file.RangeSelf, catalog.RangeSelf.String()},
		{file.RangeTouch, catalog.RangeTouch.String()},
		{file.RangeDistance, catalog.RangeDistance.String()},
		{file.RangeSight, catalog.RangeSight.String()},
		{file.RangeUnlimited, catalog.RangeUnlimited.String()},
		{file.RangeSpecial, catalog.RangeSpecial.String()},

		{file.DurationInstantaneous, catalog.DurationInstantaneous.String()},
		{file.DurationTimed, catalog.DurationTimed.String()},
		{file.DurationUntilDispelled, catalog.DurationUntilDispelled.String()},
		{file.DurationSpecial, catalog.DurationSpecial.String()},

		{file.UnitRound, catalog.Round.String()},
		{file.UnitMinute, catalog.Minute.String()},
		{file.UnitHour, catalog.Hour.String()},
		{file.UnitDay, catalog.Day.String()},

		{file.AttackMelee, catalog.MeleeSpellAttack.String()},
		{file.AttackRanged, catalog.RangedSpellAttack.String()},

		{file.SaveNegates, catalog.SaveNegates.String()},
		{file.SaveHalf, catalog.SaveHalvesDamage.String()},
		{file.SaveOther, catalog.SaveOther.String()},

		{file.AreaCone, catalog.AreaCone.String()},
		{file.AreaCube, catalog.AreaCube.String()},
		{file.AreaCylinder, catalog.AreaCylinder.String()},
		{file.AreaLine, catalog.AreaLine.String()},
		{file.AreaSphere, catalog.AreaSphere.String()},

		{file.WeaponSimple, catalog.SimpleWeapon.String()},
		{file.WeaponMartial, catalog.MartialWeapon.String()},
		{file.WeaponMelee, catalog.MeleeWeapon.String()},
		{file.WeaponRanged, catalog.RangedWeapon.String()},

		{file.ArmorLight, catalog.LightArmor.String()},
		{file.ArmorMedium, catalog.MediumArmor.String()},
		{file.ArmorHeavy, catalog.HeavyArmor.String()},
		{file.ArmorShield, catalog.Shield.String()},

		{file.RarityCommon, catalog.RarityCommon.String()},
		{file.RarityUncommon, catalog.RarityUncommon.String()},
		{file.RarityRare, catalog.RarityRare.String()},
		{file.RarityVeryRare, catalog.RarityVeryRare.String()},
		{file.RarityLegendary, catalog.RarityLegendary.String()},
		{file.RarityArtifact, catalog.RarityArtifact.String()},
		{file.RarityVaries, catalog.RarityVaries.String()},

		{file.LanguageStandard, catalog.LanguageStandard.String()},
		{file.LanguageExotic, catalog.LanguageExotic.String()},

		{file.ProficiencyArmor, catalog.ProficiencyArmor.String()},
		{file.ProficiencyWeapons, catalog.ProficiencyWeapons.String()},
		{file.ProficiencyArtisansTools, catalog.ProficiencyArtisansTools.String()},
		{file.ProficiencyGamingSets, catalog.ProficiencyGamingSets.String()},
		{file.ProficiencyMusicalInstruments, catalog.ProficiencyMusicalInstruments.String()},
		{file.ProficiencyOtherTools, catalog.ProficiencyOtherTools.String()},
		{file.ProficiencyVehicles, catalog.ProficiencyVehicles.String()},
		{file.ProficiencySkills, catalog.ProficiencySkills.String()},
		{file.ProficiencySavingThrows, catalog.ProficiencySavingThrows.String()},
	}
	for _, tt := range tests {
		if tt.wire != tt.domain {
			t.Errorf("wire vocabulary says %q, the domain says %q", tt.wire, tt.domain)
		}
	}
}
