package hexsheet

import (
	"fmt"
	"strings"
	"time"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// converter turns one export into a log, accumulating a report as it goes.
type converter struct {
	src sheet
	cat *catalog.Catalog

	report Report

	// race is resolved early because the ability scores depend on it: the
	// projection adds the race's fixed bonuses, so the init event has to
	// record the scores without them.
	race    catalog.Race
	hasRace bool
	changes []character.Change
}

func newConverter(src sheet, cat *catalog.Catalog) *converter {
	return &converter{src: src, cat: cat}
}

// build assembles the log.
//
// The order below is the log's order, not the order things are computed in.
// An init event must come first (Log.Validate insists), and the typed events
// follow it. Which tier a change lands in is decided by its path rather than
// by the event carrying it, so init can seed the ability scores and pin the
// armor class that is derived long afterwards.
func (c *converter) build(at time.Time) (character.Log, error) {
	c.resolveRace()

	c.identity()
	c.abilities()
	c.vitals()
	c.skills()
	c.savingThrows()
	c.languages()
	c.equipment()
	c.conditions()

	events := []character.Event{{
		Type:    character.EventInit,
		At:      at,
		Changes: c.changes,
	}}
	events = append(events, c.structure(at)...)

	var log character.Log
	if err := log.Append(events...); err != nil {
		return character.Log{}, err
	}

	c.noteSkipped()
	c.noteOpen(log)
	return log, nil
}

// set appends a change that replaces a value.
func (c *converter) set(path string, value character.Value) {
	c.changes = append(c.changes, character.Change{
		Path: character.Path(path), Op: character.OpSet, Value: value,
	})
}

func (c *converter) unresolved(field, detail string) {
	c.report.Unresolved = append(c.report.Unresolved, Entry{Field: field, Detail: detail})
}

func (c *converter) skipped(field, detail string) {
	c.report.Skipped = append(c.report.Skipped, Entry{Field: field, Detail: detail})
}

// resolveRace looks the race up ahead of everything else, since the ability
// scores cannot be written without it.
func (c *converter) resolveRace() {
	if c.src.Race == "" {
		return
	}
	idx := newIndex(c.cat.Races, func(r catalog.Race) string { return r.Name })
	slug, ok := idx.find(c.src.Race)
	if !ok {
		c.unresolved("character.race", fmt.Sprintf("%q is not in SRD 5.1", c.src.Race))
		return
	}
	c.race, c.hasRace = c.cat.Races.Get(slug)
}

func (c *converter) identity() {
	c.set("identity.name", character.StringValue(c.src.Name))

	if c.src.Alignment != "" {
		idx := newIndex(c.cat.Alignments, func(a catalog.Alignment) string { return a.Name })
		if slug, ok := idx.find(c.src.Alignment); ok {
			c.set("identity.alignment", character.SlugValue(slug))
		} else {
			c.unresolved("character.alignment",
				fmt.Sprintf("%q is not an SRD alignment", c.src.Alignment))
		}
	}

	for _, f := range []struct {
		path, field, text string
	}{
		{"identity.personalityTraits", "personalityTraits", c.src.PersonalityTraits},
		{"identity.ideals", "ideals", c.src.Ideals},
		{"identity.bonds", "bonds", c.src.Bonds},
		{"identity.flaws", "flaws", c.src.Flaws},
	} {
		if strings.TrimSpace(f.text) != "" {
			c.set(f.path, character.StringValue(f.text))
		}
	}
}

// abilities records the scores the character was generated with.
//
// The export's numbers are final: every racial bonus is folded in. The
// projection adds those bonuses back on every run, so writing the export's
// numbers verbatim would count the race twice -- a half-elf's Charisma 14
// would read 16.
//
// Only the race's *fixed* bonuses are subtracted, because only those are
// stated by the compendium. A half-elf also has two +1s the player placed
// somewhere, and the export does not record where; that prompt is left open
// rather than guessed at, which is why the scores below can be a point or two
// under what the player originally rolled.
func (c *converter) abilities() {
	c.set("abilities.method", character.SlugValue("manual"))

	final := map[rules.Ability]int{
		rules.Strength:     c.src.Abilities.Strength.Score,
		rules.Dexterity:    c.src.Abilities.Dexterity.Score,
		rules.Constitution: c.src.Abilities.Constitution.Score,
		rules.Intelligence: c.src.Abilities.Intelligence.Score,
		rules.Wisdom:       c.src.Abilities.Wisdom.Score,
		rules.Charisma:     c.src.Abilities.Charisma.Score,
	}
	if c.hasRace {
		for _, bonus := range c.race.AbilityBonuses {
			final[bonus.Ability] -= bonus.Bonus
		}
	}
	for _, ability := range rules.Abilities() {
		if score := final[ability]; score > 0 {
			c.set("abilities."+ability.String(), character.IntValue(score))
		}
	}
}

// vitals records the numbers a sheet leads with.
//
// These are pinned rather than derived because the export is the authority on
// what this character's sheet says. HexSheet's own hit point rule may differ
// from the SRD's fixed average, and a player who has been running a character
// at 24 hit points should not open easydnd and find 23.
func (c *converter) vitals() {
	if c.src.HitPoints.Max > 0 {
		c.set("hitPoints.max", character.IntValue(c.src.HitPoints.Max))
		// Current is set after max on purpose: changeHitPoints clamps current
		// to max, so the other order would silently lose a wounded character's
		// actual total.
		c.set("hitPoints.current", character.IntValue(c.src.HitPoints.Current))
	}
	if temp := c.src.TempHitPoints.Current; temp > 0 {
		c.set("hitPoints.temporary", character.IntValue(temp))
	}
	if ac := c.src.ArmorClass.Base; ac > 0 {
		c.set("status.armorClass", character.IntValue(ac))
	}
	c.set("status.initiative", character.IntValue(c.src.Initiative))

	if level := countTrue(c.src.Exhaustion); level > 0 {
		c.set("base.exhaustion", character.IntValue(level))
	}
	if c.src.Inspiration {
		c.set("base.inspiration", character.BoolValue(true))
	}
}

// skills records how trained the character is in each skill and tool.
//
// HexSheet's modifier is the proficiency contribution alone, not the total
// bonus, so it maps onto a proficiency level by comparison with the character's
// proficiency bonus: equal means proficient, double means Expertise, half
// means Jack of All Trades.
func (c *converter) skills() {
	bonus := proficiencyBonus(c.level())

	skills := newIndex(c.cat.Skills, func(s catalog.Skill) string { return s.Name })
	profs := newIndex(c.cat.Proficiencies, func(p catalog.ProficiencyDef) string { return p.Name })

	var tools []rules.Slug
	for _, row := range c.src.SkillsTools {
		level := proficiencyOf(row.Modifier, bonus)
		if level == rules.NotProficient {
			continue
		}

		if slug, ok := skills.find(row.Name); ok {
			c.set("skills."+slug.String(), character.StringValue(level.String()))
			continue
		}
		// Not a skill: a tool, an instrument, a vehicle. Those have no
		// training level on the sheet, only presence in the "other
		// proficiencies" box.
		if slug, ok := profs.find(row.Name); ok {
			tools = append(tools, slug)
			continue
		}
		c.unresolved("character.skillsTools",
			fmt.Sprintf("%q is not an SRD skill or proficiency", row.Name))
	}
	if len(tools) > 0 {
		c.set("proficiencies", character.SlugListValue(tools))
	}
}

// savingThrows reads the export's six-element boolean array.
//
// The array is positional with no key, and the order is the SRD's own:
// Strength, Dexterity, Constitution, Intelligence, Wisdom, Charisma. The
// reference export's [false true false true false false] gives a rogue
// Dexterity and Intelligence, which is what the class grants -- that agreement
// is what pins the reading.
func (c *converter) savingThrows() {
	abilities := rules.Abilities()
	if len(c.src.SavingThrows) != len(abilities) {
		if len(c.src.SavingThrows) > 0 {
			c.skipped("character.savingThrows", fmt.Sprintf(
				"expected %d entries, found %d", len(abilities), len(c.src.SavingThrows)))
		}
		return
	}
	for i, proficient := range c.src.SavingThrows {
		if proficient {
			c.set("savingThrows."+abilities[i].String(), character.BoolValue(true))
		}
	}
}

// languages splits the export's free-text language line.
//
// It is prose, not a list: the reference reads "Common, Elvish, One language of
// your choice". The first two resolve and the third is the prompt text for a
// choice the player never made, so it resolves to nothing and is reported as
// what it is.
func (c *converter) languages() {
	if strings.TrimSpace(c.src.Languages) == "" {
		return
	}
	idx := newIndex(c.cat.Languages, func(l catalog.Language) string { return l.Name })

	var known []rules.Slug
	for _, part := range strings.Split(c.src.Languages, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if slug, ok := idx.find(part); ok {
			known = append(known, slug)
			continue
		}
		c.unresolved("character.languages",
			fmt.Sprintf("%q is not an SRD language", part))
	}
	if len(known) > 0 {
		c.set("base.languages", character.SlugListValue(known))
	}
}

// equipment resolves the three carried lists.
//
// Counts are lost: State.ItemStack has one, but equipment changes always add a
// stack of one, so two daggers arrive as one dagger. That is reported rather
// than worked around, because widening the change vocabulary is a larger
// decision than an importer should make on its own.
func (c *converter) equipment() {
	idx := newIndex(c.cat.Items, func(i catalog.Item) string { return i.Name })

	for _, list := range []struct {
		path, field string
		items       []item
	}{
		{"equipment.equipped", "equippedItems", c.src.EquippedItems},
		{"equipment.backpack", "backpackItems", c.src.BackpackItems},
		{"equipment.loot", "loot", c.src.Loot},
	} {
		var slugs []rules.Slug
		for _, it := range list.items {
			slug, ok := idx.find(it.Name)
			if !ok {
				c.unresolved("character."+list.field,
					fmt.Sprintf("%q is not in SRD 5.1", it.Name))
				continue
			}
			slugs = append(slugs, slug)
			if it.Count > 1 {
				c.skipped("character."+list.field, fmt.Sprintf(
					"%s ×%d imported as one; carried counts are not modelled",
					it.Name, it.Count))
			}
		}
		if len(slugs) > 0 {
			c.set(list.path, character.SlugListValue(slugs))
		}
	}
}

// conditions imports only the conditions actually in effect. HexSheet writes
// all fourteen with an active flag, which is a UI checklist rather than state.
func (c *converter) conditions() {
	idx := newIndex(c.cat.Conditions, func(x catalog.Condition) string { return x.Name })

	var active []rules.Slug
	for _, row := range c.src.Conditions {
		if !row.Active {
			continue
		}
		if slug, ok := idx.find(row.Name); ok {
			active = append(active, slug)
			continue
		}
		c.unresolved("character.conditions",
			fmt.Sprintf("%q is not an SRD condition", row.Name))
	}
	if len(active) > 0 {
		c.set("conditions", character.SlugListValue(active))
	}
}

// structure emits the typed events that say what the character is.
//
// None of them carries an answer. They exist so the sheet names a race and a
// class, and so the traits, features and level-scaled numbers the compendium
// attaches to those actually appear; the prompts they open are the player's to
// answer.
func (c *converter) structure(at time.Time) []character.Event {
	var events []character.Event

	if c.hasRace {
		events = append(events, character.Event{
			Type: character.EventRace, At: at,
			Ref: rules.NewRef(rules.RefRace, c.race.Slug),
		})
	}

	if c.src.Background != "" {
		idx := newIndex(c.cat.Backgrounds, func(b catalog.Background) string { return b.Name })
		if slug, ok := idx.find(c.src.Background); ok {
			events = append(events, character.Event{
				Type: character.EventBackground, At: at,
				Ref: rules.NewRef(rules.RefBackground, slug),
			})
		} else {
			c.unresolved("character.background", fmt.Sprintf(
				"%q is not in SRD 5.1, which publishes only Acolyte", c.src.Background))
		}
	}

	classes := newIndex(c.cat.Classes, func(x catalog.Class) string { return x.Name })
	subclasses := newIndex(c.cat.Subclasses, func(x catalog.Subclass) string { return x.Name })

	for _, taken := range c.classLevels() {
		slug, ok := classes.find(taken.ClassName)
		if !ok {
			c.unresolved("character.classes",
				fmt.Sprintf("%q is not an SRD class", taken.ClassName))
			continue
		}
		ref := rules.NewRef(rules.RefClass, slug)

		// The first level of a class is a class event; every level after it is
		// a level event. Levels are emitted one at a time rather than jumping
		// to the total, because each one is what opens that level's prompts.
		events = append(events, character.Event{
			Type: character.EventClass, At: at, Ref: ref, Level: 1,
		})

		subclassAt := 0
		var subclassRef rules.Ref
		if taken.Subclass != "" {
			if sub, ok := subclasses.find(taken.Subclass); ok {
				subclassRef = rules.NewRef(rules.RefSubclass, sub)
				subclassAt = subclassLevel(c.cat, slug)
			} else {
				c.unresolved("character.classes",
					fmt.Sprintf("%q is not an SRD subclass", taken.Subclass))
			}
		}

		// The level comes first and the subclass follows it, never the other
		// way round. A subclass is due *at* a level, so a log that names it
		// before the level has been taken is a log the build flow could not
		// have written and the replay behind a replacement will not keep --
		// it would drop the subclass as something nothing was asking for.
		for level := 2; level <= taken.Levels; level++ {
			events = append(events, character.Event{
				Type: character.EventLevel, At: at, Ref: ref, Level: level,
			})
			if level == subclassAt && !subclassRef.IsZero() {
				events = append(events, character.Event{
					Type: character.EventSubclass, At: at,
					Ref: subclassRef, Level: level,
				})
			}
		}
		// A subclass taken at first level has no later level event to precede.
		if subclassAt <= 1 && !subclassRef.IsZero() {
			events = append(events, character.Event{
				Type: character.EventSubclass, At: at, Ref: subclassRef, Level: 1,
			})
		}
	}

	return events
}

// classLevels returns the multiclass table, falling back to the flat fields
// for an export that omits it.
func (c *converter) classLevels() []classLevel {
	if len(c.src.Classes) > 0 {
		return c.src.Classes
	}
	if c.src.Class == "" {
		return nil
	}
	levels := c.src.Level
	if levels < 1 {
		levels = 1
	}
	return []classLevel{{
		ClassName: c.src.Class, Subclass: c.src.Subclass, Levels: levels, IsPrimary: true,
	}}
}

// level is the character's total level across classes.
func (c *converter) level() int {
	total := 0
	for _, taken := range c.classLevels() {
		total += taken.Levels
	}
	if total < 1 {
		return 1
	}
	return total
}

// noteSkipped records everything real that the model has no home for.
//
// Each of these is a deliberate omission rather than an oversight, and the
// list is the honest answer to "what did I lose?". Several are waiting on the
// battle tracker, which is where spent resources belong.
func (c *converter) noteSkipped() {
	if p := c.src.Currency; p.CP+p.SP+p.EP+p.GP+p.PP > 0 {
		c.skipped("character.currency", fmt.Sprintf(
			"%d cp, %d sp, %d ep, %d gp, %d pp -- a purse is not imported",
			p.CP, p.SP, p.EP, p.GP, p.PP))
	}
	for _, r := range c.src.ClassResources {
		c.skipped("character.classResources", fmt.Sprintf(
			"%q -- class resources are derived from the class, not imported", r.Name))
	}
	for die, pool := range c.src.HitDicePool {
		if pool.Used > 0 {
			c.skipped("character.hitDicePool", fmt.Sprintf(
				"%d spent %s Hit Dice -- spent resources are not imported", pool.Used, die))
		}
	}
	if n := countTrue(c.src.DeathSaves.Saves) + countTrue(c.src.DeathSaves.Deaths); n > 0 {
		c.skipped("character.deathSaves", "a death save tally is not imported")
	}
	for _, list := range []struct {
		field   string
		actions []action
	}{
		{"actions", c.src.Actions},
		{"bonusActions", c.src.BonusActions},
		{"reactions", c.src.Reactions},
	} {
		for _, a := range list.actions {
			// Actions HexSheet generated from a piece of equipment are not
			// worth reporting: easydnd derives its own from what is equipped,
			// so nothing is actually lost.
			if a.AutoGenerated != "" {
				continue
			}
			c.skipped("character."+list.field, fmt.Sprintf(
				"%q -- actions are not imported", a.Name))
		}
	}
	for _, n := range c.src.CampaignNotes {
		c.skipped("character.campaignNotes", fmt.Sprintf("%q is not imported", n.Name))
	}
	for _, s := range c.src.Spells {
		c.skipped("character.spells", fmt.Sprintf(
			"%q -- spell selection is not modelled yet", s.Name))
	}
}

// noteOpen lists the prompts the import left for the player.
//
// This is the counterpart to answering none of them: the player is told
// exactly what is outstanding rather than discovering it by wandering the
// build screen. A prompt that cannot be listed is not an error -- the log is
// still valid -- so a failure here is swallowed.
func (c *converter) noteOpen(log character.Log) {
	prompts, err := character.Prompts(log, c.cat)
	if err != nil {
		return
	}
	for _, p := range prompts {
		if p.Optional {
			continue
		}
		c.report.Open = append(c.report.Open, p.Choice.Prompt)
	}
}

// proficiencyOf reads HexSheet's modifier as a training level.
func proficiencyOf(modifier, proficiencyBonus int) rules.Proficiency {
	if proficiencyBonus <= 0 {
		return rules.NotProficient
	}
	switch modifier {
	case proficiencyBonus * 2:
		return rules.Expertise
	case proficiencyBonus:
		return rules.Proficient
	case proficiencyBonus / 2:
		return rules.HalfProficient
	}
	// Anything else is a bonus the rules cannot express as a training level --
	// a magic item, a DM's ruling. Proficient is the closest honest reading.
	if modifier > 0 {
		return rules.Proficient
	}
	return rules.NotProficient
}

// subclassLevel is the level at which a class chooses its subclass, read from
// the compendium rather than hardcoded: rogues choose at 3, clerics at 1.
func subclassLevel(cat *catalog.Catalog, class rules.Slug) int {
	lowest := 0
	for _, sub := range cat.Subclasses.All() {
		if sub.Class != class {
			continue
		}
		for _, row := range cat.ClassLevels(sub.Slug) {
			if row.Level > 0 && (lowest == 0 || row.Level < lowest) {
				lowest = row.Level
			}
		}
	}
	if lowest == 0 {
		return 1
	}
	return lowest
}

// proficiencyBonus mirrors the projector's own formula. It is duplicated
// rather than exported because the domain deriving it is the authority; this
// copy only has to agree well enough to read a modifier back as a level, and
// the golden test is what keeps them in step.
func proficiencyBonus(characterLevel int) int {
	if characterLevel < 1 {
		return 2
	}
	return 2 + (characterLevel-1)/4
}

func countTrue(flags []bool) int {
	n := 0
	for _, f := range flags {
		if f {
			n++
		}
	}
	return n
}
