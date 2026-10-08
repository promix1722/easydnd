package character

import (
	"slices"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

// This is the golden test. It projects the level-3 half-elf rogue that
// docs/reference_hexsheet/rouge_3_level.json exports from a real character
// sheet, and checks the derived numbers against it.
//
// Reading the export takes one piece of decoding: HexSheet's
// skillsTools[].modifier is the *proficiency contribution*, not the total. A
// 2 means proficient and a 4 means Expertise, which is why Persuasion and
// Stealth read 4 there and why the character must have answered the Expertise
// prompt the original fixture omitted.
//
// Three of the export's fields are hand-entered and do not follow the rules,
// so they are deliberately not asserted:
//
//   - initiative reads 1, where Dexterity 16 gives +3
//   - the per-skill modifiers omit the ability modifier, being only the
//     proficiency part
//   - the background is Urchin, which SRD 5.1 does not publish
//
// Everything else in the export is reproduced exactly.
func rogueSheet(t *testing.T) State {
	t.Helper()
	state, err := Project(RogueLog(t), LoadCatalog(t))
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	return state
}

func TestProjectRogueIdentityAndAbilities(t *testing.T) {
	s := rogueSheet(t)

	if s.Identity.Name != "Сахарок" {
		t.Errorf("name = %q, want Сахарок", s.Identity.Name)
	}
	if s.Identity.Alignment != "neutral" {
		t.Errorf("alignment = %q, want neutral", s.Identity.Alignment)
	}
	if s.Identity.Race != "half-elf" || s.Identity.Background != "acolyte" {
		t.Errorf("race/background = %q/%q", s.Identity.Race, s.Identity.Background)
	}
	if got := s.Identity.Level(); got != 3 {
		t.Errorf("character level = %d, want 3", got)
	}
	if len(s.Identity.Classes) != 1 ||
		s.Identity.Classes[0].Class != "rogue" ||
		s.Identity.Classes[0].Level != 3 ||
		s.Identity.Classes[0].Subclass != "thief" {
		t.Errorf("classes = %+v, want one rogue 3 / thief", s.Identity.Classes)
	}
	if s.Abilities.Method != "point-buy" {
		t.Errorf("ability method = %q, want point-buy", s.Abilities.Method)
	}

	// Base 10/15/13/10/12/12 plus half-elf's fixed +2 CHA and chosen +1 DEX,
	// +1 CON. These are the export's numbers exactly.
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

func TestProjectRogueStatusBlock(t *testing.T) {
	s := rogueSheet(t)

	if s.Status.ProficiencyBonus != 2 {
		t.Errorf("proficiency bonus = %d, want 2", s.Status.ProficiencyBonus)
	}
	// The export's armorClass.base. Leather armor is BaseAC 11 with an
	// uncapped Dexterity bonus, so 11 + 3.
	if s.Status.ArmorClass != 14 {
		t.Errorf("armor class = %d, want 14", s.Status.ArmorClass)
	}
	// Derived, not copied: the export's own initiative field reads 1.
	if s.Status.Initiative != 3 {
		t.Errorf("initiative = %d, want 3", s.Status.Initiative)
	}
	// 10 + Wisdom 1 + proficiency 2.
	if s.Status.PassivePerception != 13 {
		t.Errorf("passive Perception = %d, want 13", s.Status.PassivePerception)
	}
	// The export's hitPoints.max: 8+2 at first level, then 5+2 twice.
	if s.Base.HitPoints.Max != 24 {
		t.Errorf("hit point maximum = %d, want 24", s.Base.HitPoints.Max)
	}
	if s.Base.HitPoints.Current != 24 {
		t.Errorf("current hit points = %d, want 24", s.Base.HitPoints.Current)
	}
	if len(s.Status.Spellcasting) != 0 {
		t.Errorf("spellcasting = %v, want none: a rogue does not cast", s.Status.Spellcasting)
	}
}

// The Custom slot is the one placement the character records, and it only
// ever names something worn: take the item off, by count or by list, and the
// slot forgets it.
func TestProjectCustomSlotFollowsWhatIsEquipped(t *testing.T) {
	log := RogueLog(t)
	cat := LoadCatalog(t)
	custom := func(slugs ...rules.Slug) Event {
		return Event{Type: EventChange, Changes: []Change{{Path: "equipment.custom", Op: OpSet, Value: SlugListValue(slugs)}}}
	}
	if err := log.Append(custom("leather-armor")); err != nil {
		t.Fatal(err)
	}
	s, err := Project(log, cat)
	if err != nil {
		t.Fatal(err)
	}
	if s.Equipment.Custom != "leather-armor" {
		t.Fatalf("custom = %q, want leather-armor", s.Equipment.Custom)
	}
	if err := log.Append(Event{Type: EventChange, Changes: []Change{{Path: "equipment.equipped.leather-armor", Op: OpSet, Value: IntValue(0)}}}); err != nil {
		t.Fatal(err)
	}
	if s, err = Project(log, cat); err != nil {
		t.Fatal(err)
	}
	if s.Equipment.Custom != "" {
		t.Errorf("custom = %q after taking the armor off by count, want none", s.Equipment.Custom)
	}
	if err := log.Append(
		Event{Type: EventChange, Changes: []Change{{Path: "equipment.equipped", Op: OpSet, Value: SlugListValue([]rules.Slug{"leather-armor"})}}},
		custom("leather-armor"),
		Event{Type: EventChange, Changes: []Change{{Path: "equipment.equipped", Op: OpSet, Value: SlugListValue(nil)}}},
	); err != nil {
		t.Fatal(err)
	}
	if s, err = Project(log, cat); err != nil {
		t.Fatal(err)
	}
	if s.Equipment.Custom != "" {
		t.Errorf("custom = %q after a whole-list set without it, want none", s.Equipment.Custom)
	}
	if err := log.Append(custom("not-an-item")); err != nil {
		t.Fatal(err)
	}
	if _, err = Project(log, cat); err == nil {
		t.Error("a custom slot naming nothing in the catalogue projected")
	}
}

// Armor worn as a counted stack protects exactly as armor worn as a list
// entry does. The import writes stacks, because a sheet prints quantities.
func TestProjectArmorEquippedByCountSetsArmorClass(t *testing.T) {
	log := RogueLog(t)
	if err := log.Append(
		Event{Type: EventChange, Changes: []Change{{Path: "equipment.equipped", Op: OpSet, Value: SlugListValue(nil)}}},
	); err != nil {
		t.Fatal(err)
	}
	bare, err := Project(log, LoadCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	if bare.Status.ArmorClass != 13 {
		t.Fatalf("unarmored armor class = %d, want 10 + DEX 3", bare.Status.ArmorClass)
	}
	if err := log.Append(
		Event{Type: EventChange, Changes: []Change{{Path: "equipment.equipped.leather-armor", Op: OpSet, Value: IntValue(1)}}},
	); err != nil {
		t.Fatal(err)
	}
	worn, err := Project(log, LoadCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	if worn.Status.ArmorClass != 14 {
		t.Errorf("armor class = %d, want leather 11 + DEX 3", worn.Status.ArmorClass)
	}
}

// A counted write edits the stack where it is. The sheet draws the backpack
// in list order, so "one fewer" that deleted and re-appended sent the row to
// the bottom on every use.
func TestProjectCountedWriteKeepsTheStackInPlace(t *testing.T) {
	log := RogueLog(t)
	if err := log.Append(
		Event{Type: EventChange, Changes: []Change{
			{Path: "equipment.backpack", Op: OpSet, Value: SlugListValue(nil)},
			{Path: "equipment.backpack.torch", Op: OpSet, Value: IntValue(2)},
			{Path: "equipment.backpack.candle", Op: OpSet, Value: IntValue(1)},
			{Path: "equipment.backpack.torch", Op: OpSet, Value: IntValue(1)},
		}},
	); err != nil {
		t.Fatal(err)
	}
	got, err := Project(log, LoadCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []ItemStack{{Item: "torch", Count: 1}, {Item: "candle", Count: 1}}
	if !slices.Equal(got.Equipment.Backpack, want) {
		t.Errorf("backpack = %v, want %v", got.Equipment.Backpack, want)
	}
}

// Unarmored Defense is a rule the pack states, not a number armorClass knows:
// a barbarian adds Constitution while wearing no armor and keeps a shield, a
// monk adds Wisdom and loses it to either. Nobody else gets anything.
func TestProjectUnarmoredDefense(t *testing.T) {
	// The installed pack, not the bare compendium: a rule is pack policy, and
	// the plain source carries none.
	cat := spellCatalog(t)
	for _, tt := range []struct {
		name     string
		class    rules.Slug
		equipped []rules.Slug
		want     int
	}{
		{"barbarian, bare", "barbarian", nil, 13},                                // 10 + DEX 2 + CON 1
		{"barbarian with a shield", "barbarian", []rules.Slug{"shield"}, 15},     // + 2
		{"barbarian in leather", "barbarian", []rules.Slug{"leather-armor"}, 13}, // 11 + DEX 2, no CON
		{"monk, bare", "monk", nil, 15},                                          // 10 + DEX 2 + WIS 3
		{"monk with a shield", "monk", []rules.Slug{"shield"}, 14},               // 10 + DEX 2 + 2, no WIS
		{"fighter, bare", "fighter", nil, 12},
	} {
		var log Log
		if err := log.Append(
			Event{Type: EventInit, Changes: []Change{
				{Path: "identity.name", Op: OpSet, Value: StringValue("Bare")},
				{Path: "abilities.dex", Op: OpSet, Value: IntValue(14)},
				{Path: "abilities.con", Op: OpSet, Value: IntValue(13)},
				{Path: "abilities.wis", Op: OpSet, Value: IntValue(16)},
			}},
			Event{Type: EventClass, Ref: rules.NewRef(rules.RefClass, tt.class), Level: 1},
			Event{Type: EventChange, Changes: []Change{{Path: "equipment.equipped", Op: OpSet, Value: SlugListValue(tt.equipped)}}},
		); err != nil {
			t.Fatal(err)
		}
		state, err := Project(log, cat)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if state.Status.ArmorClass != tt.want {
			t.Errorf("%s: armor class = %d, want %d", tt.name, state.Status.ArmorClass, tt.want)
		}
	}
}

func TestProjectRogueSkillsAndSaves(t *testing.T) {
	s := rogueSheet(t)

	// Ability modifier plus the proficiency contribution the export records.
	tests := []struct {
		skill rules.Slug
		want  int
		how   rules.Proficiency
	}{
		{"persuasion", 6, rules.Expertise},       // cha 2 + 2x2
		{"stealth", 7, rules.Expertise},          // dex 3 + 2x2
		{"deception", 4, rules.Proficient},       // cha 2 + 2
		{"sleight-of-hand", 5, rules.Proficient}, // dex 3 + 2
		{"acrobatics", 5, rules.Proficient},      // dex 3 + 2
		{"perception", 3, rules.Proficient},      // wis 1 + 2
	}
	for _, tt := range tests {
		got, ok := s.Skills.BySkill[tt.skill]
		if !ok {
			t.Errorf("%s is not on the sheet", tt.skill)
			continue
		}
		if got.Proficiency != tt.how {
			t.Errorf("%s proficiency = %s, want %s", tt.skill, got.Proficiency, tt.how)
		}
		if got.Bonus != tt.want {
			t.Errorf("%s bonus = %+d, want %+d", tt.skill, got.Bonus, tt.want)
		}
	}

	// The export's savingThrows array is [str, dex, con, int, wis, cha] =
	// [f, t, f, t, f, f], which is the rogue's pair.
	for _, tt := range []struct {
		ability    rules.Ability
		proficient bool
		bonus      int
	}{
		{rules.Strength, false, 0},
		{rules.Dexterity, true, 5},
		{rules.Constitution, false, 2},
		{rules.Intelligence, true, 2},
		{rules.Wisdom, false, 1},
		{rules.Charisma, false, 2},
	} {
		got := s.SavingThrows.ByAbility[tt.ability]
		if got.Proficient != tt.proficient {
			t.Errorf("%s save proficient = %v, want %v", tt.ability, got.Proficient, tt.proficient)
		}
		if got.Bonus != tt.bonus {
			t.Errorf("%s save = %+d, want %+d", tt.ability, got.Bonus, tt.bonus)
		}
	}
}

// Every skill in the compendium is on the sheet, not only the trained ones.
//
// The untrained ones are the point: a player reads a skill list to find out
// what to roll, and the skill nothing trained is what that question is usually
// about. An untrained skill carries the bare ability modifier, which is also
// what proves the seeding runs before the bonuses are derived rather than
// leaving a row of zeroes.
func TestProjectPutsEveryUntrainedSkillOnTheSheet(t *testing.T) {
	cat := LoadCatalog(t)
	s := rogueSheet(t)

	if got, want := len(s.Skills.BySkill), cat.Skills.Len(); got != want {
		t.Errorf("sheet has %d skills, want all %d of them", got, want)
	}
	for _, slug := range cat.Skills.Slugs() {
		if _, ok := s.Skills.BySkill[slug]; !ok {
			t.Errorf("%s is not on the sheet", slug)
		}
	}

	// The rogue's abilities project to str 10, dex 16, con 14, int 10,
	// wis 12, cha 14. Nothing trained any of these four, so each is worth its
	// ability modifier and nothing else.
	for _, tt := range []struct {
		skill rules.Slug
		want  int
	}{
		{"athletics", 0},    // str 10
		{"arcana", 0},       // int 10
		{"survival", 1},     // wis 12
		{"intimidation", 2}, // cha 14
	} {
		got := s.Skills.BySkill[tt.skill]
		if got.Proficiency != rules.NotProficient {
			t.Errorf("%s proficiency = %s, want none", tt.skill, got.Proficiency)
		}
		if got.Bonus != tt.want {
			t.Errorf("%s bonus = %+d, want %+d", tt.skill, got.Bonus, tt.want)
		}
	}
}

// Expertise doubles a proficiency bonus, so it needs one to double.
//
// This guards the seeding above. Nothing validates an expertise answer against
// what the character is actually trained in -- the projector reads the picks
// back out of the log and trusts them -- so the guard is what stops a log that
// names an untrained skill from doubling a bonus that is not there. It used to
// ask whether the skill was in the map at all, which was true only of trained
// skills; now that every skill is in the map, that question answers yes for
// all eighteen and the guard has to ask about the training level instead.
func TestProjectGivesExpertiseOnlyToTrainedSkills(t *testing.T) {
	var log Log
	err := log.Append(
		Event{Type: EventInit, Changes: []Change{
			{Path: "identity.name", Op: OpSet, Value: StringValue("Test")},
			{Path: "abilities.str", Op: OpSet, Value: IntValue(16)},
			{Path: "abilities.dex", Op: OpSet, Value: IntValue(16)},
		}},
		Event{
			Type:  EventClass,
			Ref:   rules.NewRef(rules.RefClass, "rogue"),
			Level: 1,
			Choices: []Answer{
				{Prompt: "rogue/proficiency/0", Picks: []rules.Slug{
					"skill-deception", "skill-persuasion", "skill-sleight-of-hand", "skill-stealth",
				}},
				// Stealth the rogue is trained in; Athletics they are not.
				{Prompt: "rogue-expertise-1/expertise/0", Picks: []rules.Slug{
					"skill-athletics", "skill-stealth",
				}},
			},
		},
	)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	s, err := Project(log, LoadCatalog(t))
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}

	// Strength 16 is +3. Doubling a proficiency bonus the character has not
	// got would read +7 here, and would say "expertise" beside it.
	athletics := s.Skills.BySkill["athletics"]
	if athletics.Proficiency != rules.NotProficient {
		t.Errorf("athletics proficiency = %s, want none: nothing trained it", athletics.Proficiency)
	}
	if athletics.Bonus != 3 {
		t.Errorf("athletics bonus = %+d, want +3", athletics.Bonus)
	}

	// Dexterity 16 is +3, doubled proficiency at level 1 adds 4.
	stealth := s.Skills.BySkill["stealth"]
	if stealth.Proficiency != rules.Expertise {
		t.Errorf("stealth proficiency = %s, want expertise", stealth.Proficiency)
	}
	if stealth.Bonus != 7 {
		t.Errorf("stealth bonus = %+d, want +7", stealth.Bonus)
	}
}

// Passive Perception is 10 plus the Perception bonus, and an untrained
// character still has one -- their Wisdom modifier.
//
// This was wrong for every such character. Perception was absent from the map
// unless something trained it, the lookup returned the zero value, and the
// Wisdom modifier was silently dropped: the sheet read exactly 10 whatever the
// score. The golden rogue never caught it because the acolyte background makes
// them Perception-proficient.
func TestProjectPassivePerceptionWithoutTheProficiency(t *testing.T) {
	// A rogue who answered no proficiency prompts, with Wisdom 12 for +1.
	s := skillLog(t)

	if got := s.Skills.BySkill[perceptionSkill]; got.Proficiency != rules.NotProficient {
		t.Fatalf("perception proficiency = %s, want none", got.Proficiency)
	}
	if got := s.Skills.BySkill[perceptionSkill].Bonus; got != 1 {
		t.Fatalf("perception bonus = %+d, want +1", got)
	}
	if s.Status.PassivePerception != 11 {
		t.Errorf("passive Perception = %d, want 11", s.Status.PassivePerception)
	}
}

func TestProjectRogueTraitsFeaturesAndSenses(t *testing.T) {
	s := rogueSheet(t)

	for _, trait := range []rules.Slug{"darkvision", "fey-ancestry", "skill-versatility"} {
		if !slices.Contains(s.Traits, trait) {
			t.Errorf("traits %v are missing %q", s.Traits, trait)
		}
	}
	// Traits come from the race, features from the class. Keeping them apart
	// is what lets the sheet answer "what did my race give me?".
	for _, feature := range []rules.Slug{
		"rogue-expertise-1", "sneak-attack", "thieves-cant",
		"cunning-action", "roguish-archetype",
		"fast-hands", "second-story-work",
	} {
		if !slices.Contains(s.Features, feature) {
			t.Errorf("features %v are missing %q", s.Features, feature)
		}
	}
	if slices.Contains(s.Features, "darkvision") {
		t.Error("darkvision is a racial trait and must not appear among features")
	}

	// The export's movement list: Walking 30 ft. and Darkvision 60 ft.
	if len(s.Base.Speeds) != 1 || s.Base.Speeds[0].Kind != Walking || s.Base.Speeds[0].Distance != 30 {
		t.Errorf("speeds = %+v, want walking 30", s.Base.Speeds)
	}
	if len(s.Base.Senses) != 1 || s.Base.Senses[0].Kind != Darkvision || s.Base.Senses[0].Distance != 60 {
		t.Errorf("senses = %+v, want darkvision 60", s.Base.Senses)
	}

	// The third language was never picked, so the sheet shows the two the
	// race grants and no more.
	if len(s.Base.Languages) != 2 ||
		!slices.Contains(s.Base.Languages, "common") ||
		!slices.Contains(s.Base.Languages, "elvish") {
		t.Errorf("languages = %v, want common and elvish", s.Base.Languages)
	}
}

func TestProjectRogueResourcesAndEquipment(t *testing.T) {
	s := rogueSheet(t)

	if len(s.Resources.HitDice) != 1 {
		t.Fatalf("hit dice pools = %d, want 1", len(s.Resources.HitDice))
	}
	hd := s.Resources.HitDice[0]
	if hd.Max != 3 || hd.Dice == nil || hd.Dice.String() != "3d8" {
		t.Errorf("hit dice = %d x %v, want 3 x 3d8", hd.Max, hd.Dice)
	}

	// The rogue level-3 row: Sneak Attack 2d6.
	var sneak *Pool
	for i := range s.Resources.Class {
		if s.Resources.Class[i].Key == "sneak-attack" {
			sneak = &s.Resources.Class[i]
		}
	}
	if sneak == nil {
		t.Fatalf("class resources %v have no sneak-attack", s.Resources.Class)
	}
	if sneak.Dice == nil || sneak.Dice.String() != "2d6" {
		t.Errorf("sneak attack dice = %v, want 2d6", sneak.Dice)
	}

	for level := 1; level <= MaxSpellLevel; level++ {
		if s.Resources.SpellSlots[level].Max != 0 {
			t.Errorf("rogue has spell slots at level %d", level)
		}
	}

	// Equipping is an explicit change; everything else stays packed.
	if len(s.Equipment.Equipped) != 1 || s.Equipment.Equipped[0].Item != "leather-armor" {
		t.Errorf("equipped = %+v, want the leather armor", s.Equipment.Equipped)
	}
	for _, want := range []rules.Slug{"dagger", "thieves-tools", "rapier", "shortbow", "burglars-pack"} {
		found := slices.ContainsFunc(s.Equipment.Backpack, func(st ItemStack) bool { return st.Item == want })
		if !found {
			t.Errorf("backpack %v is missing %q", s.Equipment.Backpack, want)
		}
	}
	// Two daggers, not one: a RefOption's Count is a quantity.
	for _, stack := range s.Equipment.Backpack {
		if stack.Item == "dagger" && stack.Count != 2 {
			t.Errorf("daggers = %d, want 2", stack.Count)
		}
		if stack.Item == "arrow" && stack.Count != 20 {
			t.Errorf("arrows = %d, want 20", stack.Count)
		}
	}
}

// Project must be pure: the same log and catalogue give the same sheet, with
// no clock and no randomness. Hit points are the one place that could have
// gone wrong, since the SRD offers a roll -- the fixed average is used
// precisely so that reading a sheet twice cannot change it.
func TestProjectIsDeterministic(t *testing.T) {
	cat := LoadCatalog(t)
	log := RogueLog(t)

	first, err := Project(log, cat)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	for i := range 5 {
		again, err := Project(log, cat)
		if err != nil {
			t.Fatalf("Project() error = %v", err)
		}
		if again.Base.HitPoints.Max != first.Base.HitPoints.Max ||
			again.Status.ArmorClass != first.Status.ArmorClass ||
			len(again.Features) != len(first.Features) {
			t.Fatalf("projection %d differs from the first", i)
		}
	}
}

// A rolled hit point total is recorded as a change, and must beat the average
// the rules derive -- otherwise the projection would recompute the ruling
// away.
func TestChangeOverridesDerivedHitPoints(t *testing.T) {
	log := RogueLog(t)
	if err := log.Append(Event{
		Type: EventChange,
		At:   time.Now(),
		Changes: []Change{
			{Path: "hitPoints.max", Op: OpIncrement, Value: IntValue(5)},
		},
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	s, err := Project(log, LoadCatalog(t))
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if s.Base.HitPoints.Max != 29 {
		t.Errorf("hit point maximum = %d, want 29 (24 derived + 5 ruled)", s.Base.HitPoints.Max)
	}
}

// A change to a path that addresses nothing is an error, not a silent no-op:
// a ruling the table believes is in effect and is not is worse than a sheet
// that refuses to render.
func TestProjectRejectsAnUnresolvablePath(t *testing.T) {
	log := RogueLog(t)
	if err := log.Append(Event{
		Type:    EventChange,
		At:      time.Now(),
		Changes: []Change{{Path: "morale.courage", Op: OpSet, Value: IntValue(11)}},
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	if _, err := Project(log, LoadCatalog(t)); err == nil {
		t.Error("Project() accepted a change to a path that does not exist")
	}
}

// The formula and the compendium must agree wherever the compendium has an
// opinion, which is every single-class case.
func TestProficiencyBonusMatchesTheData(t *testing.T) {
	cat := LoadCatalog(t)

	for _, class := range cat.Classes.All() {
		for level := 1; level <= 20; level++ {
			row, ok := cat.ClassLevel(class.Slug, level)
			if !ok {
				continue
			}
			if got := proficiencyBonus(level); got != row.ProficiencyBonus {
				t.Errorf("%s level %d: formula = %d, compendium = %d",
					class.Slug, level, got, row.ProficiencyBonus)
			}
		}
	}
}

// The action list is derived from what is wielded and what the pack tagged.
// The actions a pack gives everybody live in its mechanics, which this
// package's bare Source does not read; the file adapter's tests cover them.
func TestProjectActionsComeFromWeaponsAndTags(t *testing.T) {
	log := RogueLog(t)
	if err := log.Append(Event{Type: EventChange, Changes: []Change{
		{Path: "equipment.equipped", Op: OpSet, Value: SlugListValue([]rules.Slug{"rapier", "shortbow", "greataxe"})},
	}}); err != nil {
		t.Fatal(err)
	}
	s, err := Project(log, LoadCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Action{}
	for _, a := range s.Actions {
		got[a.Origin.String()] = a
	}
	attack := func(origin string, toHit int, damage string, reach rules.Feet) {
		t.Helper()
		a, ok := got[origin]
		if !ok || a.ToHit == nil || a.Damage == nil {
			t.Errorf("%s gives no attack: %+v", origin, a)
			return
		}
		if *a.ToHit != toHit || a.Damage.Dice.String() != damage || a.Range != reach || a.Kind != MainAction || a.Category != ActionFromEquipment {
			t.Errorf("%s = to hit %d, damage %s, range %d; want %d, %s, %d", origin, *a.ToHit, a.Damage.Dice, a.Range, toHit, damage, reach)
		}
	}
	// Finesse takes Dexterity 3 over Strength; a rogue is proficient, +2.
	attack("item:rapier", 5, "1d8+3", 5)
	attack("item:shortbow", 5, "1d6+3", 80)
	// No proficiency with a greataxe, and it is swung with Strength.
	strength := s.Abilities.Modifier(rules.Strength)
	if a := got["item:greataxe"]; a.ToHit == nil || *a.ToHit != strength {
		t.Errorf("greataxe to hit = %v, want the bare Strength modifier %d", a.ToHit, strength)
	}
	if a := got["feature:cunning-action"]; a.Kind != BonusAction || a.Category != ActionFromFeature || a.Name == "" {
		t.Errorf("Cunning Action = %+v, want a bonus action from a feature", a)
	}
	// Carried is not wielded.
	if _, ok := got["item:leather-armor"]; ok {
		t.Error("armor produced an action")
	}
	if _, ok := rogueSheetActions(t)["item:rapier"]; ok {
		t.Error("a rapier that is not equipped produced an attack")
	}
}

func rogueSheetActions(t *testing.T) map[string]Action {
	t.Helper()
	out := map[string]Action{}
	for _, a := range rogueSheet(t).Actions {
		out[a.Origin.String()] = a
	}
	return out
}
