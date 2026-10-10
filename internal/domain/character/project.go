package character

import (
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// Project folds a log into a State against a catalogue.
//
// It is pure: the same log and the same catalogue always produce the same
// state, with no clock, no randomness and no I/O. That is what makes the
// event-sourced design worth its cost -- a character can be re-derived at any
// time, and a projection bug is fixed by shipping new code rather than by
// migrating stored data.
//
// The order of derivation matters and is fixed:
//
//  1. replay events in sequence, accumulating chosen entries and raw scores
//  2. resolve the catalogue entries those choices name
//  3. compute the derived values -- proficiency bonus, then ability
//     modifiers, then skills and saving throws, then armor class, initiative
//     and the spellcasting summaries
//  4. assemble actions, deriving from equipment, tagged entries and the
//     pack's standalone actions
//
// Step 3 is ordered because each stage feeds the next: proficiency bonus
// depends on character level, saving throws depend on the proficiency bonus,
// and the spell save DC depends on both.
//
// Step 4 happens after the build, in deriveActions: an equipped weapon gives
// its attack, an entry the pack tagged gives its action, and the pack's
// standalone actions give the rest. A casting from a prepared spell is not
// derived; spells are listed on their own.
//
// # Base scores versus final scores
//
// An init event records the scores as *generated* -- the point buy, the
// standard array, the dice. Racial bonuses and Ability Score Improvements are
// applied here, every time. That is the only arrangement under which choosing
// a different race changes the sheet, which is the entire point of projecting
// rather than storing; the alternative would make the player re-enter six
// numbers whenever they went back a step.
func projectBuild(log Log, cat *catalog.Catalog) (State, error) {
	if cat == nil {
		return State{}, types.NewValidationError("projecting against a nil catalogue")
	}
	if lock := log.RulesLock(); !lock.IsZero() && !lock.Equal(cat.Lock) {
		return State{}, types.NewValidationError("character rules lock does not match catalogue")
	}
	if err := log.Validate(); err != nil {
		return State{}, err
	}

	cat = WithCustomCatalog(log, cat)
	p := projector{cat: cat, answers: foldAnswers(log)}
	p.state.Abilities.ModifierRule = cat.Mechanics.Core.AbilityModifier
	return p.run(log)
}

// projector carries the working state of one projection.
type projector struct {
	finalAbilities map[rules.Ability]int
	cat            *catalog.Catalog
	answers        answers

	state State

	// Changes fall into three tiers, and the tier decides when they are
	// applied. Getting this wrong is silent: the change lands, and the number
	// it was meant to affect was computed before or after it.
	//
	//   inputs    seed derivation -- ability scores, name, alignment. They
	//             must land before anything reads them, and the hit point
	//             maximum reads Constitution.
	//   equipment moves items between the carried lists. It lands after the
	//             starting kit has been granted, because you cannot equip
	//             armor you have not been given, and before armor class is
	//             derived, because armor class reads what is equipped.
	//   overrides are everything else, applied last so that a DM's ruling
	//             wins over the rules rather than being recomputed away.
	inputs    []seqChange
	equipment []seqChange
	overrides []seqChange

	// proficiencies is every proficiency slug granted, in grant order,
	// before it is sorted into skills, saving throws and the rest.
	proficiencies []rules.Slug

	// expertise is the skills doubled by a feature.
	expertise   []rules.Slug
	err         error
	statEffects []pendingEffect
}

// seqChange is a change together with the event that carried it, so an error
// can say where a bad path came from.
type seqChange struct {
	Seq    int
	Change Change
}

func (p *projector) run(log Log) (State, error) {
	p.replay(log)

	if err := p.applyChanges(p.inputs); err != nil {
		return State{}, err
	}
	// The declared level comes first, because it *is* the character's level
	// and almost everything below is per-level. Ability scores are the
	// non-obvious one: deriveAbilities applies the improvement every fourth
	// level grants, so a character who declared 4th level but had not been
	// raised to it yet had their improvement silently do nothing.
	// A source may describe a subclass before its parent class is imported.
	for _, custom := range CustomOptions(log) {
		if !custom.Selected {
			continue
		}
		if custom.Kind == "subclass" {
			for _, taken := range p.state.Identity.Classes {
				if taken.Class.String() == custom.Parent && taken.Subclass.IsZero() {
					p.customStructure(custom)
				}
			}
		}
		if custom.Kind == "subrace" && p.state.Identity.Subrace.IsZero() && p.state.Identity.Race.String() == custom.Parent {
			p.customStructure(custom)
		}
	}
	p.advanceToDesiredLevel()

	// Ability scores are finalised before anything that reads them. The hit
	// point maximum is the reason this cannot wait: it is Constitution
	// modifier per level, and a half-elf who put their +1 into Constitution
	// would otherwise be three hit points short at 3rd level.
	p.deriveAbilities()
	for ability, score := range p.finalAbilities {
		p.state.Abilities.Scores[ability] = score
	}

	p.applyRace()
	p.applyBackground()
	p.applyClasses()
	// The bonus is the pack's to state: a loaded pack always carries the
	// expression, so there is no table here for it to disagree with.
	bonus, err := p.cat.Mechanics.Core.Proficiency.Eval(rules.Variables{"level": p.state.Identity.Level()})
	if err != nil {
		return State{}, err
	}
	p.state.Status.ProficiencyBonus = bonus
	// The starting kit comes first, into the backpack: the sheet's own
	// equipment writes are counts of what it granted.
	p.applyEquipmentChoices()
	// Equipped items are explicit inputs to pack conditions as well as AC.
	// Apply that independent list before rules; carried-item changes still
	// follow rule grants.
	var carriedChanges []seqChange
	for _, change := range p.equipment {
		if change.Change.Path == "equipment.equipped" {
			if err := p.applyChanges([]seqChange{change}); err != nil {
				return State{}, err
			}
		} else {
			carriedChanges = append(carriedChanges, change)
		}
	}
	p.equipment = carriedChanges
	if err := p.applyPackRules(); err != nil {
		return State{}, err
	}
	// Recompute HP from final modifiers, including per-level minimums.
	p.state.Base.HitPoints.Max = 0
	for i, taken := range p.state.Identity.Classes {
		if class, ok := p.cat.Classes.Get(taken.Class); ok {
			p.addHitPoints(class.HitDie, taken.Level, i == 0)
		}
	}
	p.seedCustomItems(log)
	if err := p.applyChanges(p.equipment); err != nil {
		return State{}, err
	}
	// The whole-list equipped writes ran before the rules and the Custom slot
	// writes after, so a slot set between two list writes is checked only now,
	// against the list as it finally stands.
	p.clearCustomIfBare()

	p.deriveProficiencies()
	p.deriveStatus()
	p.applySpells()
	for _, e := range p.statEffects {
		if err := p.applyEffect(e.rule, e.effect); err != nil {
			return State{}, err
		}
	}
	p.state.Base.HitPoints.Current = p.state.Base.HitPoints.Max
	if !p.cat.Lock.IsZero() {
		if err := p.resourceDefinitions(); err != nil {
			return State{}, err
		}
	}

	if err := p.applyChanges(p.overrides); err != nil {
		return State{}, err
	}
	for i := range p.state.Contributions {
		c := &p.state.Contributions[i]
		for _, e := range log.Events {
			if e.Ref == c.Owner {
				c.EventID = e.ID
			}
		}
	}
	p.privateNames()
	p.customDetails(log)
	return p.state, p.err
}

// replay walks the log in order, recording what was chosen. Nothing is
// derived here: this stage only says which catalogue entries the character
// named and which changes were requested.
func (p *projector) replay(log Log) {
	custom := CustomOptions(log)
	for _, e := range log.Events {
		if e.Custom != nil {
			// Only the latest definition applies, at its original selection position.
			for i, c := range custom {
				if c.ID == e.Custom.ID {
					p.customStructure(c)
					custom = append(custom[:i], custom[i+1:]...)
					break
				}
			}
		}
		switch e.Type {
		case EventInit, EventChange:
			for _, ch := range e.Changes {
				sc := seqChange{Seq: e.Seq, Change: ch}
				switch {
				case isInputPath(ch.Path) || strings.HasPrefix(string(ch.Path), "finalAbilities."):
					p.inputs = append(p.inputs, sc)
				case isEquipmentPath(ch.Path):
					p.equipment = append(p.equipment, sc)
				default:
					p.overrides = append(p.overrides, sc)
				}
			}
		case EventRace:
			p.state.Identity.Race = e.Ref.Slug
		case EventSubrace:
			p.state.Identity.Subrace = e.Ref.Slug
		case EventBackground:
			p.state.Identity.Background = e.Ref.Slug
		case EventClass:
			p.takeLevel(e.Ref.Slug, max(e.Level, 1))
		case EventLevel:
			p.takeLevel(e.Ref.Slug, e.Level)
		case EventSubclass:
			p.setSubclass(e.Ref.Slug)
		case EventFeat:
			if !e.Ref.Slug.IsZero() && !slices.Contains(p.state.Feats, e.Ref.Slug) {
				p.state.Feats = append(p.state.Feats, e.Ref.Slug)
			}
		case EventNote, EventNone:
			if strings.HasPrefix(e.Note, "import.session:") {
				p.state.ImportSession = strings.TrimPrefix(e.Note, "import.session:")
			}
			if strings.HasPrefix(e.Note, "import.manual:") {
				_, body, _ := strings.Cut(e.Note, "\n")
				p.state.ImportedNotes = append(p.state.ImportedNotes, body)
			}
		}
	}
}

// takeLevel records that the character now has a level in a class. Levels are
// idempotent by number rather than cumulative, so replaying a log twice
// cannot inflate them.
func (p *projector) takeLevel(class rules.Slug, level int) {
	if class.IsZero() || level < 1 {
		return
	}
	for i, c := range p.state.Identity.Classes {
		if c.Class == class {
			if level > c.Level {
				p.state.Identity.Classes[i].Level = level
			}
			return
		}
	}
	p.state.Identity.Classes = append(p.state.Identity.Classes, ClassLevel{Class: class, Level: level})
}

// advanceToDesiredLevel raises a single-class character to the level they
// declared they are building towards.
//
// Levels used to be taken one event at a time, each one answering "which class
// does this go into?" -- a question with one answer while multiclassing is
// off, asked eight times on the way to ninth level. There is nothing to
// record, so nothing records it: the declaration *is* the level, and
// applyClasses grants what that level grants. What the player is actually
// asked is what those levels open -- the archetype, the improvements -- which
// Prompts derives from the same number.
//
// Raise only. Going back down is not something the rules do, and takeLevel is
// already max-by-number for the same reason.
//
// With two classes this does nothing, because then the question is real again.
// That is the whole of what turning multiclassing back on has to undo.
func (p *projector) advanceToDesiredLevel() {
	if len(p.state.Identity.Classes) != 1 {
		return
	}
	if p.state.Identity.DesiredLevel > p.state.Identity.Classes[0].Level {
		p.state.Identity.Classes[0].Level = p.state.Identity.DesiredLevel
	}
}

// setSubclass attaches a subclass to whichever class offers it.
func (p *projector) setSubclass(subclass rules.Slug) {
	entry, ok := p.cat.Subclasses.Get(subclass)
	if !ok {
		return
	}
	for i, c := range p.state.Identity.Classes {
		if c.Class == entry.Class {
			p.state.Identity.Classes[i].Subclass = subclass
			return
		}
	}
}

// isInputPath reports whether a change seeds derivation rather than
// overriding it.
//
// The split is what makes both kinds of change work. Ability scores and the
// character's name are inputs: nothing derives them, and everything else
// derives *from* them, so they must land before the rules run. A hit point
// maximum set by a DM is an override: the rules would otherwise recompute it
// and the ruling would vanish.
func isInputPath(path Path) bool {
	switch path {
	case "identity.image", "identity.name", "identity.alignment", "identity.desiredLevel",
		"identity.ruleset", "abilities.method":
		return true
	}
	segments := path.Segments()
	return len(segments) == 2 && segments[0] == "abilities"
}

// isEquipmentPath reports whether a change moves items between the carried
// lists, which has to happen between the starting kit being granted and armor
// class being derived from what is worn.
//
// A stack set by count -- equipment.equipped.leather-armor -- is as much a
// move as a list is, and has to land in the same place: left to the override
// tier it arrives after armor class was derived, and the armor an imported
// character is wearing protects nobody. The purse is not carried in a list
// and stays an ordinary override.
func isEquipmentPath(path Path) bool {
	segments := path.Segments()
	return len(segments) >= 2 && segments[0] == "equipment" && segments[1] != "purse"
}

// Private definitions travel in the locked projection, including shared and
// copied sheets. The global compendium intentionally cannot resolve them.
func (p *projector) privateNames() {
	private := false
	for _, release := range p.cat.Lock.Packs {
		if strings.HasPrefix(release.ID, "import-") {
			private = true
			break
		}
	}
	if !private {
		return
	}
	p.state.CatalogNames = map[string]string{}
	add := func(collection string, entry catalog.Entry) {
		if strings.HasPrefix(entry.Slug.String(), "import-") {
			p.state.CatalogNames[collection+":"+entry.Slug.String()] = entry.Name
			body := entry.Name
			if len(entry.Desc) > 0 {
				body += "\n" + strings.Join(entry.Desc, "\n\n")
			}
			p.state.ImportedNotes = append(p.state.ImportedNotes, body)
		}
	}
	for _, v := range p.cat.Spells.All() {
		add("spells", v.Entry)
	}
	for _, v := range p.cat.Items.All() {
		add("equipment", v.Entry)
	}
	for _, v := range p.cat.MagicItems.All() {
		add("equipment", v.Entry)
	}
	for _, v := range p.cat.Races.All() {
		add("races", v.Entry)
	}
	for _, v := range p.cat.Subraces.All() {
		add("subraces", v.Entry)
	}
	for _, v := range p.cat.Classes.All() {
		add("classes", v.Entry)
	}
	for _, v := range p.cat.Subclasses.All() {
		add("subclasses", v.Entry)
	}
	for _, v := range p.cat.Backgrounds.All() {
		add("backgrounds", v.Entry)
	}
	for _, v := range p.cat.Feats.All() {
		add("feats", v.Entry)
	}
	for _, v := range p.cat.Features.All() {
		add("features", v.Entry)
	}
	for _, v := range p.cat.Traits.All() {
		add("traits", v.Entry)
	}
	for _, v := range p.cat.Languages.All() {
		add("languages", v.Entry)
	}
}
