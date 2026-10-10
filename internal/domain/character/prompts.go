package character

import (
	"fmt"
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// PromptGroup says which stage of building a character a prompt belongs to.
//
// It exists because a step counter cannot: prompts nest, and a nested one
// does not exist until its parent is answered, so the total is unknowable
// until the end. Grouping is the honest form of progress.
type PromptGroup uint8

// The stages of building a character.
const (
	PromptGroupNone PromptGroup = iota
	GroupIdentity
	GroupAbilities
	GroupRace
	GroupBackground
	GroupClass
	GroupPersonality
)

var promptGroupNames = map[PromptGroup]string{
	PromptGroupNone: "none",
	GroupIdentity:   "identity",
	GroupAbilities:  "abilities",
	GroupRace:       "race",
	GroupBackground: "background",
	GroupClass:      "class",
	// Who the character is, as against what they are. The background decides
	// what they did before; this is what they are like, and it is the one
	// group whose answers are the player's own words rather than a pick.
	GroupPersonality: "personality",
}

// String returns the group's wire name, or "unknown" outside the enumeration.
func (g PromptGroup) String() string {
	if name, ok := promptGroupNames[g]; ok {
		return name
	}
	return "unknown"
}

// PromptEvent is the event an answer must be posted as.
//
// It is load-bearing rather than decorative. The class a character starts as
// is a class event, a subclass is its own type, and what a level grants is a
// level event carrying that level's answers. A client that decided this for
// itself would be reimplementing the rules in the browser.
type PromptEvent struct {
	Type  EventType
	Ref   rules.Ref
	Level int
}

// Prompt is one question the character still has to answer.
type Prompt struct {
	Blocked []rules.Slug
	Purpose string
	UpTo    bool
	// Choice is the question, in the same grammar the compendium uses for
	// the prompts it poses itself. Prompts the catalogue does not pose --
	// "which race?" -- are synthesised into the same shape rather than into
	// a second vocabulary a client would have to learn.
	Choice rules.Choice

	// Source names the catalogue entry that poses this prompt: the race, the
	// feature. Zero for a synthetic prompt.
	Source rules.Ref

	Group PromptGroup

	// Level is the class level this prompt belongs to, or zero.
	Level int

	// Optional reports that a character is complete without answering it.
	//
	// Without this distinction nothing is ever finished: a character who has
	// not picked their personality traits would read as unfinished forever,
	// and every surface that asks "is this character done?" would answer no.
	Optional bool

	// Event is what the answer must be posted as.
	Event PromptEvent

	// Recommended is the character's class's advice for the six scores: every
	// ability, most important first. Set on the ability-score prompt only, and
	// only once there is a class whose pack gives one.
	Recommended []rules.Ability

	// Held lists the options the character already has from another source.
	//
	// The prompt is *not* narrowed to exclude them. Narrowing would make the
	// question depend on the order the player answered in -- a rogue's four
	// skills offered before the race's two are a different set than after --
	// so going back a step could change what had been legal. Reporting what
	// is held lets the client grey those out while the prompt itself stays a
	// pure function of the log.
	Held []rules.Slug

	// HeldOnly inverts what Held means: the options in it are the only legal
	// answers, rather than the illegal ones.
	//
	// Expertise is why. "Choose two of your skill proficiencies" doubles a
	// proficiency the character already has, so holding a skill is the
	// precondition for picking it, not a conflict with it -- the exact
	// opposite of every other prompt, where picking what you already have
	// wastes the choice. One flag rather than two lists, because a client
	// renders both cases the same way: grey out everything on the wrong side
	// of it.
	HeldOnly bool
}

// optionalPrompt marks a prompt as one a character can advance without.
//
// Optional does not mean unimportant -- a build flow still walks the player
// through every prompt in order. It means only that leaving it open must not
// deadlock the character, and the line is drawn where the SRD's own published
// sheets draw it.
//
// Bonus languages, starting equipment, alignment and the roleplaying picks
// are optional: each leaves an unclaimed benefit rather than an ill-formed
// character, and the reference sheet this project is built against reads
// "Common, Elvish, One language of your choice" -- a real character, played
// at a real table, with the prompt still open.
//
// Proficiency choices are not optional, because they change numbers the
// player will roll with rather than lines in a list, and neither are the
// decisions without which there is no character at all: ability scores, race,
// class, and the subclass and Ability Score Improvement a level makes due.
func optionalPrompt(p Prompt) Prompt {
	p.Optional = true
	return p
}

// Complete reports whether a character has answered everything they must.
func Complete(prompts []Prompt) bool {
	for _, p := range prompts {
		if !p.Optional {
			return false
		}
	}
	return true
}

// Prompts returns everything the character still has to decide, in the order
// a build flow should ask.
//
// It is the counterpart to Project and the reason creation and level-up are
// one code path rather than two: levelling up is raising the character's
// desired level, Project makes that the class's level, and the levels it adds
// pose their own questions here -- the archetype, the improvements, a
// feature's picks -- which arrive in the same list as every other question.
// There is no separate level-up endpoint because there is no separate
// question, and no question at all about which class a level goes into while
// there is one class to go into.
//
// The result is a pure function of the log and the catalogue. In particular
// it does not depend on the order the player answered in, which is what makes
// a Back button safe.
func Prompts(log Log, cat *catalog.Catalog) ([]Prompt, error) {
	cat = WithCustomCatalog(log, cat)
	state, err := Project(log, cat)
	if err != nil {
		return nil, err
	}
	b := &promptBuilder{
		cat:     cat,
		state:   state,
		answers: foldAnswers(log),
		scored:  scoresWereSet(log),
		empty:   log.Len() == 0,
	}
	return b.build(), nil
}

// scoresWereSet reports whether the log has ever set an ability score.
//
// An unset score projects as 10, which is a legal score, so the state alone
// cannot say whether the player has chosen yet.
func scoresWereSet(log Log) bool {
	for _, e := range log.Events {
		for _, ch := range e.Changes {
			segments := ch.Path.Segments()
			if len(segments) == 2 && (segments[0] == "abilities" || segments[0] == "finalAbilities") {
				if _, ok := rules.ParseAbility(segments[1]); ok {
					return true
				}
			}
		}
	}
	return false
}

type promptBuilder struct {
	cat     *catalog.Catalog
	state   State
	answers answers
	scored  bool
	empty   bool
	all     bool

	out []Prompt
}

func (b *promptBuilder) build() []Prompt {
	b.identity()
	b.ruleset()
	b.desiredLevel()
	b.abilities()
	b.race()
	b.background()
	b.personality()
	b.classes()
	b.packRules()
	spells, _ := spellChoices(b.state, b.cat, b.answers, b.all)
	b.out = append(b.out, spells...)
	return b.out
}

// add appends a prompt unless it has already been fully answered.
func (b *promptBuilder) add(p Prompt) {
	if p.Choice.Prompt.IsZero() {
		return
	}
	if !b.all && b.answers.answered(p.Choice) {
		return
	}
	p.Held = b.heldIn(p.Choice)
	p.Blocked = b.blockedIn(p.Choice)
	if p.HeldOnly {
		p.Held = b.expertiseEligible(p.Choice, p.Held)
	}
	// HeldOnly is a statement about picking proficiencies, so it applies to
	// the prompt that picks them and not to a branch selector above it.
	// Expertise's outer prompt chooses between "two skills" and "one skill
	// plus thieves' tools" -- its picks are branch ids, and asking whether
	// the character is proficient in a branch is not a question.
	p.HeldOnly = p.HeldOnly && picksEntries(p.Choice)
	b.out = append(b.out, p)
}

// picksEntries reports whether a prompt's answers name catalogue entries
// rather than further prompts.
func picksEntries(c rules.Choice) bool {
	if c.From.Kind != rules.OptionsExplicit {
		return true
	}
	for _, option := range c.From.Options {
		switch option.(type) {
		case rules.NestedOption, rules.BundleOption:
			return false
		}
	}
	return true
}

// addChoice adds a catalogue prompt, and -- once answered -- the prompts its
// answer opened. That recursion is what makes the rogue's Expertise work:
// choosing the "two skills" branch is what brings the two-skill prompt into
// existence.
func (b *promptBuilder) addChoice(c *rules.Choice, p Prompt) {
	if c == nil {
		return
	}
	resolved := b.cat.ResolveChoice(*c)
	c = &resolved
	p.Choice = *c
	b.add(p)
	if !b.answers.answered(*c) {
		return
	}
	for _, key := range b.answers.picks(c.Prompt) {
		option, ok := rules.FindOption(c.From, key)
		if !ok {
			continue
		}
		b.addOpened(option, p)
	}
}

func (b *promptBuilder) addOpened(o rules.Option, p Prompt) {
	switch opt := o.(type) {
	case rules.NestedOption:
		nested := opt.Choice
		b.addChoice(&nested, p)
	case rules.BundleOption:
		for _, item := range opt.Items {
			b.addOpened(item, p)
		}
	}
}

func (b *promptBuilder) addChoices(cs []rules.Choice, p Prompt) {
	for i := range cs {
		b.addChoice(&cs[i], p)
	}
}

// identity asks for a name on a log that has nothing in it at all. It exists
// so that a character created without an init event -- a client crash between
// the create call and the first append -- has a way back rather than being
// permanently unreadable.
func (b *promptBuilder) identity() {
	if !b.empty {
		return
	}
	b.out = append(b.out, Prompt{
		Choice: rules.Choice{Prompt: "character/init", Choose: 1, Kind: rules.ChooseText},
		Group:  GroupIdentity,
		Event:  PromptEvent{Type: EventInit},
	})
}

// ruleset asks which rules the character is built under, once.
//
// There is exactly one answer today -- the compendium's own ruleset -- and the
// validator holds every answer to it, so the question is a recorded fact
// rather than a decision. It exists so that a 2024 compendium, when one is
// wired in, meets characters that already say which rules they meant.
//
// Required, with the desired level below it, and the two of them are why the
// identity stage is three questions rather than a name: a character is built
// to a level under a set of rules, and both are decisions the sheet cannot
// derive. A log that predates them -- an import -- reads as unfinished until
// they are answered, which is the honest reading: nothing in it says which
// rules it was written under.
func (b *promptBuilder) ruleset() {
	if b.empty || !b.state.Identity.Ruleset.IsZero() {
		return
	}
	b.out = append(b.out, Prompt{
		Choice: rules.Choice{Prompt: "character/ruleset", Choose: 1, Kind: rules.ChooseText},
		Group:  GroupIdentity,
		Event:  PromptEvent{Type: EventChange},
	})
}

// desiredLevel asks what level the character is being built towards.
//
// It is answered by a change event setting identity.desiredLevel, so like the
// ability scores it closes by state rather than by a recorded answer.
func (b *promptBuilder) desiredLevel() {
	if b.empty || b.state.Identity.DesiredLevel > 0 {
		return
	}
	b.out = append(b.out, Prompt{
		Choice: rules.Choice{Prompt: "character/desired-level", Choose: 1, Kind: rules.ChooseLevel},
		Group:  GroupIdentity,
		Event:  PromptEvent{Type: EventChange},
	})
}

func (b *promptBuilder) abilities() {
	if b.scored {
		return
	}
	b.out = append(b.out, Prompt{
		Choice: rules.Choice{
			Prompt: "character/abilities",
			Choose: len(b.cat.AbilityIDs()),
			Kind:   rules.ChooseAbilityScores,
		},
		Group:       GroupAbilities,
		Event:       PromptEvent{Type: EventChange},
		Recommended: b.abilityPriority(),
	})
}

// abilityPriority is the first class's advice, when it names every ability
// exactly once. Anything else -- a pack that lists four, or one twice -- is
// not an order the six scores can be dealt out by, so it is no advice at all.
func (b *promptBuilder) abilityPriority() []rules.Ability {
	if len(b.state.Identity.Classes) == 0 {
		return nil
	}
	class, ok := b.cat.Classes.Get(b.state.Identity.Classes[0].Class)
	if !ok || len(class.AbilityPriority) != len(b.cat.AbilityIDs()) {
		return nil
	}
	seen := map[rules.Ability]bool{}
	for _, ability := range class.AbilityPriority {
		seen[ability] = true
	}
	if len(seen) != len(class.AbilityPriority) {
		return nil
	}
	return class.AbilityPriority
}

func (b *promptBuilder) race() {
	if b.state.Identity.Race.IsZero() {
		b.out = append(b.out, Prompt{
			Choice: rules.Choice{
				Prompt: "character/race",
				Choose: 1,
				Kind:   rules.ChooseRace,
				From:   rules.OptionSet{Kind: rules.OptionsFromCollection, Collection: rules.RefRace},
			},
			Group: GroupRace,
			Event: PromptEvent{Type: EventRace},
		})
		return
	}

	race, ok := b.cat.Races.Get(b.state.Identity.Race)
	if !ok {
		return
	}
	source := rules.NewRef(rules.RefRace, race.Slug)
	base := Prompt{Group: GroupRace, Source: source, Event: PromptEvent{Type: EventRace, Ref: source}}

	b.addChoice(race.AbilityBonusOptions, base)
	b.addChoice(race.LanguageOptions, optionalPrompt(base))
	b.addChoices(race.ProficiencyOptions, base)

	if len(race.Subraces) > 0 && b.state.Identity.Subrace.IsZero() {
		b.out = append(b.out, Prompt{
			Choice: rules.Choice{
				Prompt: "character/subrace",
				Choose: 1,
				Kind:   rules.ChooseSubrace,
				From:   refOptions(rules.RefSubrace, race.Subraces),
			},
			Group:  GroupRace,
			Source: source,
			// A subrace is optional only in the sense that the SRD's four
			// cover four of nine races; where one exists the rules require
			// picking it.
			Event: PromptEvent{Type: EventSubrace},
		})
	}
	if subrace, ok := b.cat.Subraces.Get(b.state.Identity.Subrace); ok {
		subSource := rules.NewRef(rules.RefSubrace, subrace.Slug)
		b.addChoice(subrace.LanguageOptions, optionalPrompt(Prompt{
			Group:  GroupRace,
			Source: subSource,
			Event:  PromptEvent{Type: EventSubrace, Ref: subSource},
		}))
	}

	// Traits pose prompts of their own, and only once the race that grants
	// them has been chosen -- which is why an answer to one necessarily
	// arrives in a later event than the race event that opened it.
	for _, slug := range b.state.Traits {
		trait, ok := b.cat.Traits.Get(slug)
		if !ok {
			continue
		}
		traitSource := rules.NewRef(rules.RefTrait, trait.Slug)
		traitPrompt := Prompt{
			Group:  GroupRace,
			Source: traitSource,
			Event:  PromptEvent{Type: EventRace, Ref: source},
		}
		b.addChoice(trait.ProficiencyOptions, traitPrompt)
		if trait.Specific != nil {
			b.addChoice(trait.Specific.SpellOptions, traitPrompt)
			b.addChoice(trait.Specific.SubtraitOptions, traitPrompt)
			// BreathWeapon describes the attack granted by ancestry. Its
			// legacy choice-shaped payload is not a player decision.
		}
	}
}

func (b *promptBuilder) background() {
	if b.state.Identity.Background.IsZero() {
		b.out = append(b.out, Prompt{
			Choice: rules.Choice{
				Prompt: "character/background",
				Choose: 1,
				Kind:   rules.ChooseBackground,
				From:   rules.OptionSet{Kind: rules.OptionsFromCollection, Collection: rules.RefBackground},
			},
			Group: GroupBackground,
			Event: PromptEvent{Type: EventBackground},
		})
		return
	}

	background, ok := b.cat.Backgrounds.Get(b.state.Identity.Background)
	if !ok {
		return
	}
	source := rules.NewRef(rules.RefBackground, background.Slug)
	base := Prompt{Group: GroupBackground, Source: source, Event: PromptEvent{Type: EventBackground, Ref: source}}

	b.addChoice(background.LanguageOptions, optionalPrompt(base))

	optional := optionalPrompt(base)
	b.addChoices(background.StartingEquipmentOptions, optional)
}

// personality poses who the character is: the four roleplaying questions and
// an alignment.
//
// These are available from the start, independently of the background.
// A background may suggest answers, but the player can write their own first.
//
// The four are asked as **text**, and the SRD's own d8 tables are not offered.
// A trait is the one thing on a character sheet that is nobody's but the
// player's, and a menu of eight makes it the compendium's. The state behind
// them was always free text -- see State.Identity -- so what changes here is
// only that the prompt stops pretending otherwise: each is answered by the
// change that settles it, exactly as the alignment beside them is, and the
// suggestions remain in the compendium for anybody who wants to read them.
//
// Each is posed only while its answer is unset. There is nothing here to
// compare against an option set, so "answered" is a question about the sheet
// rather than about the log -- which is the same rule the alignment follows,
// and the reason neither of them goes through addChoice.
func (b *promptBuilder) personality() {
	written := func(prompt rules.Slug, kind rules.ChoiceKind, set []string) {
		if len(set) > 0 {
			return
		}
		b.out = append(b.out, Prompt{
			Choice: rules.Choice{
				Prompt: prompt,
				// One, whatever the background's table suggests. The count
				// belonged to a menu -- "pick two of these eight" -- and what
				// is asked now is one answer in the player's own words, which
				// can be as long as it wants to be.
				Choose: 1,
				Kind:   kind,
				// Explicit and empty: there is nothing to pick between, and a
				// client tells this apart from a menu by the absence of one.
				From: rules.OptionSet{Kind: rules.OptionsExplicit},
			},
			Group:    GroupPersonality,
			Optional: true,
			Event:    PromptEvent{Type: EventChange},
		})
	}

	id := b.state.Identity
	written("character/personality-trait", rules.ChoosePersonality, id.PersonalityTraits)
	written("character/ideal", rules.ChooseIdeal, id.Ideals)
	written("character/bond", rules.ChooseBond, id.Bonds)
	written("character/flaw", rules.ChooseFlaw, id.Flaws)

	if b.state.Identity.Alignment.IsZero() {
		b.out = append(b.out, Prompt{
			Choice: rules.Choice{
				Prompt: "character/alignment",
				Choose: 1,
				Kind:   rules.ChooseAlignment,
				From:   rules.OptionSet{Kind: rules.OptionsFromCollection, Collection: rules.RefAlignment},
			},
			Group:    GroupPersonality,
			Optional: true,
			Event:    PromptEvent{Type: EventChange},
		})
	}
}

func (b *promptBuilder) classes() {
	if len(b.state.Identity.Classes) == 0 {
		b.out = append(b.out, Prompt{
			Choice: rules.Choice{
				Prompt: "character/class",
				Choose: 1,
				Kind:   rules.ChooseClass,
				From:   rules.OptionSet{Kind: rules.OptionsFromCollection, Collection: rules.RefClass},
			},
			Group: GroupClass,
			Event: PromptEvent{Type: EventClass, Level: 1},
		})
		return
	}

	for i, taken := range b.state.Identity.Classes {
		class, ok := b.cat.Classes.Get(taken.Class)
		if !ok {
			continue
		}
		source := rules.NewRef(rules.RefClass, taken.Class)
		// Level 1, not the level the character has reached. What follows is
		// the grant a class makes when it is *entered* -- its skills, its
		// starting kit -- so a fifth-level barbarian's unanswered skill choice
		// belongs to the level that offered it and files under it. Stamping
		// the current level instead filed every one of them at the end of the
		// class story, under a level that granted nothing.
		base := Prompt{
			Group:  GroupClass,
			Source: source,
			Level:  1,
			Event:  PromptEvent{Type: EventClass, Ref: source, Level: 1},
		}

		g := classGrant(class, 1, i == 0)
		b.addChoices(g.Choices, base)

		if i == 0 {
			optional := base
			optional.Optional = true
			b.addChoices(class.StartingEquipmentOptions, optional)
		}

		b.subclass(class, taken)
		b.levelPrompts(class, taken)
	}
}

// subclass offers the subclass prompt at the level the class's own
// advancement rows say it is due.
func (b *promptBuilder) subclass(class catalog.Class, taken ClassLevel) {
	if !taken.Subclass.IsZero() || len(class.Subclasses) == 0 {
		return
	}
	due := subclassLevel(b.cat, class)
	if due == 0 || taken.Level < due {
		return
	}
	b.out = append(b.out, Prompt{
		Choice: rules.Choice{
			Prompt: rules.Slug(fmt.Sprintf("%s/subclass", class.Slug)),
			Choose: 1,
			Kind:   rules.ChooseSubclass,
			From:   refOptions(rules.RefSubclass, class.Subclasses),
		},
		Group:  GroupClass,
		Source: rules.NewRef(rules.RefClass, class.Slug),
		Level:  due,
		Event:  PromptEvent{Type: EventSubclass, Level: due},
	})
}

// levelPrompts walks the levels already taken in a class and adds whatever
// each one still has open: the prompts its features pose, and the Ability
// Score Improvement.
func (b *promptBuilder) levelPrompts(class catalog.Class, taken ClassLevel) {
	source := rules.NewRef(rules.RefClass, class.Slug)
	for level := 1; level <= taken.Level; level++ {
		row, ok := b.cat.ClassLevel(class.Slug, level)
		if !ok {
			continue
		}
		features := row.Features
		if !taken.Subclass.IsZero() {
			if subRow, ok := b.cat.ClassLevel(taken.Subclass, level); ok {
				features = append(slices.Clone(features), subRow.Features...)
			}
		}
		for _, slug := range features {
			b.featurePrompts(slug, level, source)
		}
		if grantsAbilityScoreImprovement(b.cat, class.Slug, level) {
			b.abilityScoreImprovement(class.Slug, level)
		}
	}
}

func (b *promptBuilder) featurePrompts(slug rules.Slug, level int, class rules.Ref) {
	feature, ok := b.cat.Features.Get(slug)
	if !ok || feature.Specific == nil {
		return
	}
	p := Prompt{
		Group:  GroupClass,
		Source: rules.NewRef(rules.RefFeature, slug),
		Level:  level,
		Event:  PromptEvent{Type: EventLevel, Ref: class, Level: level},
	}
	expertise := p
	expertise.HeldOnly = true
	// oneList, because the rogue's first Expertise is transcribed as a choice
	// between branches and is one list of two. Project reads it through the
	// same function, or an answer to the flat question would not resolve
	// against the nested shape.
	b.addChoice(oneList(feature.Specific.ExpertiseOptions), expertise)
	b.addChoice(feature.Specific.SubfeatureOptions, p)
	b.addChoice(feature.Specific.EnemyTypeOptions, p)
	b.addChoice(feature.Specific.TerrainTypeOptions, p)
}

// abilityScoreImprovement builds the "+2 to one ability, +1 to two, or a
// feat" prompt.
//
// It is synthesised because the SRD data does not carry it: the feature row
// is bare, and only the cumulative AbilityScoreBonuses count marks the level.
// The shape is the compendium's own nested-choice idiom -- choose one branch,
// the branch carries the picks -- so a client that can already render the
// rogue's Expertise can render this with no new code.
func (b *promptBuilder) abilityScoreImprovement(class rules.Slug, level int) {
	prompt := asiPrompt(class, level)
	scores := rules.Choice{
		Prompt: prompt + "/0",
		Choose: abilityScoreIncrease(b.cat),
		Kind:   rules.ChooseAbilityBonus,
		From:   rules.OptionSet{Kind: rules.OptionsExplicit, Options: abilityBonusOptions(b.cat.AbilityIDs())},
		// Two points rather than two scores: both may go into one ability,
		// which is the "+2 to one" half of the rule. This is the only choice
		// in the game that says so -- a half-elf's two look identical and are
		// "two *different* scores" -- which is why it is stated here rather
		// than inferred from the kind.
		Repeatable: true,
	}
	feat := rules.Choice{
		Prompt: prompt + "/1",
		Choose: 1,
		Kind:   rules.ChooseFeature,
		From:   rules.OptionSet{Kind: rules.OptionsFromCollection, Collection: rules.RefFeat},
	}
	b.addChoice(&rules.Choice{
		Prompt: prompt,
		Choose: 1,
		Kind:   rules.ChooseAbilityScores,
		From: rules.OptionSet{Kind: rules.OptionsExplicit, Options: []rules.Option{
			rules.NestedOption{Choice: scores},
			rules.NestedOption{Choice: feat},
		}},
	}, Prompt{
		Group:  GroupClass,
		Source: rules.NewRef(rules.RefClass, class),
		Level:  level,
		Event:  PromptEvent{Type: EventLevel, Ref: rules.NewRef(rules.RefClass, class), Level: level},
	})
}

// asiPrompt is the id of the Ability Score Improvement prompt for one class
// at one level. Prompts emits it and Project reads it, so it lives in one
// place: two spellings of the same synthesised id is an answer that resolves
// on the way in and vanishes on the way out.
func asiPrompt(class rules.Slug, level int) rules.Slug {
	return rules.Slug(fmt.Sprintf("%s/ability-score-improvement/%d", class, level))
}

// abilityBonusOptions is "+1 to any ability", once per ability. Picking the
// same ability twice is the "+2 to one" half of the rule, which the choice
// above allows by being Repeatable.
func abilityBonusOptions(abilities []rules.Ability) []rules.Option {
	out := make([]rules.Option, 0, len(abilities))
	for _, ability := range abilities {
		out = append(out, rules.AbilityBonusOption{Ability: ability, Bonus: 1})
	}
	return out
}

// MaxCharacterLevel is where advancement stops in the 2014 rules.
const MaxCharacterLevel = 20

// refOptions builds an explicit option set naming catalogue entries.
func refOptions(kind rules.RefKind, slugs []rules.Slug) rules.OptionSet {
	options := make([]rules.Option, 0, len(slugs))
	for _, slug := range slugs {
		options = append(options, rules.RefOption{Ref: rules.Ref{Kind: kind, Slug: slug}, Count: 1})
	}
	return rules.OptionSet{Kind: rules.OptionsExplicit, Options: options}
}

// heldIn reports which of a prompt's options the character already has.
//
// Only the cases where a duplicate is actually illegal are reported:
// proficiencies, languages, traits, feats, and features (including fighting
// styles shared by different classes). A second rapier is fine.
//
// It looks inside branches as well as at the options themselves, because the
// client answers a branch in the card that offered it -- so the options it
// draws are the branches' options, and the greying-out has to reach them. The
// monk's "one artisan's tool or one musical instrument" is the case: without
// this, a tool the character already had looked pickable and the server
// refused the answer. Option keys are unique within a prompt, so one flat list
// still says which option each held entry is.
func (b *promptBuilder) heldIn(c rules.Choice) []rules.Slug {
	var held []rules.Slug
	var walk func(options []rules.Option)
	walk = func(options []rules.Option) {
		for _, option := range options {
			switch opt := option.(type) {
			case rules.RefOption:
				if b.holds(opt.Ref) {
					held = append(held, rules.OptionKey(option))
				}
			case rules.NestedOption:
				walk(opt.Choice.From.Options)
			case rules.BundleOption:
				walk(opt.Items)
			}
		}
	}
	walk(c.From.Options)
	if c.From.Kind == rules.OptionsFromCollection && c.From.Collection == rules.RefLanguage {
		held = append(held, b.state.Base.Languages...)
	}
	if c.From.Kind == rules.OptionsFromCollection && c.From.Collection == rules.RefFeat {
		held = append(held, b.state.Feats...)
	}
	return held
}

// Class-specific fighting-style entries describe the same non-repeatable benefit.
func featureIdentity(slug rules.Slug) string {
	s := slug.String()
	if at := strings.Index(s, "fighting-style-"); at >= 0 {
		return s[at:]
	}
	return s
}

func (b *promptBuilder) expertiseEligible(c rules.Choice, held []rules.Slug) []rules.Slug {
	var out []rules.Slug
	for _, slug := range held {
		skill := slug
		if def, ok := b.cat.Proficiencies.Get(slug); ok {
			skill = def.Reference.Slug
		}
		if b.state.Skills.BySkill[skill].Proficiency == rules.Expertise {
			continue
		}
		used := false
		for _, feature := range b.state.Features {
			def, ok := b.cat.Features.Get(feature)
			if !ok || def.Specific == nil {
				continue
			}
			ch := oneList(def.Specific.ExpertiseOptions)
			if ch != nil && ch.Prompt != c.Prompt && slices.Contains(b.answers.slugs(ch), slug) {
				used = true
			}
		}
		if !used {
			out = append(out, slug)
		}
	}
	return out
}

func (b *promptBuilder) holds(ref rules.Ref) bool {
	switch ref.Kind {
	case rules.RefTrait:
		return slices.Contains(b.state.Traits, ref.Slug)
	case rules.RefFeat:
		return slices.Contains(b.state.Feats, ref.Slug)
	case rules.RefFeature:
		for _, held := range b.state.Features {
			if featureIdentity(held) == featureIdentity(ref.Slug) {
				return true
			}
		}
		return false
	case rules.RefLanguage:
		return slices.Contains(b.state.Base.Languages, ref.Slug)
	case rules.RefProficiency:
		if slices.Contains(b.state.Proficiencies, ref.Slug) {
			return true
		}
		def, ok := b.cat.Proficiencies.Get(ref.Slug)
		if !ok {
			return false
		}
		if def.Reference.Kind == rules.RefSkill {
			state, known := b.state.Skills.BySkill[def.Reference.Slug]
			return known && state.Proficiency != rules.NotProficient
		}
		return false
	case rules.RefSkill:
		state, known := b.state.Skills.BySkill[ref.Slug]
		return known && state.Proficiency != rules.NotProficient
	}
	return false
}

func (b *promptBuilder) packRules() {
	for _, r := range b.cat.Mechanics.Rules {
		active, err := activeRule(b.state, b.cat, r)
		if err != nil || !active {
			continue
		}
		for _, ch := range r.Choices {
			p := Prompt{Group: ruleGroup(r.Owner), Source: r.Owner, Level: b.featLevel(r.Owner), Event: PromptEvent{Type: EventRule, Ref: rules.NewRef(rules.RefRule, r.ID)}}
			// Expertise doubles a proficiency already held, whoever asks.
			p.HeldOnly = ch.Kind == rules.ChooseExpertise
			b.addChoice(&ch, p)
		}
	}
}

// ruleGroup files a rule's question with what owns the rule: a background's
// gaming set is asked with the background, not among the class levels.
func ruleGroup(owner rules.Ref) PromptGroup {
	switch owner.Kind {
	case rules.RefRace, rules.RefSubrace, rules.RefTrait:
		return GroupRace
	case rules.RefBackground:
		return GroupBackground
	}
	return GroupClass
}

// featLevel is the class level a feat was taken at, or zero when it was not
// taken at one -- a rule owned by something else, or a feat a race gave.
//
// A feat's own question belongs to the level that brought the feat: Slasher's
// "+1 to Strength or Dexterity" is part of what fourth level asked. Without a
// level it reads as belonging to no level at all, and a build screen draws it
// above first level, ahead of the improvement that opened it.
//
// A feature's question belongs to the level the feature is gained at, which
// its own row says: Student of War's tool is asked with third level.
func (b *promptBuilder) featLevel(owner rules.Ref) int {
	if owner.Kind == rules.RefFeature {
		feature, _ := b.cat.Features.Get(owner.Slug)
		return feature.Level
	}
	if owner.Kind != rules.RefFeat {
		return 0
	}
	for _, taken := range b.state.Identity.Classes {
		for level := 1; level <= taken.Level; level++ {
			if slices.Contains(b.answers.picks(asiPrompt(taken.Class, level)+"/1"), owner.Slug) {
				return level
			}
		}
	}
	return 0
}

func abilityScoreIncrease(cat *catalog.Catalog) int {
	if cat.Mechanics.Core.AbilityScoreIncrease > 0 {
		return cat.Mechanics.Core.AbilityScoreIncrease
	}
	return 2
}

// ResolvedSelections reads answers through their original choice trees. Bundle
// keys are identities, not display labels; flattening here preserves quantities
// and fixed items beside nested choices.
func ResolvedSelections(log Log, cat *catalog.Catalog) (map[rules.Slug]ResolvedChoice, error) {
	state, err := Project(log, cat)
	if err != nil {
		return nil, err
	}
	b := promptBuilder{cat: cat, state: state, answers: foldAnswers(log), scored: scoresWereSet(log), all: true}
	out := map[rules.Slug]ResolvedChoice{}
	for _, prompt := range b.build() {
		resolved := ResolvedChoice{Source: prompt.Source, Kind: prompt.Choice.Kind, Purpose: prompt.Purpose}
		b.answers.chosen(prompt.Choice, func(option rules.Option) { resolved.Options = append(resolved.Options, option) })
		out[prompt.Choice.Prompt] = resolved
	}
	return out, nil
}

func (b *promptBuilder) blockedIn(choice rules.Choice) []rules.Slug {
	var blocked []rules.Slug
	for _, requirement := range b.cat.Mechanics.ChoiceRequirements {
		if requirement.Prompt != choice.Prompt {
			continue
		}
		allowed := false
		for _, proficiency := range requirement.AnyProficiency {
			if b.holds(rules.NewRef(rules.RefProficiency, proficiency)) {
				allowed = true
			}
		}
		if !allowed {
			blocked = append(blocked, requirement.Pick)
		}
	}
	var walk func(rules.Option)
	walk = func(option rules.Option) {
		switch opt := option.(type) {
		case rules.RefOption:
			if opt.Ref.Kind == rules.RefFeature && !b.meets(opt.Ref.Slug) {
				blocked = append(blocked, rules.OptionKey(option))
			}
		case rules.NestedOption:
			blocked = append(blocked, b.blockedIn(opt.Choice)...)
		case rules.BundleOption:
			for _, item := range opt.Items {
				walk(item)
			}
		}
	}
	for _, option := range choice.From.Options {
		walk(option)
	}
	return blocked
}

// meets reports whether the character satisfies an offered feature's own
// prerequisites: Thirsting Blade's fifth level and Pact of the Blade.
//
// A level is read against the character as they stand now, not against the
// level whose prompt is offering the feature. That is what lets a twelfth-level
// warlock put Lifedrinker into the pick second level opened, and it is why
// there is no "replace an invocation" step: every earlier answer is editable,
// and what is legal in it is what is legal for the character today -- the same
// policy the spell prompts follow.
//
// The level is the one in the feature's own class when it names one, and a
// character with no level in that class meets nothing: Eldritch Adept gives a
// fighter an invocation, but only one without a prerequisite.
func (b *promptBuilder) meets(slug rules.Slug) bool {
	feature, ok := b.cat.Features.Get(slug)
	if !ok || len(feature.Prerequisites) == 0 {
		return true
	}
	level := b.state.Identity.Level()
	if !feature.Class.IsZero() {
		if level = ownerLevel(b.state, b.cat, rules.NewRef(rules.RefClass, feature.Class)); level == 0 {
			return false
		}
	}
	spells := b.state.Spells
	for _, p := range feature.Prerequisites {
		switch p.Kind {
		case catalog.PrerequisiteLevel:
			if level < p.Level {
				return false
			}
		case catalog.PrerequisiteAbility:
			if b.state.Abilities.Scores[p.Ability] < p.MinimumScore {
				return false
			}
		case catalog.PrerequisiteEntry:
			// Not through holds: that answers "would a second one be a
			// duplicate", and a spell held twice is not one.
			if p.Ref.Kind == rules.RefSpell {
				if !slices.Contains(spells.Cantrips, p.Ref.Slug) && !slices.Contains(spells.Known, p.Ref.Slug) && !slices.Contains(spells.Prepared, p.Ref.Slug) {
					return false
				}
			} else if !b.holds(p.Ref) {
				return false
			}
		}
	}
	return true
}

// ResolvedChoice carries the semantics of a now-closed question alongside its picks.
type ResolvedChoice struct {
	Source  rules.Ref
	Kind    rules.ChoiceKind
	Purpose string
	Options []rules.Option
}
