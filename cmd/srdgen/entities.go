package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/promix1722/easydnd/internal/adapter/catalog/file"
)

// indexes extracts the slug from each upstream reference.
func indexes(refs []apiRef) []string {
	if len(refs) == 0 {
		return nil
	}
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Index)
	}
	return out
}

// stacks converts upstream's quantity/item pairs, under either of the two
// keys upstream uses for the reference. See upCount.
func stacks(ups []upCount) []file.ItemStack {
	if len(ups) == 0 {
		return nil
	}
	out := make([]file.ItemStack, 0, len(ups))
	for _, up := range ups {
		out = append(out, file.ItemStack{Item: up.ref().Index, Count: up.Quantity})
	}
	return out
}

// block wraps a single paragraph, dropping empties so an absent field stays
// absent rather than becoming a one-element slice of "".
func block(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return []string{s}
}

func (g *generator) abilities() error {
	ups, err := read[upAbilityScore](g, "5e-SRD-Ability-Scores.json")
	if err != nil {
		return err
	}
	out := make([]file.AbilityScore, 0, len(ups))
	for _, up := range ups {
		out = append(out, file.AbilityScore{Slug: up.Index, Skills: indexes(up.Skills)})
		g.put(file.FileAbilities, up.Index, file.Prose{
			Name:   up.Name,
			Desc:   up.Desc,
			Fields: map[string]string{file.ProseFullName: up.FullName},
		})
	}
	return emit(g, file.FileAbilities, out, func(a file.AbilityScore) string { return a.Slug })
}

func (g *generator) skills() error {
	ups, err := read[upSkill](g, "5e-SRD-Skills.json")
	if err != nil {
		return err
	}
	out := make([]file.Skill, 0, len(ups))
	for _, up := range ups {
		out = append(out, file.Skill{Slug: up.Index, Ability: up.AbilityScore.Index})
		g.put(file.FileSkills, up.Index, file.Prose{Name: up.Name, Desc: up.Desc})
	}
	return emit(g, file.FileSkills, out, func(s file.Skill) string { return s.Slug })
}

func (g *generator) alignments() error {
	ups, err := read[upAlignment](g, "5e-SRD-Alignments.json")
	if err != nil {
		return err
	}
	out := make([]file.Named, 0, len(ups))
	for _, up := range ups {
		out = append(out, file.Named{Slug: up.Index})
		g.put(file.FileAlignments, up.Index, file.Prose{
			Name:   up.Name,
			Desc:   up.Desc,
			Fields: map[string]string{file.ProseAbbreviation: up.Abbreviation},
		})
	}
	return emit(g, file.FileAlignments, out, func(n file.Named) string { return n.Slug })
}

func (g *generator) languages() error {
	ups, err := read[upLanguage](g, "5e-SRD-Languages.json")
	if err != nil {
		return err
	}
	out := make([]file.Language, 0, len(ups))
	for _, up := range ups {
		out = append(out, file.Language{Slug: up.Index, Type: strings.ToLower(up.Type)})
		p := file.Prose{Name: up.Name, Desc: up.Desc}
		if up.Script != "" {
			p.Fields = map[string]string{file.ProseScript: up.Script}
		}
		if len(up.TypicalSpeakers) > 0 {
			p.Blocks = map[string][]string{file.ProseTypicalSpeakers: up.TypicalSpeakers}
		}
		g.put(file.FileLanguages, up.Index, p)
	}
	return emit(g, file.FileLanguages, out, func(l file.Language) string { return l.Slug })
}

// named converts the four collections whose every attribute is prose.
func (g *generator) named(source, dataFile string) error {
	ups, err := read[upNamed](g, source)
	if err != nil {
		return err
	}
	out := make([]file.Named, 0, len(ups))
	for _, up := range ups {
		out = append(out, file.Named{Slug: up.Index})
		g.put(dataFile, up.Index, file.Prose{Name: up.Name, Desc: up.Desc})
	}
	return emit(g, dataFile, out, func(n file.Named) string { return n.Slug })
}

func (g *generator) conditions() error {
	return g.named("5e-SRD-Conditions.json", file.FileConditions)
}

func (g *generator) damageTypes() error {
	return g.named("5e-SRD-Damage-Types.json", file.FileDamageTypes)
}

func (g *generator) magicSchools() error {
	return g.named("5e-SRD-Magic-Schools.json", file.FileMagicSchools)
}

func (g *generator) weaponProperties() error {
	return g.named("5e-SRD-Weapon-Properties.json", file.FileWeaponProperties)
}

// proficiencyTypeFor maps upstream's proficiency type onto the wire's closed
// vocabulary. A closed enum is mapped explicitly rather than slugified,
// because slugify would silently invent a value the loader has never heard of
// the moment upstream rewords a label.
var proficiencyTypeFor = map[string]string{
	"Armor":               file.ProficiencyArmor,
	"Weapons":             file.ProficiencyWeapons,
	"Artisan's Tools":     file.ProficiencyArtisansTools,
	"Gaming Sets":         file.ProficiencyGamingSets,
	"Musical Instruments": file.ProficiencyMusicalInstruments,
	"Other":               file.ProficiencyOtherTools,
	"Vehicles":            file.ProficiencyVehicles,
	"Skills":              file.ProficiencySkills,
	"Saving Throws":       file.ProficiencySavingThrows,
}

func (g *generator) proficiencies() error {
	ups, err := read[upProficiency](g, "5e-SRD-Proficiencies.json")
	if err != nil {
		return err
	}
	out := make([]file.Proficiency, 0, len(ups))
	for _, up := range ups {
		kind, ok := proficiencyTypeFor[up.Type]
		if !ok {
			g.warnf("unknown proficiency type %q on %s", up.Type, up.Index)
		}
		p := file.Proficiency{
			Slug:    up.Index,
			Type:    kind,
			Classes: indexes(up.Classes),
			Races:   indexes(up.Races),
		}
		if up.Reference != nil {
			p.Reference = g.ref(*up.Reference)
		}
		out = append(out, p)
		g.put(file.FileProficiencies, up.Index, file.Prose{Name: up.Name})
	}
	return emit(g, file.FileProficiencies, out, func(p file.Proficiency) string { return p.Slug })
}

func (g *generator) equipmentCategories() error {
	ups, err := read[upEquipmentCategory](g, "5e-SRD-Equipment-Categories.json")
	if err != nil {
		return err
	}
	out := make([]file.EquipmentCategory, 0, len(ups))
	for _, up := range ups {
		out = append(out, file.EquipmentCategory{Slug: up.Index, Items: indexes(up.Equipment)})
		g.put(file.FileEquipmentCategories, up.Index, file.Prose{Name: up.Name})
	}
	return emit(g, file.FileEquipmentCategories, out, func(c file.EquipmentCategory) string { return c.Slug })
}

func (g *generator) races() error {
	ups, err := read[upRace](g, "5e-SRD-Races.json")
	if err != nil {
		return err
	}
	out := make([]file.Race, 0, len(ups))
	for _, up := range ups {
		r := file.Race{
			Slug:                  up.Index,
			Speed:                 up.Speed,
			Size:                  strings.ToLower(up.Size),
			AbilityBonusOptions:   g.choice(up.AbilityBonusOptions, up.Index+"/ability-bonus", 0),
			Languages:             indexes(up.Languages),
			LanguageOptions:       g.choice(up.LanguageOptions, up.Index+"/language", 0),
			StartingProficiencies: indexes(up.StartingProficiencies),
			Traits:                indexes(up.Traits),
			Subraces:              indexes(up.Subraces),
		}
		for _, b := range up.AbilityBonuses {
			r.AbilityBonuses = append(r.AbilityBonuses, file.AbilityBonus{Ability: b.AbilityScore.Index, Bonus: b.Bonus})
		}
		if c := g.choice(up.StartingProfOptions, up.Index+"/proficiency", 0); c != nil {
			r.ProficiencyOptions = []file.Choice{*c}
		}
		out = append(out, r)

		g.put(file.FileRaces, up.Index, file.Prose{
			Name: up.Name,
			Blocks: map[string][]string{
				file.ProseAge:          block(up.Age),
				file.ProseAlignment:    block(up.Alignment),
				file.ProseSizeDesc:     block(up.SizeDescription),
				file.ProseLanguageDesc: block(up.LanguageDesc),
			},
		})
	}
	return emit(g, file.FileRaces, out, func(r file.Race) string { return r.Slug })
}

func (g *generator) subraces() error {
	ups, err := read[upSubrace](g, "5e-SRD-Subraces.json")
	if err != nil {
		return err
	}
	out := make([]file.Subrace, 0, len(ups))
	for _, up := range ups {
		s := file.Subrace{
			Slug:                  up.Index,
			Race:                  up.Race.Index,
			Traits:                indexes(up.RacialTraits),
			StartingProficiencies: indexes(up.StartingProficiencies),
			LanguageOptions:       g.choice(up.LanguageOptions, up.Index+"/language", 0),
		}
		for _, b := range up.AbilityBonuses {
			s.AbilityBonuses = append(s.AbilityBonuses, file.AbilityBonus{Ability: b.AbilityScore.Index, Bonus: b.Bonus})
		}
		out = append(out, s)
		g.put(file.FileSubraces, up.Index, file.Prose{Name: up.Name, Desc: up.Desc})
	}
	return emit(g, file.FileSubraces, out, func(s file.Subrace) string { return s.Slug })
}

func (g *generator) traits() error {
	ups, err := read[upTrait](g, "5e-SRD-Traits.json")
	if err != nil {
		return err
	}
	out := make([]file.Trait, 0, len(ups))
	for _, up := range ups {
		t := file.Trait{
			Slug:               up.Index,
			Races:              indexes(up.Races),
			Subraces:           indexes(up.Subraces),
			Proficiencies:      indexes(up.Proficiencies),
			ProficiencyOptions: g.choice(up.ProficiencyChoices, up.Index+"/proficiency", 0),
		}
		if up.TraitSpecific != nil {
			t.SpellOptions = g.choice(up.TraitSpecific.SpellOptions, up.Index+"/spell", 0)
			t.SubtraitOptions = g.choice(up.TraitSpecific.SubtraitOptions, up.Index+"/subtrait", 0)
			if up.TraitSpecific.DamageType != nil {
				t.DamageResistance = []string{up.TraitSpecific.DamageType.Index}
			}
			if len(up.TraitSpecific.BreathWeapon) > 0 {
				t.BreathWeapon = g.breathWeapon(up.TraitSpecific.BreathWeapon, up.Index)
			}
		}
		out = append(out, t)
		g.put(file.FileTraits, up.Index, file.Prose{Name: up.Name, Desc: up.Desc})
	}
	return emit(g, file.FileTraits, out, func(t file.Trait) string { return t.Slug })
}

// breathWeapon converts the dragonborn's ancestry-keyed breath attack, which
// upstream stores as a bare action object rather than as a choice.
func (g *generator) breathWeapon(raw json.RawMessage, owner string) *file.Choice {
	var v struct {
		Name   string `json:"name"`
		Damage []struct {
			DamageType        *apiRef           `json:"damage_type"`
			DamageAtCharLevel map[string]string `json:"damage_at_character_level"`
		} `json:"damage"`
		DC *struct {
			DCType apiRef `json:"dc_type"`
		} `json:"dc"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		g.warnf("breath weapon for %s: %v", owner, err)
		return nil
	}
	opt := file.Option{Kind: file.OptionDamage, Key: slugify(v.Name)}
	if len(v.Damage) > 0 {
		if v.Damage[0].DamageType != nil {
			opt.DamageType = v.Damage[0].DamageType.Index
		}
		opt.Dice = v.Damage[0].DamageAtCharLevel["1"]
	}
	return &file.Choice{
		Prompt: owner + "/breath-weapon/0",
		Choose: 1,
		Kind:   "damage",
		From:   file.OptionSet{Kind: file.OptionSetExplicit, Options: []file.Option{opt}},
	}
}

func (g *generator) classes() error {
	ups, err := read[upClass](g, "5e-SRD-Classes.json")
	if err != nil {
		return err
	}
	out := make([]file.Class, 0, len(ups))
	for _, up := range ups {
		c := file.Class{
			Slug:                     up.Index,
			HitDie:                   up.HitDie,
			SavingThrows:             indexes(up.SavingThrows),
			Proficiencies:            indexes(up.Proficiencies),
			ProficiencyOptions:       g.choices(up.ProficiencyChoices, up.Index+"/proficiency"),
			StartingEquipment:        stacks(up.StartingEquipment),
			StartingEquipmentOptions: g.choices(up.StartingEquipmentOptions, up.Index+"/starting-equipment"),
			Subclasses:               indexes(up.Subclasses),
		}
		p := file.Prose{Name: up.Name}
		if up.Spellcasting != nil {
			c.SpellcastingLevel = up.Spellcasting.Level
			c.SpellcastingAbility = up.Spellcasting.SpellcastingAbility.Index
			// Sections are flattened to alternating title and body, the
			// flattest encoding a translator cannot reorder into nonsense.
			var blocks []string
			for _, info := range up.Spellcasting.Info {
				blocks = append(blocks, info.Name, strings.Join(info.Desc, "\n\n"))
			}
			p.Blocks = map[string][]string{file.ProseSpellcasting: blocks}
		}
		if up.MultiClassing != nil {
			for _, pre := range up.MultiClassing.Prerequisites {
				c.MulticlassPrerequisites = append(c.MulticlassPrerequisites, file.Prerequisite{
					Kind:         file.PrerequisiteAbility,
					Ability:      pre.AbilityScore.Index,
					MinimumScore: pre.MinimumScore,
				})
			}
			c.MulticlassProficiencies = indexes(up.MultiClassing.Proficiencies)
			c.MulticlassOptions = g.choices(up.MultiClassing.ProficiencyChoices, up.Index+"/multiclass")
		}
		out = append(out, c)
		g.put(file.FileClasses, up.Index, p)
	}
	return emit(g, file.FileClasses, out, func(c file.Class) string { return c.Slug })
}

func (g *generator) classLevels() error {
	ups, err := read[upLevel](g, "5e-SRD-Levels.json")
	if err != nil {
		return err
	}
	out := make([]file.ClassLevel, 0, len(ups))
	for _, up := range ups {
		l := file.ClassLevel{
			Class:               up.Class.Index,
			Level:               up.Level,
			ProficiencyBonus:    up.ProfBonus,
			AbilityScoreBonuses: up.AbilityScoreBonuses,
			Features:            indexes(up.Features),
		}
		if up.Subclass != nil {
			l.Subclass = up.Subclass.Index
		}
		for key, n := range up.RawSpellcasting {
			switch {
			case key == "cantrips_known":
				l.CantripsKnown = n
			case key == "spells_known":
				l.SpellsKnown = n
			case strings.HasPrefix(key, "spell_slots_level_"):
				if l.SpellSlots == nil {
					l.SpellSlots = make(map[string]int)
				}
				if n > 0 {
					l.SpellSlots[strings.TrimPrefix(key, "spell_slots_level_")] = n
				}
			default:
				g.warnf("unknown spellcasting key %q at %s level %d", key, l.Class, l.Level)
			}
		}
		for key, raw := range up.ClassSpecific {
			if res, ok := g.levelResource(key, raw, l.Class, l.Level); ok {
				l.Resources = append(l.Resources, res)
			}
		}
		sortResources(l.Resources)
		out = append(out, l)
	}
	return emit(g, file.FileClassLevels, out, func(l file.ClassLevel) string {
		owner := l.Class
		if l.Subclass != "" {
			owner = l.Subclass
		}
		return fmt.Sprintf("%s-%02d", owner, l.Level)
	})
}

// levelResource converts one class_specific value.
//
// Upstream types these inconsistently -- an integer for ki points, a
// {dice_count, dice_value} object for a martial arts die, a fraction for a
// druid's maximum Wild Shape challenge rating -- so the decode tries each
// shape in turn rather than assuming one.
func (g *generator) levelResource(key string, raw json.RawMessage, class string, level int) (file.LevelResource, bool) {
	res := file.LevelResource{Key: slugify(key)}

	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		res.Number = n
		return res, true
	}

	var dice struct {
		DiceCount int `json:"dice_count"`
		DiceValue int `json:"dice_value"`
	}
	if err := json.Unmarshal(raw, &dice); err == nil && dice.DiceValue > 0 {
		res.Dice = fmt.Sprintf("%dd%d", dice.DiceCount, dice.DiceValue)
		res.Number = dice.DiceCount
		return res, true
	}

	var flag bool
	if err := json.Unmarshal(raw, &flag); err == nil {
		// A boolean resource is a capability rather than a count -- whether a
		// druid's Wild Shape may fly. Storing it as 0 or 1 keeps every
		// resource one shape; the feature prose says what it means.
		if flag {
			res.Number = 1
		}
		return res, true
	}

	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		// A challenge rating below 1 is printed as a fraction; rounding it
		// would quietly upgrade what a low-level druid can become.
		res.Text = formatFraction(f)
		return res, true
	}

	// Anything else is a structured table -- the sorcerer's spell-slot
	// creation costs are the only case -- and belongs with the feature prose
	// that already describes it, not in a numeric resource row.
	var probe []json.RawMessage
	if err := json.Unmarshal(raw, &probe); err == nil {
		return file.LevelResource{}, false
	}

	g.warnf("unconvertible class_specific %q at %s level %d", key, class, level)
	return file.LevelResource{}, false
}

// formatFraction renders a challenge rating the way the SRD prints it.
func formatFraction(f float64) string {
	switch f {
	case 0.125:
		return "1/8"
	case 0.25:
		return "1/4"
	case 0.5:
		return "1/2"
	default:
		return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.3f", f), "0"), ".")
	}
}

func sortResources(rs []file.LevelResource) {
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && rs[j].Key < rs[j-1].Key; j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}

func (g *generator) subclasses() error {
	ups, err := read[upSubclass](g, "5e-SRD-Subclasses.json")
	if err != nil {
		return err
	}
	out := make([]file.Subclass, 0, len(ups))
	for _, up := range ups {
		s := file.Subclass{Slug: up.Index, Class: up.Class.Index}
		for _, sp := range up.Spells {
			// Upstream states the level as prose ("Cleric 1"), so the
			// trailing number is what carries the mechanics.
			level := 0
			for _, pre := range sp.Prerequisites {
				if pre.Type == "level" {
					if n, err := strconv.Atoi(lastField(pre.Name)); err == nil {
						level = n
					}
				}
			}
			s.Spells = append(s.Spells, file.SubclassSpell{Spell: sp.Spell.Index, Level: level})
		}
		out = append(out, s)
		g.put(file.FileSubclasses, up.Index, file.Prose{
			Name:   up.Name,
			Desc:   up.Desc,
			Fields: map[string]string{file.ProseFlavor: up.SubclassFlavor},
		})
	}
	return emit(g, file.FileSubclasses, out, func(s file.Subclass) string { return s.Slug })
}

func (g *generator) features() error {
	ups, err := read[upFeature](g, "5e-SRD-Features.json")
	if err != nil {
		return err
	}
	out := make([]file.Feature, 0, len(ups))
	for _, up := range ups {
		f := file.Feature{Slug: up.Index, Level: up.Level}
		if up.Class != nil {
			f.Class = up.Class.Index
		}
		if up.Subclass != nil {
			f.Subclass = up.Subclass.Index
		}
		if up.Parent != nil {
			f.Parent = up.Parent.Index
		}
		for _, pre := range up.Prerequisites {
			switch pre.Type {
			case "level":
				f.Prerequisites = append(f.Prerequisites, file.Prerequisite{Kind: file.PrerequisiteLevel, Level: pre.Level})
			case "feature":
				f.Prerequisites = append(f.Prerequisites, file.Prerequisite{
					Kind: file.PrerequisiteEntry, Ref: file.Ref("feature:" + slugify(pre.Feature)),
				})
			case "spell":
				f.Prerequisites = append(f.Prerequisites, file.Prerequisite{
					Kind: file.PrerequisiteEntry, Ref: file.Ref("spell:" + slugify(pre.Spell)),
				})
			}
		}
		if s := up.FeatureSpecific; s != nil {
			f.ExpertiseOptions = g.choice(s.ExpertiseOptions, up.Index+"/expertise", 0)
			f.SubfeatureOptions = g.choice(s.SubfeatureOptions, up.Index+"/subfeature", 0)
			f.EnemyTypeOptions = g.choice(s.EnemyTypeOptions, up.Index+"/enemy-type", 0)
			f.TerrainTypeOptions = g.choice(s.TerrainTypeOptions, up.Index+"/terrain-type", 0)
			f.Invocations = indexes(s.Invocations)
		}
		out = append(out, f)
		g.put(file.FileFeatures, up.Index, file.Prose{Name: up.Name, Desc: up.Desc})
	}
	return emit(g, file.FileFeatures, out, func(f file.Feature) string { return f.Slug })
}

func (g *generator) backgrounds() error {
	ups, err := read[upBackground](g, "5e-SRD-Backgrounds.json")
	if err != nil {
		return err
	}
	out := make([]file.Background, 0, len(ups))
	for _, up := range ups {
		b := file.Background{
			Slug:                     up.Index,
			StartingProficiencies:    indexes(up.StartingProficiencies),
			LanguageOptions:          g.choice(up.LanguageOptions, up.Index+"/language", 0),
			StartingEquipment:        stacks(up.StartingEquipment),
			StartingEquipmentOptions: g.choices(up.StartingEquipmentOptions, up.Index+"/starting-equipment"),
			PersonalityTraits:        g.choice(up.PersonalityTraits, up.Index+"/personality", 0),
			Ideals:                   g.choice(up.Ideals, up.Index+"/ideal", 0),
			Bonds:                    g.choice(up.Bonds, up.Index+"/bond", 0),
			Flaws:                    g.choice(up.Flaws, up.Index+"/flaw", 0),
		}
		p := file.Prose{Name: up.Name}
		if up.Feature != nil {
			// The background's feature has no entry of its own upstream, so
			// it becomes a feature slug scoped to the background.
			slug := up.Index + "-" + slugify(up.Feature.Name)
			b.Feature = slug
			g.put(file.FileFeatures, slug, file.Prose{Name: up.Feature.Name, Desc: up.Feature.Desc})
		}
		out = append(out, b)
		g.put(file.FileBackgrounds, up.Index, p)
	}
	return emit(g, file.FileBackgrounds, out, func(b file.Background) string { return b.Slug })
}

func (g *generator) feats() error {
	ups, err := read[upFeat](g, "5e-SRD-Feats.json")
	if err != nil {
		return err
	}
	out := make([]file.Feat, 0, len(ups))
	for _, up := range ups {
		f := file.Feat{Slug: up.Index}
		for _, pre := range up.Prerequisites {
			f.Prerequisites = append(f.Prerequisites, file.Prerequisite{
				Kind:         file.PrerequisiteAbility,
				Ability:      pre.AbilityScore.Index,
				MinimumScore: pre.MinimumScore,
			})
		}
		out = append(out, f)
		g.put(file.FileFeats, up.Index, file.Prose{Name: up.Name, Desc: up.Desc})
	}
	return emit(g, file.FileFeats, out, func(f file.Feat) string { return f.Slug })
}

func (g *generator) equipment() error {
	ups, err := read[upEquipment](g, "5e-SRD-Equipment.json")
	if err != nil {
		return err
	}
	out := make([]file.Item, 0, len(ups))
	for _, up := range ups {
		it := file.Item{
			Slug:     up.Index,
			Category: up.EquipmentCategory.Index,
			Cost:     file.Cost{Amount: up.Cost.Quantity, Unit: up.Cost.Unit},
			Weight:   up.Weight,
		}
		p := file.Prose{Name: up.Name, Desc: up.Desc}

		if up.WeaponCategory != "" {
			w := &file.Weapon{
				Category:   slugify(up.WeaponCategory),
				Range:      strings.ToLower(up.WeaponRange),
				Properties: indexes(up.Properties),
			}
			if up.Damage != nil {
				w.Dice = up.Damage.DamageDice
				if up.Damage.DamageType != nil {
					w.DamageType = up.Damage.DamageType.Index
				}
			}
			if up.TwoHandedDamage != nil {
				w.TwoHandedDice = up.TwoHandedDamage.DamageDice
				if up.TwoHandedDamage.DamageType != nil {
					w.TwoHandedDamageType = up.TwoHandedDamage.DamageType.Index
				}
			}
			if up.Range != nil {
				w.NormalRange = up.Range.Normal
				if up.Range.Long != nil {
					w.LongRange = *up.Range.Long
				}
			}
			if up.ThrowRange != nil {
				w.ThrowNormal = up.ThrowRange.Normal
				w.ThrowLong = up.ThrowRange.Long
			}
			it.Weapon = w
		}

		if up.ArmorClass != nil {
			it.Armor = &file.Armor{
				Category:            slugify(up.ArmorCategory),
				BaseAC:              up.ArmorClass.Base,
				AddsDexBonus:        up.ArmorClass.DexBonus,
				MaxDexBonus:         up.ArmorClass.MaxBonus,
				StrengthMinimum:     up.StrMinimum,
				StealthDisadvantage: up.StealthDisadvantage,
			}
		}

		if up.GearCategory != nil {
			it.Gear = &file.Gear{
				GearCategory: up.GearCategory.Index,
				Quantity:     up.Quantity,
				Contents:     stacks(up.Contents),
			}
		}

		if up.ToolCategory != "" {
			it.Tool = &file.Tool{ToolCategory: slugify(up.ToolCategory)}
		}

		if up.VehicleCategory != "" {
			v := &file.Vehicle{VehicleCategory: slugify(up.VehicleCategory)}
			fields := map[string]string{}
			if up.Speed != nil {
				v.Speed = up.Speed.Quantity
				fields[file.ProseSpeedUnit] = up.Speed.Unit
			}
			if up.Capacity != "" {
				fields[file.ProseCapacity] = up.Capacity
			}
			if len(fields) > 0 {
				p.Fields = fields
			}
			it.Vehicle = v
		}

		out = append(out, it)
		g.put(file.FileEquipment, up.Index, p)
	}
	return emit(g, file.FileEquipment, out, func(i file.Item) string { return i.Slug })
}

func (g *generator) magicItems() error {
	ups, err := read[upMagicItem](g, "5e-SRD-Magic-Items.json")
	if err != nil {
		return err
	}
	out := make([]file.MagicItem, 0, len(ups))
	for _, up := range ups {
		m := file.MagicItem{
			Slug:      up.Index,
			Category:  up.EquipmentCategory.Index,
			Variants:  indexes(up.Variants),
			IsVariant: up.Variant,
		}
		if up.Rarity != nil {
			m.Rarity = slugify(up.Rarity.Name)
		}
		out = append(out, m)
		g.put(file.FileMagicItems, up.Index, file.Prose{Name: up.Name, Desc: up.Desc})
	}
	return emit(g, file.FileMagicItems, out, func(m file.MagicItem) string { return m.Slug })
}

func (g *generator) spells() error {
	ups, err := read[upSpell](g, "5e-SRD-Spells.json")
	if err != nil {
		return err
	}
	out := make([]file.Spell, 0, len(ups))
	for _, up := range ups {
		castingTime, err := parseCastingTime(up.CastingTime)
		if err != nil {
			g.warnf("%s: %v", up.Index, err)
		}
		spellRange, err := parseSpellRange(up.Range)
		if err != nil {
			g.warnf("%s: %v", up.Index, err)
		}
		duration, err := parseDuration(up.Duration)
		if err != nil {
			g.warnf("%s: %v", up.Index, err)
		}

		s := file.Spell{
			Slug:          up.Index,
			Level:         up.Level,
			School:        up.School.Index,
			CastingTime:   castingTime,
			Range:         spellRange,
			Duration:      duration,
			Components:    components(up.Components),
			Ritual:        up.Ritual,
			Concentration: up.Concentration,
			AttackType:    up.AttackType,
			Classes:       indexes(up.Classes),
			Subclasses:    indexes(up.Subclasses),
		}
		if up.DC != nil {
			s.Save = &file.SavingThrow{Ability: up.DC.DCType.Index, Success: saveOutcome(up.DC.DCSuccess)}
		}
		if up.Damage != nil {
			d := &file.SpellDamage{Scaling: file.SpellScaling{
				AtSlotLevel:      up.Damage.DamageAtSlotLevel,
				AtCharacterLevel: up.Damage.DamageAtCharacterLevel,
			}}
			if up.Damage.DamageType != nil {
				d.Type = up.Damage.DamageType.Index
			}
			s.Damage = d
		}
		if len(up.HealAtSlotLevel) > 0 {
			s.Heal = &file.SpellScaling{AtSlotLevel: up.HealAtSlotLevel}
		}
		if up.AreaOfEffect != nil {
			s.Area = &file.AreaOfEffect{Kind: strings.ToLower(up.AreaOfEffect.Type), Size: up.AreaOfEffect.Size}
		}
		out = append(out, s)

		p := file.Prose{Name: up.Name, Desc: up.Desc}
		if up.Material != "" {
			p.Fields = map[string]string{file.ProseMaterial: up.Material}
		}
		if len(up.HigherLevel) > 0 {
			p.Blocks = map[string][]string{file.ProseHigherLevel: up.HigherLevel}
		}
		g.put(file.FileSpells, up.Index, p)
	}
	return emit(g, file.FileSpells, out, func(s file.Spell) string { return s.Slug })
}

func components(cs []string) file.Components {
	var out file.Components
	for _, c := range cs {
		switch strings.ToUpper(c) {
		case "V":
			out.Verbal = true
		case "S":
			out.Somatic = true
		case "M":
			out.Material = true
		}
	}
	return out
}

// saveOutcome maps upstream's dc_success onto the wire vocabulary.
func saveOutcome(s string) string {
	switch s {
	case "none":
		return file.SaveNegates
	case "half":
		return file.SaveHalf
	default:
		return file.SaveOther
	}
}

// lastField returns the final whitespace-separated word of s.
func lastField(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}
