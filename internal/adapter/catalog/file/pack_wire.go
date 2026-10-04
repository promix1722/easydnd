package file

import (
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// PackManifest is shared by directory and single-file representations.
type PackManifest struct {
	Title         string            `json:"title,omitempty"`
	Sources       map[string]string `json:"sources,omitempty"`
	SchemaVersion int               `json:"schemaVersion"`
	ID            string            `json:"id"`
	Version       string            `json:"version"`
	Edition       string            `json:"edition"`
	Semantics     string            `json:"semantics"`
	Requires      []string          `json:"requires,omitempty"`
	Dependencies  []Dependency      `json:"dependencies,omitempty"`
	DefaultLocale string            `json:"defaultLocale"`
	Source        string            `json:"source,omitempty"`
	Attribution   string            `json:"attribution,omitempty"`
	Files         map[string]string `json:"files,omitempty"`
}
type Dependency struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}
type LockedRelease struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}
type RulesLock struct {
	Edition   string          `json:"edition"`
	Semantics string          `json:"semantics"`
	Packs     []LockedRelease `json:"packs"`
}

func LockOf(l pack.Lock) RulesLock {
	out := RulesLock{Edition: l.Edition, Semantics: l.Semantics}
	for _, p := range l.Packs {
		out.Packs = append(out.Packs, LockedRelease{p.ID, p.Version, p.Digest})
	}
	return out
}
func (l RulesLock) Domain() pack.Lock {
	out := pack.Lock{Edition: l.Edition, Semantics: l.Semantics}
	for _, p := range l.Packs {
		out.Packs = append(out.Packs, pack.Release{ID: p.ID, Version: p.Version, Digest: p.Digest})
	}
	return out
}

type Expression struct {
	Op    string       `json:"op"`
	Value int          `json:"value,omitempty"`
	Ref   string       `json:"ref,omitempty"`
	Args  []Expression `json:"args,omitempty"`
}

func (e Expression) domain() rules.Expression {
	out := rules.Expression{Op: e.Op, Value: e.Value, Ref: e.Ref}
	for _, a := range e.Args {
		out.Args = append(out.Args, a.domain())
	}
	return out
}

type Rational struct {
	Numerator   int `json:"numerator"`
	Denominator int `json:"denominator"`
}

type ResourceRow struct {
	Rational  *Rational `json:"rational,omitempty"`
	Boolean   *bool     `json:"boolean,omitempty"`
	From      int       `json:"from"`
	Capacity  int       `json:"capacity"`
	Dice      string    `json:"dice,omitempty"`
	SlotLevel int       `json:"slotLevel,omitempty"`
	Text      string    `json:"text,omitempty"`
}
type RecoveryPolicy struct {
	When      *Expression `json:"when,omitempty"`
	Trigger   string      `json:"trigger"`
	Operation string      `json:"operation"`
	Amount    Expression  `json:"amount,omitempty"`
	Budget    string      `json:"budget,omitempty"`
}
type ResourceDefinition struct {
	RequiresSubclass bool             `json:"requiresSubclass,omitempty"`
	ID               string           `json:"id"`
	Owner            Ref              `json:"owner"`
	MinimumLevel     int              `json:"minimumLevel"`
	Kind             string           `json:"kind"`
	Input            string           `json:"input"`
	Rows             []ResourceRow    `json:"rows,omitempty"`
	Capacity         *Expression      `json:"capacity,omitempty"`
	Recovery         []RecoveryPolicy `json:"recovery,omitempty"`
	SharedKey        string           `json:"sharedKey,omitempty"`
	Combine          string           `json:"combine,omitempty"`
	Group            string           `json:"group,omitempty"`
}
type RuleDefinition struct {
	ID           string      `json:"id"`
	Owner        Ref         `json:"owner"`
	MinimumLevel int         `json:"minimumLevel,omitempty"`
	When         *Expression `json:"when,omitempty"`
	Choices      []Choice    `json:"choices,omitempty"`
	Effects      []Effect    `json:"effects,omitempty"`
	Manual       bool        `json:"manual,omitempty"`
}
type Effect struct {
	Op     string     `json:"op"`
	Target string     `json:"target,omitempty"`
	Ref    Ref        `json:"ref,omitempty"`
	Value  Expression `json:"value,omitempty"`
}
type CastingProfile struct {
	Selection        string `json:"selection,omitempty"`
	PrepareDivisor   int    `json:"prepareDivisor,omitempty"`
	BookStart        int    `json:"bookStart,omitempty"`
	BookPerLevel     int    `json:"bookPerLevel,omitempty"`
	ReplaceKnown     bool   `json:"replaceKnown,omitempty"`
	ExpandedSubclass bool   `json:"expandedSubclass,omitempty"`

	Kind        string `json:"kind"`
	Numerator   int    `json:"numerator"`
	Denominator int    `json:"denominator"`
	Rounding    string `json:"rounding"`
	StartsAt    int    `json:"startsAt"`
	Resource    string `json:"resource,omitempty"`
}
type CoreRules struct {
	MinScore             int                   `json:"minScore"`
	MaxScore             int                   `json:"maxScore"`
	AbilityScoreIncrease int                   `json:"abilityScoreIncrease"`
	BaseArmorClass       int                   `json:"baseArmorClass"`
	SpellSaveBase        int                   `json:"spellSaveBase"`
	HitPointFirst        Expression            `json:"hitPointFirst"`
	HitPointLater        Expression            `json:"hitPointLater"`
	HitDiceRecovery      []RecoveryPolicy      `json:"hitDiceRecovery"`
	SpellSlotRecovery    []RecoveryPolicy      `json:"spellSlotRecovery"`
	MaxLevel             int                   `json:"maxLevel"`
	Proficiency          Expression            `json:"proficiency"`
	AbilityModifier      Expression            `json:"abilityModifier"`
	StandardArray        []int                 `json:"standardArray"`
	PointBuyBudget       int                   `json:"pointBuyBudget"`
	PointCosts           map[int]int           `json:"pointCosts"`
	MulticlassSlots      map[int]map[int]int   `json:"multiclassSlots"`
	Senses               map[string]SenseGrant `json:"senses"`
}
type SenseGrant struct {
	Kind     string `json:"kind"`
	Distance int    `json:"distance"`
}
type ActionDefinition struct {
	ID           string         `json:"id"`
	Owner        Ref            `json:"owner"`
	MinimumLevel int            `json:"minimumLevel,omitempty"`
	When         *Expression    `json:"when,omitempty"`
	Costs        []ResourceCost `json:"costs"`
	Manual       bool           `json:"manual"`
}
type ResourceCost struct {
	Resource string     `json:"resource"`
	Amount   Expression `json:"amount"`
}
type SpellBenefit struct {
	Ability     string   `json:"ability,omitempty"`
	ID          string   `json:"id"`
	Owner       Ref      `json:"owner"`
	Class       string   `json:"class,omitempty"`
	Level       int      `json:"level"`
	Count       int      `json:"count,omitempty"`
	SpellLevel  int      `json:"spellLevel"`
	Mode        string   `json:"mode"`
	From        string   `json:"from,omitempty"`
	Spells      []string `json:"spells,omitempty"`
	CountsKnown bool     `json:"countsKnown,omitempty"`
}

type ChoiceRequirement struct {
	Prompt         string   `json:"prompt"`
	Pick           string   `json:"pick"`
	AnyProficiency []string `json:"anyProficiency"`
}

type PackMechanics struct {
	ChoiceRequirements []ChoiceRequirement `json:"choiceRequirements,omitempty"`
	SpellBenefits      []SpellBenefit      `json:"spellBenefits,omitempty"`

	Actions   []ActionDefinition        `json:"actions,omitempty"`
	Overrides []Override                `json:"overrides,omitempty"`
	Core      *CoreRules                `json:"core,omitempty"`
	Resources []ResourceDefinition      `json:"resources,omitempty"`
	Rules     []RuleDefinition          `json:"rules,omitempty"`
	Casting   map[string]CastingProfile `json:"casting,omitempty"`
}

func (w PackMechanics) domain(prose, actionProse Bundle) (catalog.Mechanics, error) {
	c := &conv{where: "pack mechanics"}
	out := catalog.Mechanics{Casting: map[rules.Slug]catalog.CastingProfile{}}
	if w.Core != nil {
		x := w.Core
		out.Core = catalog.CoreRules{MinScore: x.MinScore, MaxScore: x.MaxScore, AbilityScoreIncrease: x.AbilityScoreIncrease, BaseArmorClass: x.BaseArmorClass, SpellSaveBase: x.SpellSaveBase, HitPointFirst: x.HitPointFirst.domain(), HitPointLater: x.HitPointLater.domain(), HitDiceRecovery: recoveryDomain(x.HitDiceRecovery), SpellSlotRecovery: recoveryDomain(x.SpellSlotRecovery), MaxLevel: x.MaxLevel, Proficiency: x.Proficiency.domain(), AbilityModifier: x.AbilityModifier.domain(), StandardArray: x.StandardArray, PointBuyBudget: x.PointBuyBudget, PointCosts: x.PointCosts, MulticlassSlots: x.MulticlassSlots, Senses: map[rules.Slug]catalog.SenseGrant{}}
		for k, v := range x.Senses {
			out.Core.Senses[rules.Slug(k)] = catalog.SenseGrant{Kind: v.Kind, Distance: v.Distance}
		}
	}
	for k, v := range w.Casting {
		out.Casting[rules.Slug(k)] = catalog.CastingProfile{Selection: v.Selection, PrepareDivisor: v.PrepareDivisor, BookStart: v.BookStart, BookPerLevel: v.BookPerLevel, ReplaceKnown: v.ReplaceKnown, ExpandedSubclass: v.ExpandedSubclass, Kind: v.Kind, Numerator: v.Numerator, Denominator: v.Denominator, Rounding: v.Rounding, StartsAt: v.StartsAt, Resource: rules.Slug(v.Resource)}
	}
	for _, r := range w.ChoiceRequirements {
		out.ChoiceRequirements = append(out.ChoiceRequirements, catalog.ChoiceRequirement{Prompt: rules.Slug(r.Prompt), Pick: rules.Slug(r.Pick), AnyProficiency: slugs(r.AnyProficiency)})
	}
	for _, b := range w.SpellBenefits {
		out.SpellBenefits = append(out.SpellBenefits, catalog.SpellBenefit{Ability: c.ability(b.Ability), ID: rules.Slug(b.ID), Owner: c.ref(b.Owner), Class: rules.Slug(b.Class), Level: b.Level, Count: b.Count, SpellLevel: b.SpellLevel, Mode: b.Mode, From: b.From, Spells: slugs(b.Spells), CountsKnown: b.CountsKnown})
	}
	for _, w := range w.Resources {
		d := catalog.ResourceDefinition{RequiresSubclass: w.RequiresSubclass, Entry: entry(w.ID, prose), Owner: c.ref(w.Owner), MinimumLevel: w.MinimumLevel, Kind: w.Kind, Input: w.Input, SharedKey: rules.Slug(w.SharedKey), Combine: w.Combine, Group: w.Group}
		if w.Capacity != nil {
			e := w.Capacity.domain()
			d.Capacity = &e
		}
		for _, r := range w.Rows {
			d.Rows = append(d.Rows, catalog.ResourceRow{From: r.From, Capacity: r.Capacity, Dice: r.Dice, SlotLevel: r.SlotLevel, Text: r.Text, Rational: rationalDomain(r.Rational), Boolean: r.Boolean})
		}
		d.Recovery = recoveryDomain(w.Recovery)
		out.Resources = append(out.Resources, d)
	}
	for _, w := range w.Rules {
		d := catalog.RuleDefinition{ID: rules.Slug(w.ID), Owner: c.ref(w.Owner), MinimumLevel: w.MinimumLevel, Manual: w.Manual}
		if w.When != nil {
			e := w.When.domain()
			d.When = &e
		}
		for _, ch := range w.Choices {
			d.Choices = append(d.Choices, c.choiceValue(ch))
		}
		for _, e := range w.Effects {
			d.Effects = append(d.Effects, catalog.Effect{Op: e.Op, Target: e.Target, Ref: c.ref(e.Ref), Value: e.Value.domain()})
		}
		out.Rules = append(out.Rules, d)
	}
	for _, w := range w.Actions {
		d := catalog.ActionDefinition{Entry: entry(w.ID, actionProse), Owner: c.ref(w.Owner), MinimumLevel: w.MinimumLevel, Manual: w.Manual}
		if w.When != nil {
			e := w.When.domain()
			d.When = &e
		}
		for _, cost := range w.Costs {
			d.Costs = append(d.Costs, catalog.ResourceCost{Resource: rules.Slug(cost.Resource), Amount: cost.Amount.domain()})
		}
		out.Actions = append(out.Actions, d)
	}
	return out, c.Err()
}

func recoveryDomain(ws []RecoveryPolicy) []catalog.RecoveryPolicy {
	out := make([]catalog.RecoveryPolicy, 0, len(ws))
	for _, w := range ws {
		p := catalog.RecoveryPolicy{Trigger: w.Trigger, Operation: w.Operation, Amount: w.Amount.domain(), Budget: w.Budget}
		if w.When != nil {
			x := w.When.domain()
			p.When = &x
		}
		out = append(out, p)
	}
	return out
}

func rationalDomain(r *Rational) *catalog.Rational {
	if r == nil {
		return nil
	}
	return &catalog.Rational{Numerator: r.Numerator, Denominator: r.Denominator}
}
