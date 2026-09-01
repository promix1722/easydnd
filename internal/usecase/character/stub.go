package character

import (
	"context"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// StubName is the name every stub character is created with.
//
// It is the reference sheet's own name, kept rather than anglicised: the
// character in docs/reference_hexsheet/rouge_3_level.json is somebody's real
// rogue, and a stub that reads "Test Character" would stop being recognisable
// as the sheet the numbers can be checked against.
const StubName = "Сахарок"

// CreateStub builds the reference character in one call.
//
// It is a development aid, not a feature: reaching a character worth looking
// at means five tabs and a dozen answers, the character store is in memory, and
// so every restart costs that walk again. The route that reaches this is
// registered only when the server is in development -- see internal/api/http.
//
// It deliberately does *not* go through Import, even though the reference sheet
// is right there and the importer already reads it. An import records what a
// character is rather than what was chosen, so an imported log answers no
// prompts at all and arrives with every choice still open -- see
// docs/backend.md#importing-states-not-histories. That is the honest way to
// carry a foreign sheet across and the exact opposite of what a stub is for.
//
// So the stub is *built*: Create writes the opening entry and Apply puts the
// rest through validateAndAttribute, the same path an append from the build
// screen takes. Every entry is checked against the prompts open at that moment
// and stamped with the group of the prompt it answers, which means the stub
// cannot quietly stop being a log the build flow could have written -- the
// test in stub_test.go fails first.
func (s *Service) CreateStub(
	ctx context.Context, owner domain.OwnerID, folder domain.FolderID, locale rules.Locale,
) (domain.Character, error) {
	created, err := s.Create(ctx, owner, folder, NewCharacter{
		Name:      StubName,
		Alignment: "neutral",
	})
	if err != nil {
		return domain.Character{}, err
	}
	if _, err := s.Apply(
		ctx, owner, created.ID, locale, created.Log.LastSeq(), stubEvents()...,
	); err != nil {
		return domain.Character{}, err
	}
	return s.Get(ctx, owner, created.ID)
}

// stubEvents is everything after the opening entry: the level-3 half-elf rogue
// (thief) of docs/reference_hexsheet/rouge_3_level.json, as the selections that
// build one.
//
// internal/domain/character/fixtures_test.go transcribes the same character for
// the domain's own tests, and the two are deliberately not shared -- that file
// is package character in the domain, so importing this layer from it would be
// a cycle. They also differ in one place that matters, and it is the thing to
// check when reconciling them: the fixture carries the six scores in the init
// event, which is what an *imported* log looks like. Creation no longer bundles
// them, so here they are their own entry answering character/abilities, and the
// entry a player can point at and change.
//
// Three details are load-bearing and were each wrong in some earlier
// transcription of this character:
//
//   - the scores are the *base* array. Project applies the half-elf's fixed +2
//     Charisma and the two chosen +1s on top, giving the sheet's
//     10/16/14/10/12/14. Recording the final numbers would count the race
//     twice and would mean going back a step could not change it;
//   - the background is acolyte, not the sheet's Urchin, because SRD 5.1
//     publishes exactly one background;
//   - the optional prompts are answered too, and their answers are the only
//     part of this log that is invented rather than transcribed. Seven of them
//     -- the half-elf's third language and all six of acolyte's -- are optional,
//     so the character is *complete* with every one outstanding; it is not
//     *finished*, which is what a stub should be. The export cannot settle them
//     either way: it names no third language, and its background is Urchin.
//     Nothing at all is left open: the stub declares a desired level of 3,
//     which *is* the rogue's third level, and answers what those levels open.
func stubEvents() []domain.Event {
	return []domain.Event{
		// The six scores and the method that produced them: one selection, one
		// entry, closing character/abilities.
		{
			Type: domain.EventChange,
			Changes: []domain.Change{
				{Path: "abilities.method", Op: domain.OpSet, Value: domain.SlugValue("point-buy")},
				{Path: "abilities.str", Op: domain.OpSet, Value: domain.IntValue(10)},
				{Path: "abilities.dex", Op: domain.OpSet, Value: domain.IntValue(15)},
				{Path: "abilities.con", Op: domain.OpSet, Value: domain.IntValue(13)},
				{Path: "abilities.int", Op: domain.OpSet, Value: domain.IntValue(10)},
				{Path: "abilities.wis", Op: domain.OpSet, Value: domain.IntValue(12)},
				{Path: "abilities.cha", Op: domain.OpSet, Value: domain.IntValue(12)},
			},
		},
		{
			Type: domain.EventRace,
			Ref:  rules.NewRef(rules.RefRace, "half-elf"),
			Choices: []domain.Answer{
				// Charisma is the half-elf's fixed +2 and is not on offer here;
				// the prompt chooses two of the other five.
				{Prompt: "half-elf/ability-bonus/0", Picks: []rules.Slug{"dex", "con"}},
				// Skill Versatility is a trait, so its prompt only exists once
				// the race has been chosen -- which is why answering it in the
				// same entry works: validateAndAttribute applies the reference
				// first and checks the answers against what that opened.
				{Prompt: "skill-versatility/proficiency/0", Picks: []rules.Slug{
					"skill-perception", "skill-acrobatics",
				}},
				// The export reads "Common, Elvish, One language of your
				// choice" and never says which, so this one is invented rather
				// than transcribed -- see the note on optional prompts below.
				{Prompt: "half-elf/language/0", Picks: []rules.Slug{"undercommon"}},
			},
		},
		// Acolyte's prompts, and the four personality ones that follow it, are
		// every one of them optional -- so a character can be complete with all
		// of them outstanding. They are answered anyway: "complete" and
		// "finished" are not the same thing, and a stub with seven untouched
		// rows under "still to choose" is a worse example of a built character
		// than one with none.
		//
		// None of this is in the export -- the real character's background is
		// Urchin, which SRD 5.1 does not publish. So these are chosen to suit
		// the character rather than transcribed: a thief who reads the criminal
		// underworld's tongue, an ideal that fits a neutral alignment, and a
		// holy symbol as the one piece of equipment the background grants.
		{
			Type: domain.EventBackground,
			Ref:  rules.NewRef(rules.RefBackground, "acolyte"),
			Choices: []domain.Answer{
				// Two more, and neither may be one the character already holds:
				// common and elvish come from the race, and undercommon was
				// just taken above.
				{Prompt: "acolyte/language/0", Picks: []rules.Slug{"celestial", "draconic"}},
				// The prompt is the holy-symbols equipment category; an amulet
				// is one of its three.
				{Prompt: "acolyte/starting-equipment/0", Picks: []rules.Slug{"amulet"}},
			},
		},
		// Who this character is, which is four sentences and an alignment.
		//
		// They are the acolyte's own suggestions, written out rather than
		// picked: the four roleplaying prompts are text now, so what settles
		// them is the change that settles them -- the same shape the alignment
		// above travels in, and the same shape the build screen posts.
		{
			Type: domain.EventChange,
			Changes: []domain.Change{{
				Path: "identity.personalityTraits", Op: domain.OpSet,
				Value: domain.StringValue(
					"I quote (or misquote) sacred texts and proverbs in almost every situation."),
			}},
		},
		{
			Type: domain.EventChange,
			Changes: []domain.Change{{
				Path: "identity.ideals", Op: domain.OpSet,
				Value: domain.StringValue(
					"Aspiration. I seek to prove myself worthy of my god's favor by matching " +
						"my actions against his or her teachings."),
			}},
		},
		{
			Type: domain.EventChange,
			Changes: []domain.Change{{
				Path: "identity.bonds", Op: domain.OpSet,
				Value: domain.StringValue(
					"I owe my life to the priest who took me in when my parents died."),
			}},
		},
		{
			Type: domain.EventChange,
			Changes: []domain.Change{{
				Path: "identity.flaws", Op: domain.OpSet,
				Value: domain.StringValue(
					"Once I pick a goal, I become obsessed with it to the detriment of " +
						"everything else in my life."),
			}},
		},
		// The declared goal and the rules it is built under. Declaring level 3
		// is the whole of taking those levels -- there are no level entries
		// below, because with one class there is nothing to ask about them.
		{
			Type: domain.EventChange,
			Changes: []domain.Change{
				{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(3)},
				{Path: "identity.ruleset", Op: domain.OpSet, Value: domain.SlugValue("2014")},
			},
		},
		{
			Type:  domain.EventClass,
			Ref:   rules.NewRef(rules.RefClass, "rogue"),
			Level: 1,
			Choices: []domain.Answer{
				{Prompt: "rogue/proficiency/0", Picks: []rules.Slug{
					"skill-deception", "skill-persuasion", "skill-sleight-of-hand", "skill-stealth",
				}},
				// Expertise: two of the skills the rogue is proficient in. The
				// book words it as a choice between "two skills" and "one
				// skill and thieves' tools"; oneList asks it as the single
				// list of two those branches add up to.
				{Prompt: "rogue-expertise-1/expertise/0", Picks: []rules.Slug{
					"skill-persuasion", "skill-stealth",
				}},
				{Prompt: "rogue/starting-equipment/0", Picks: []rules.Slug{"rapier"}},
				// The shortbow-and-arrows bundle has no slug of its own, so it
				// is named by what is in it.
				{Prompt: "rogue/starting-equipment/1", Picks: []rules.Slug{"shortbow+arrow"}},
				{Prompt: "rogue/starting-equipment/2", Picks: []rules.Slug{"burglars-pack"}},
			},
		},
		// A rogue's kit includes leather armor, but wearing it is a decision no
		// rule makes for you -- and armor class depends on it. This entry
		// answers no prompt and closes none, so it is the one the server cannot
		// attribute: it carries no source and sits in no build-screen tab.
		{
			Type: domain.EventChange,
			Changes: []domain.Change{
				{Path: "equipment.equipped", Op: domain.OpAdd, Value: domain.SlugValue("leather-armor")},
			},
		},
		// The Roguish Archetype, which the declared third level is what makes
		// due: nothing offers subclass:thief until the character is level 3,
		// and the declaration above is what makes them level 3. Level 2's
		// Cunning Action needs no entry, because it grants no choice.
		{Type: domain.EventSubclass, Ref: rules.NewRef(rules.RefSubclass, "thief"), Level: 3},
	}
}
