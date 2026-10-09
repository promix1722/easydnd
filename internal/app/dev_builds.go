package app

import (
	"context"
	"fmt"

	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// devBuild is one seeded character, as the selections that build it.
//
// A seed is *built*, not asserted: Create writes the opening entry and Apply
// puts the rest through the same validation an append from the build screen
// takes, so every answer is checked against a prompt open at that moment. A
// regenerated compendium that renames a prompt therefore fails the seed -- and
// the start-up check below -- instead of leaving a character
// that opens as "Unfinished" with no race and six tens, which is what the two
// casters were while they were seeded with a class and nothing else.
type devBuild struct {
	class  rules.Slug
	level  int
	scores [6]int // str, dex, con, int, wis, cha: the base array, before the race
	race   character.Event
	events []character.Event // the class, and what its levels ask
}

func answer(prompt rules.Slug, picks ...rules.Slug) character.Answer {
	return character.Answer{Prompt: prompt, Picks: picks}
}

func set(path character.Path, value character.Value) character.Change {
	return character.Change{Path: path, Op: character.OpSet, Value: value}
}

func written(path character.Path, text string) character.Event {
	return character.Event{Type: character.EventChange, Changes: []character.Change{set(path, character.StringValue(text))}}
}

// SRD 5.1 publishes one background, so every seed is an acolyte, with the
// acolyte's own suggested trait, ideal, bond and flaw written out.
func acolyte(languages ...rules.Slug) []character.Event {
	return []character.Event{
		{Type: character.EventBackground, Ref: rules.NewRef(rules.RefBackground, "acolyte"), Choices: []character.Answer{
			answer("acolyte/language/0", languages...),
			answer("acolyte/starting-equipment/0", "amulet"),
			answer("acolyte/starting-equipment/1", "prayer-book"),
		}},
		written("identity.personalityTraits", "I quote (or misquote) sacred texts and proverbs in almost every situation."),
		written("identity.ideals", "Aspiration. I seek to prove myself worthy of my god's favor by matching my actions against his or her teachings."),
		written("identity.bonds", "I owe my life to the priest who took me in when my parents died."),
		written("identity.flaws", "Once I pick a goal, I become obsessed with it to the detriment of everything else in my life."),
	}
}

func classRef(class rules.Slug) rules.Ref { return rules.NewRef(rules.RefClass, class) }

// The fourth level's Ability Score Improvement, taken as two +1s.
func improvement(class rules.Slug, first, second rules.Slug) character.Event {
	prompt := class + "/ability-score-improvement/4"
	return character.Event{Type: character.EventLevel, Ref: classRef(class), Level: 4, Choices: []character.Answer{
		answer(prompt, prompt+"/0"), answer(prompt+"/0", first, second),
	}}
}

var human = character.Event{Type: character.EventRace, Ref: rules.NewRef(rules.RefRace, "human"),
	Choices: []character.Answer{answer("human/language/0", "dwarvish")}}

// The half-elf rogue the project's reference sheet describes, at first level.
// The scores are the base array: the race's +2 Charisma and the two chosen
// +1s are applied on top by the projection.
var devRogue = devBuild{
	class: "rogue", level: 1, scores: [6]int{10, 15, 13, 10, 12, 12},
	race: character.Event{Type: character.EventRace, Ref: rules.NewRef(rules.RefRace, "half-elf"), Choices: []character.Answer{
		answer("half-elf/ability-bonus/0", "dex", "con"),
		answer("skill-versatility/proficiency/0", "skill-perception", "skill-acrobatics"),
		answer("half-elf/language/0", "undercommon"),
	}},
	events: []character.Event{
		{Type: character.EventClass, Ref: classRef("rogue"), Level: 1, Choices: []character.Answer{
			answer("rogue/proficiency/0", "skill-deception", "skill-persuasion", "skill-sleight-of-hand", "skill-stealth"),
			answer("rogue-expertise-1/expertise/0", "skill-persuasion", "skill-stealth"),
			answer("rogue/starting-equipment/main-hand", "rapier"),
			answer("rogue/starting-equipment/backup", "shortbow+arrow"),
			answer("rogue/starting-equipment/pack", "burglars-pack"),
		}},
	},
}

// Two fifth-level casters, for what a first-level rogue cannot show: spell
// slots, Lay on Hands, Channel Divinity, a subclass and an improvement.
var devPaladin = devBuild{
	class: "paladin", level: 5, scores: [6]int{15, 10, 13, 8, 10, 15}, race: human,
	events: []character.Event{
		{Type: character.EventClass, Ref: classRef("paladin"), Level: 1, Choices: []character.Answer{
			answer("paladin/proficiency/0", "skill-athletics", "skill-persuasion"),
			answer("paladin/starting-equipment/main-hand", "longsword"),
			answer("paladin/starting-equipment/off-hand", "shield"),
			answer("paladin/starting-equipment/backup", "javelin"),
			answer("paladin/starting-equipment/pack", "explorers-pack"),
			answer("paladin/starting-equipment/focus", "emblem"),
		}},
		{Type: character.EventLevel, Ref: classRef("paladin"), Level: 2, Choices: []character.Answer{
			answer("paladin-fighting-style/subfeature/0", "fighting-style-defense"),
		}},
		{Type: character.EventSubclass, Ref: rules.NewRef(rules.RefSubclass, "devotion"), Level: 3},
		improvement("paladin", "str", "cha"),
		{Type: character.EventLevel, Ref: classRef("paladin"), Level: 5, Choices: []character.Answer{
			answer("paladin/spell/prepared/5", "bless", "cure-wounds", "shield-of-faith", "command", "aid"),
		}},
	},
}

var devCleric = devBuild{
	class: "cleric", level: 5, scores: [6]int{13, 10, 14, 8, 15, 12}, race: human,
	events: []character.Event{
		{Type: character.EventClass, Ref: classRef("cleric"), Level: 1, Choices: []character.Answer{
			// Not Insight or Religion: the acolyte already has both.
			answer("cleric/proficiency/0", "skill-history", "skill-medicine"),
			answer("cleric/starting-equipment/body", "scale-mail"),
			answer("cleric/starting-equipment/main-hand", "mace"),
			answer("cleric/starting-equipment/backup", "crossbow-light+crossbow-bolt"),
			answer("cleric/starting-equipment/pack", "priests-pack"),
			answer("cleric/starting-equipment/focus", "emblem"),
		}},
		{Type: character.EventSubclass, Ref: rules.NewRef(rules.RefSubclass, "life"), Level: 1},
		{Type: character.EventLevel, Ref: classRef("cleric"), Level: 1, Choices: []character.Answer{
			answer("cleric/spell/cantrip/1", "guidance", "light", "sacred-flame"),
		}},
		{Type: character.EventLevel, Ref: classRef("cleric"), Level: 4, Choices: []character.Answer{
			answer("cleric/ability-score-improvement/4", "cleric/ability-score-improvement/4/0"),
			answer("cleric/ability-score-improvement/4/0", "wis", "con"),
			answer("cleric/spell/cantrip/4", "thaumaturgy"),
		}},
		// Not the Life domain's own spells, which are always prepared.
		{Type: character.EventLevel, Ref: classRef("cleric"), Level: 5, Choices: []character.Answer{
			answer("cleric/spell/prepared/5", "guiding-bolt", "healing-word", "shield-of-faith", "sanctuary", "hold-person", "aid", "dispel-magic", "spirit-guardians"),
		}},
	},
}

// seedCharacter builds one finished character and refuses to return one that
// is not: a seed with a required prompt still open is the bug this file exists
// to prevent, so it fails the start-up rather than the first person to open it.
func seedCharacter(ctx context.Context, chars *charuc.Service, owner character.OwnerID, name string, build devBuild, selected pack.Lock) (character.Character, error) {
	created, err := chars.Create(ctx, owner, "", charuc.NewCharacter{Name: name, Alignment: "neutral", Rules: selected})
	if err != nil {
		return character.Character{}, err
	}
	abilities := []character.Change{set("abilities.method", character.SlugValue("point-buy"))}
	for at, ability := range []string{"str", "dex", "con", "int", "wis", "cha"} {
		abilities = append(abilities, set(character.Path("abilities."+ability), character.IntValue(build.scores[at])))
	}
	languages := []rules.Slug{"celestial", "draconic"}
	events := append([]character.Event{{Type: character.EventChange, Changes: abilities}, build.race}, acolyte(languages...)...)
	events = append(events, character.Event{Type: character.EventChange, Changes: []character.Change{
		set("identity.desiredLevel", character.IntValue(build.level)),
		set("identity.ruleset", character.SlugValue("2014")),
	}})
	events = append(events, build.events...)
	// One entry at a time, so a refused answer names the entry it was in.
	seq := created.Log.LastSeq()
	for at, event := range events {
		if seq, err = chars.Apply(ctx, owner, created.ID, rules.DefaultLocale, seq, event); err != nil {
			return character.Character{}, fmt.Errorf("seed %s, entry %d (%s %s): %w", name, at, event.Type, event.Ref.Canonical(), err)
		}
	}
	// Dressed the way a finished build is: nothing above equips anything.
	if err := chars.AutoEquip(ctx, owner, created.ID, rules.DefaultLocale); err != nil {
		return character.Character{}, fmt.Errorf("seed %s: %w", name, err)
	}
	prompts, err := chars.Prompts(ctx, owner, created.ID, rules.DefaultLocale)
	if err != nil {
		return character.Character{}, err
	}
	for _, prompt := range prompts {
		if !prompt.Optional {
			return character.Character{}, fmt.Errorf("seed %s is unfinished: %s is still open", name, prompt.Choice.Prompt)
		}
	}
	return chars.Get(ctx, owner, created.ID)
}
