package character

import (
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

// MaxCharacterLevel is where advancement stops in the 2014 rules.
const MaxCharacterLevel = 20

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

// ResolvedChoice carries the semantics of a now-closed question alongside its picks.
type ResolvedChoice struct {
	Source  rules.Ref
	Kind    rules.ChoiceKind
	Purpose string
	Options []rules.Option
}
