// Package character serves the character resource.
//
// One exported handler per file, named after the action, with its request and
// response types beside it. The shapes shared by more than one handler -- the
// sheet, an event, a prompt -- live here.
package character

import (
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
)

// Summary is one row of a character listing.
type Summary struct {
	ID string `json:"id"`

	// Folder is where the character is filed. Always set: a character is
	// never in no folder, so a client can group a listing by this without
	// a fallback bucket.
	Folder  string       `json:"folder"`
	Name    string       `json:"name"`
	Image   string       `json:"image,omitempty"`
	Level   int          `json:"level"`
	Classes []ClassLevel `json:"classes,omitempty"`
}

// ClassLevel is how many levels a character has in one class.
type ClassLevel struct {
	Class    string `json:"class"`
	Subclass string `json:"subclass,omitempty"`
	Level    int    `json:"level"`
}

// Character is a character and its log.
type Character struct {
	Revision    int               `json:"revision"`
	Rules       helpers.RulesLock `json:"rules"`
	Checkpoints []Checkpoint      `json:"checkpoints,omitempty"`
	ID          string            `json:"id"`

	// Seq is the sequence the log currently ends at. It is the token an
	// append or a truncation must state, so it is returned everywhere a
	// client might be about to write.
	Seq    int     `json:"seq"`
	Events []Event `json:"events"`
}

// Event is one entry in a character's log.
type Event struct {
	Evidence      string              `json:"evidence,omitempty"`
	Observed      bool                `json:"observed,omitempty"`
	ChoiceSource  string              `json:"choiceSource,omitempty"`
	ChoiceKind    string              `json:"choiceKind,omitempty"`
	Purpose       string              `json:"purpose,omitempty"`
	Selections    []catalogapi.Option `json:"selections,omitempty"`
	ID            string              `json:"id,omitempty"`
	SchemaVersion int                 `json:"schemaVersion,omitempty"`
	Resource      string              `json:"resource,omitempty"`
	Amount        int                 `json:"amount,omitempty"`
	Trigger       string              `json:"trigger,omitempty"`
	Allocations   map[string]int      `json:"allocations,omitempty"`
	Seq           int                 `json:"seq,omitempty"`
	Type          string              `json:"type"`

	// Source is the group of the prompt this entry answers: identity, class,
	// race, background, abilities or advance. It is what lets a client group
	// the log by the question each entry settles rather than guessing from
	// the type -- a guess with no answer for a change event carrying six
	// ability scores.
	//
	// Written by the server and **ignored on the way in**. The client does
	// not decide what an answer means; a client-supplied source would be a
	// second vocabulary for the same fact, free to disagree with the rules.
	// Empty where the server cannot attribute the entry: an imported log, a
	// DM's change.
	Source string `json:"source,omitempty"`

	At string `json:"at,omitempty"`

	// Ref names the catalogue entry this event selects, as "kind:slug".
	Ref string `json:"ref,omitempty"`

	Level   int      `json:"level,omitempty"`
	Choices []Answer `json:"choices,omitempty"`
	Changes []Change `json:"changes,omitempty"`
	Note    string   `json:"note,omitempty"`
}

// Answer is the player's response to one prompt.
type Answer struct {
	Prompt string   `json:"prompt"`
	Picks  []string `json:"picks"`
}

// Change is one addressed mutation of the sheet.
type Change struct {
	Path  string `json:"path"`
	Op    string `json:"op"`
	Value Value  `json:"value"`
}

// Value is the payload of a change.
//
// Exactly one field is meaningful, named by Kind. It is a tagged shape rather
// than a bare JSON value because the domain's own Value is tagged: an
// untagged number could not say whether "3" is a score or a slug.
type Value struct {
	Kind   string   `json:"kind"`
	Int    int      `json:"int,omitempty"`
	String string   `json:"string,omitempty"`
	Bool   bool     `json:"bool,omitempty"`
	Slug   string   `json:"slug,omitempty"`
	Slugs  []string `json:"slugs,omitempty"`
	Dice   string   `json:"dice,omitempty"`
}

// Sheet is the projected character.
type Sheet struct {
	CustomOptions []CustomOption    `json:"customOptions,omitempty"`
	ImportSession string            `json:"importSession,omitempty"`
	CatalogNames  map[string]string `json:"catalogNames,omitempty"`
	// Catalog is what the sheet's slugs mean, sent with them. Only the two
	// sheet reads fill it; a write's echo of the sheet does not.
	Catalog       *catalogapi.Resolved   `json:"catalog,omitempty"`
	ImportedNotes []string               `json:"importedNotes,omitempty"`
	Contributions []Contribution         `json:"contributions,omitempty"`
	PackActions   []ActionOffer          `json:"packActions,omitempty"`
	ManualRules   []string               `json:"manualRules,omitempty"`
	Identity      Identity               `json:"identity"`
	Base          Base                   `json:"base"`
	Abilities     Abilities              `json:"abilities"`
	Skills        map[string]Skill       `json:"skills"`
	SavingThrows  map[string]SavingThrow `json:"savingThrows"`
	Status        Status                 `json:"status"`
	Equipment     Equipment              `json:"equipment"`
	Resources     Resources              `json:"resources"`
	Spells        Spellbook              `json:"spells"`
	Actions       []Action               `json:"actions"`
	Feats         []string               `json:"feats,omitempty"`
	Traits        []string               `json:"traits,omitempty"`
	Features      []string               `json:"features,omitempty"`
	Conditions    []string               `json:"conditions,omitempty"`

	// Proficiencies are the armor, weapon and tool proficiencies. Skills and
	// saving throws have their own typed homes above, where a bonus is
	// computed from them; what is left is the sheet's "other proficiencies"
	// box, which is a list and nothing more.
	Proficiencies []string `json:"proficiencies,omitempty"`
}

// Identity is who the character is.
type Identity struct {
	Name              string       `json:"name"`
	Image             string       `json:"image,omitempty"`
	Alignment         string       `json:"alignment,omitempty"`
	Race              string       `json:"race,omitempty"`
	Subrace           string       `json:"subrace,omitempty"`
	Background        string       `json:"background,omitempty"`
	Classes           []ClassLevel `json:"classes,omitempty"`
	Level             int          `json:"level"`
	DesiredLevel      int          `json:"desiredLevel,omitempty"`
	Ruleset           string       `json:"ruleset,omitempty"`
	Experience        int          `json:"experience"`
	PersonalityTraits []string     `json:"personalityTraits,omitempty"`
	Ideals            []string     `json:"ideals,omitempty"`
	Bonds             []string     `json:"bonds,omitempty"`
	Flaws             []string     `json:"flaws,omitempty"`
}

// HitPoints tracks health.
type HitPoints struct {
	Current   int `json:"current"`
	Max       int `json:"max"`
	Temporary int `json:"temporary,omitempty"`
}

// Speed is one movement mode and its rate.
type Speed struct {
	Kind     string `json:"kind"`
	Distance int    `json:"distance"`
}

// Sense is one special sense and its range.
type Sense struct {
	Kind     string `json:"kind"`
	Distance int    `json:"distance"`
}

// DeathSaves is the three-and-three tally rolled while dying.
type DeathSaves struct {
	Successes int `json:"successes"`
	Failures  int `json:"failures"`
}

// Base is the character's fundamental physical state.
type Base struct {
	HitPoints   HitPoints  `json:"hitPoints"`
	Speeds      []Speed    `json:"speeds,omitempty"`
	Senses      []Sense    `json:"senses,omitempty"`
	Size        string     `json:"size,omitempty"`
	Languages   []string   `json:"languages,omitempty"`
	Exhaustion  int        `json:"exhaustion,omitempty"`
	DeathSaves  DeathSaves `json:"deathSaves"`
	Inspiration bool       `json:"inspiration,omitempty"`
}

// Abilities holds the six scores and their modifiers.
//
// Modifiers are served though the domain never stores them, because every
// client would otherwise reimplement floor((score-10)/2) and one of them
// would get negative scores wrong.
type Abilities struct {
	Scores    map[string]int `json:"scores"`
	Modifiers map[string]int `json:"modifiers"`
	Method    string         `json:"method,omitempty"`
}

// Skill is the character's training in one skill and the resulting bonus.
type Skill struct {
	Proficiency string `json:"proficiency"`
	Bonus       int    `json:"bonus"`
}

// SavingThrow is the character's training in one save.
type SavingThrow struct {
	Proficient bool `json:"proficient"`
	Bonus      int  `json:"bonus"`
}

// Spellcasting is the at-a-glance casting block for one class.
type Spellcasting struct {
	Class       string `json:"class"`
	Ability     string `json:"ability"`
	SaveDC      int    `json:"saveDC"`
	AttackBonus int    `json:"attackBonus"`
}

// Status is the headline block: the numbers a player reads off constantly.
type Status struct {
	ArmorClass        int            `json:"armorClass"`
	Initiative        int            `json:"initiative"`
	ProficiencyBonus  int            `json:"proficiencyBonus"`
	PassivePerception int            `json:"passivePerception"`
	Spellcasting      []Spellcasting `json:"spellcasting,omitempty"`
}

// CustomItem is a homebrew or DM-granted item with no catalogue entry.
type CustomItem struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Weight      float64 `json:"weight,omitempty"`
}

// ItemStack is a quantity of one item.
type ItemStack struct {
	Item   string      `json:"item,omitempty"`
	Count  int         `json:"count"`
	Custom *CustomItem `json:"custom,omitempty"`
}

// Equipment is everything the character carries.
type Equipment struct {
	Equipped []ItemStack    `json:"equipped"`
	Backpack []ItemStack    `json:"backpack"`
	Loot     []ItemStack    `json:"loot"`
	Purse    map[string]int `json:"purse,omitempty"`
}

// Pool is a consumable resource.
type Pool struct {
	Key      string `json:"key,omitempty"`
	Max      int    `json:"max"`
	Used     int    `json:"used,omitempty"`
	Recharge string `json:"recharge,omitempty"`
	Dice     string `json:"dice,omitempty"`
}

// Resources is everything the character spends and regains.
type Resources struct {
	Pools      map[string]ResourcePool      `json:"pools,omitempty"`
	Parameters map[string]ResourceParameter `json:"parameters,omitempty"`
	// SpellSlots is keyed by spell level. Pact Magic is deliberately not
	// here: it is a separate pool at overlapping levels that recovers on a
	// short rest, so it lives among the class resources.
	SpellSlots map[string]Pool `json:"spellSlots,omitempty"`
	HitDice    []Pool          `json:"hitDice,omitempty"`
	Class      []Pool          `json:"class,omitempty"`
}

// Spellbook is what the character knows and has ready.
type SpellSource struct {
	Source           string   `json:"source"`
	Class            string   `json:"class,omitempty"`
	Ability          string   `json:"ability,omitempty"`
	Cantrips         []string `json:"cantrips,omitempty"`
	Known            []string `json:"known,omitempty"`
	Spellbook        []string `json:"spellbook,omitempty"`
	Prepared         []string `json:"prepared,omitempty"`
	Arcanum          []string `json:"arcanum,omitempty"`
	Mastery          []string `json:"mastery,omitempty"`
	PreparationLimit int      `json:"preparationLimit,omitempty"`
}

type Spellbook struct {
	Sources  []SpellSource `json:"sources,omitempty"`
	Cantrips []string      `json:"cantrips,omitempty"`
	Known    []string      `json:"known,omitempty"`
	Prepared []string      `json:"prepared,omitempty"`
	Ability  string        `json:"ability,omitempty"`
}

// Action is something the character can do on their turn.
type Action struct {
	Source string `json:"source"`
	Origin string `json:"origin,omitempty"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Range  int    `json:"range,omitempty"`
	ToHit  *int   `json:"toHit,omitempty"`
	Damage string `json:"damage,omitempty"`
	Uses   string `json:"uses,omitempty"`
	Notes  string `json:"notes,omitempty"`
}

type ResourcePool struct {
	ID         string   `json:"id"`
	Definition string   `json:"definition"`
	Owner      string   `json:"owner"`
	Name       string   `json:"name"`
	Group      string   `json:"group,omitempty"`
	Max        int      `json:"max"`
	Used       int      `json:"used"`
	Available  int      `json:"available"`
	Dice       string   `json:"dice,omitempty"`
	SlotLevel  int      `json:"slotLevel,omitempty"`
	Recovery   []string `json:"recovery,omitempty"`
}
type Rational struct {
	Numerator   int `json:"numerator"`
	Denominator int `json:"denominator"`
}
type ResourceParameter struct {
	Rational *Rational `json:"rational,omitempty"`
	Boolean  *bool     `json:"boolean,omitempty"`
	Name     string    `json:"name"`
	Number   int       `json:"number"`
	Dice     string    `json:"dice,omitempty"`
	Text     string    `json:"text,omitempty"`
}
type Checkpoint struct {
	Index    int               `json:"index"`
	Revision int               `json:"revision"`
	Reason   string            `json:"reason"`
	Rules    helpers.RulesLock `json:"rules"`
}

type Contribution struct {
	EventID string `json:"eventId,omitempty"`
	Rule    string `json:"rule"`
	Owner   string `json:"owner"`
	Target  string `json:"target"`
	Amount  int    `json:"amount"`
}
type ActionOffer struct {
	ID        string         `json:"id"`
	Owner     string         `json:"owner"`
	Name      string         `json:"name"`
	Manual    bool           `json:"manual"`
	Available bool           `json:"available"`
	Costs     map[string]int `json:"costs"`
}
