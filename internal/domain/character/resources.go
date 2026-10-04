package character

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

type ResourcePool struct {
	ID, Definition       rules.Slug
	Owner                rules.Ref
	Name, Group          string
	Max, Used, SlotLevel int
	Dice                 string
	Recovery             []catalog.RecoveryPolicy
}

func (p ResourcePool) Available() int { return max(0, p.Max-p.Used) }

type Parameter struct {
	Rational   *catalog.Rational
	Boolean    *bool
	Name       string
	Number     int
	Dice, Text string
}
type Contribution struct {
	EventID string
	Rule    rules.Slug
	Owner   rules.Ref
	Target  string
	Amount  int
}

func variables(s State, cat *catalog.Catalog) rules.Variables {
	out := rules.Variables{"level": s.Identity.Level(), "proficiency": s.Status.ProficiencyBonus, "equipped:armor": 0, "equipped:shield": 0}
	for _, c := range s.Identity.Classes {
		out["class:"+c.Class.String()] = c.Level
	}
	for a, score := range s.Abilities.Scores {
		out["ability:"+a.String()] = score
		out["modifier:"+a.String()] = s.Abilities.Modifier(a)
	}
	for _, stack := range s.Equipment.Equipped {
		if stack.Count <= 0 {
			continue
		}
		item, ok := cat.Items.Get(stack.Item)
		if !ok || item.Armor == nil {
			continue
		}
		if item.Armor.Category == catalog.Shield {
			out["equipped:shield"] = 1
		} else {
			out["equipped:armor"] = 1
		}
	}
	return out
}
func ownerLevel(s State, cat *catalog.Catalog, owner rules.Ref) int {
	switch owner.Kind {
	case rules.RefClass:
		for _, c := range s.Identity.Classes {
			if c.Class == owner.Slug {
				return c.Level
			}
		}
	case rules.RefSubclass:
		for _, c := range s.Identity.Classes {
			if c.Subclass == owner.Slug {
				return c.Level
			}
		}
	case rules.RefRace:
		if s.Identity.Race == owner.Slug {
			return max(1, s.Identity.Level())
		}
	case rules.RefSubrace:
		if s.Identity.Subrace == owner.Slug {
			return max(1, s.Identity.Level())
		}
	case rules.RefBackground:
		if s.Identity.Background == owner.Slug {
			return max(1, s.Identity.Level())
		}
	case rules.RefFeat:
		if slices.Contains(s.Feats, owner.Slug) {
			return max(1, s.Identity.Level())
		}
	case rules.RefTrait:
		if slices.Contains(s.Traits, owner.Slug) {
			return max(1, s.Identity.Level())
		}
	case rules.RefFeature:
		if slices.Contains(s.Features, owner.Slug) {
			if f, ok := cat.Features.Get(owner.Slug); ok && !f.Class.IsZero() {
				return ownerLevel(s, cat, rules.NewRef(rules.RefClass, f.Class))
			}
			return max(1, s.Identity.Level())
		}
	}
	return 0
}
func activeRule(s State, cat *catalog.Catalog, r catalog.RuleDefinition) (bool, error) {
	if n := ownerLevel(s, cat, r.Owner); n == 0 || n < r.MinimumLevel {
		return false, nil
	}
	if r.When != nil {
		n, err := r.When.Eval(variables(s, cat))
		return n != 0, err
	}
	return true, nil
}
func (p *projector) resourceDefinitions() error {
	p.state.Resources.Pools = map[rules.Slug]ResourcePool{}
	p.state.Resources.Parameters = map[rules.Slug]Parameter{}
	vars := variables(p.state, p.cat)
	for _, def := range p.cat.Mechanics.Resources {
		if n := ownerLevel(p.state, p.cat, def.Owner); n == 0 || n < def.MinimumLevel {
			continue
		}
		if def.RequiresSubclass {
			has := false
			for _, c := range p.state.Identity.Classes {
				if c.Class == def.Owner.Slug && !c.Subclass.IsZero() {
					has = true
				}
			}
			if !has {
				continue
			}
		}
		input, ok := vars[def.Input]
		if !ok {
			return fmt.Errorf("resource %s: unavailable input %s", def.Slug, def.Input)
		}
		var row catalog.ResourceRow
		if len(def.Rows) > 0 {
			found := false
			for _, candidate := range def.Rows {
				if candidate.From > input {
					break
				}
				row = candidate
				found = true
			}
			if !found {
				return fmt.Errorf("resource %s: progression undefined at %d", def.Slug, input)
			}
		}
		if def.Capacity != nil {
			n, err := def.Capacity.Eval(vars)
			if err != nil {
				return err
			}
			row.Capacity = n
		}
		if row.Capacity < 0 {
			return fmt.Errorf("negative resource capacity")
		}
		if def.Kind == "parameter" {
			p.state.Resources.Parameters[def.Slug] = Parameter{Name: def.Name, Number: row.Capacity, Dice: row.Dice, Text: row.Text, Rational: row.Rational, Boolean: row.Boolean}
			continue
		}
		id := def.Slug
		if def.SharedKey != "" {
			id = def.SharedKey
		}
		pool := ResourcePool{ID: id, Definition: def.Slug, Owner: def.Owner, Name: def.Name, Group: def.Group, Max: row.Capacity, Dice: row.Dice, SlotLevel: row.SlotLevel, Recovery: def.Recovery}
		if previous, ok := p.state.Resources.Pools[id]; ok {
			if def.SharedKey == "" || previous.Dice != pool.Dice || previous.SlotLevel != pool.SlotLevel {
				return fmt.Errorf("incompatible shared resource %s", id)
			}
			if def.Combine == "sum" {
				pool.Max += previous.Max
			} else {
				pool.Max = max(pool.Max, previous.Max)
			}
		}
		p.state.Resources.Pools[id] = pool
		p.state.Contributions = append(p.state.Contributions, Contribution{Rule: def.Slug, Owner: def.Owner, Target: "resources." + id.String(), Amount: row.Capacity})
	}
	for level, pool := range p.state.Resources.SpellSlots {
		if level == 0 || pool.Max == 0 {
			continue
		}
		id := rules.Slug("spell-slots/" + strconv.Itoa(level))
		p.state.Resources.Pools[id] = ResourcePool{ID: id, Definition: "spell-slots", Group: "spell-slots", Max: pool.Max, SlotLevel: level, Recovery: p.cat.Mechanics.Core.SpellSlotRecovery}
	}
	for _, class := range p.state.Identity.Classes {
		def, ok := p.cat.Classes.Get(class.Class)
		if !ok {
			continue
		}
		id := rules.Slug("hit-dice/" + class.Class.String())
		p.state.Resources.Pools[id] = ResourcePool{ID: id, Definition: "hit-dice", Owner: rules.NewRef(rules.RefClass, class.Class), Group: "hit-dice", Max: class.Level, Dice: fmt.Sprintf("1d%d", def.HitDie), Recovery: p.cat.Mechanics.Core.HitDiceRecovery}
	}
	return nil
}
func (p *projector) applyPackRules() error {
	applied := map[rules.Slug]bool{}
	for pass := 0; pass <= len(p.cat.Mechanics.Rules); pass++ {
		changed := false
		for _, r := range p.cat.Mechanics.Rules {
			if applied[r.ID] {
				continue
			}
			active, err := activeRule(p.state, p.cat, r)
			if err != nil {
				return err
			}
			if !active {
				continue
			}
			applied[r.ID] = true
			changed = true
			if r.Manual {
				p.state.ManualRules = append(p.state.ManualRules, r.ID)
			}
			for _, choice := range r.Choices {
				p.answers.chosen(choice, func(o rules.Option) {
					switch v := o.(type) {
					case rules.RefOption:
						p.grantPackRef(v.Ref)
					case rules.AbilityBonusOption:
						if _, pinned := p.finalAbilities[v.Ability]; !pinned {
							p.state.Abilities.Scores[v.Ability] += v.Bonus
						}
					}
				})
			}
			for _, e := range r.Effects {
				if e.Op == "grant" {
					p.grantPackRef(e.Ref)
					continue
				}
				if !strings.HasPrefix(e.Target, "abilities.") {
					p.statEffects = append(p.statEffects, pendingEffect{r, e})
					continue
				}
				if err := p.applyEffect(r, e); err != nil {
					return err
				}
			}
		}
		if !changed {
			return nil
		}
	}
	return fmt.Errorf("rule expansion did not converge")
}
func (p *projector) grantPackRef(ref rules.Ref) {
	switch ref.Kind {
	case rules.RefFeature:
		if !slices.Contains(p.state.Features, ref.Slug) {
			p.state.Features = append(p.state.Features, ref.Slug)
		}
	case rules.RefTrait:
		if !slices.Contains(p.state.Traits, ref.Slug) {
			p.state.Traits = append(p.state.Traits, ref.Slug)
		}
	case rules.RefFeat:
		if !slices.Contains(p.state.Feats, ref.Slug) {
			p.state.Feats = append(p.state.Feats, ref.Slug)
		}
	case rules.RefSpell:
		if !slices.Contains(p.state.Spells.Known, ref.Slug) {
			p.state.Spells.Known = append(p.state.Spells.Known, ref.Slug)
		}
	case rules.RefProficiency:
		p.proficiencies = append(p.proficiencies, ref.Slug)
	case rules.RefLanguage:
		p.addLanguages(ref.Slug)
	case rules.RefItem, rules.RefMagicItem:
		p.addStacks([]catalog.ItemStack{{Item: ref.Slug, Count: 1}})
	}
}

type pendingEffect struct {
	rule   catalog.RuleDefinition
	effect catalog.Effect
}

func (p *projector) applyEffect(r catalog.RuleDefinition, e catalog.Effect) error {
	value, err := e.Value.Eval(variables(p.state, p.cat))
	if err != nil {
		return err
	}
	var target *int
	var ability rules.Ability
	var local int
	var distance *rules.Feet
	if strings.HasPrefix(e.Target, "abilities.") {
		ability = rules.Ability(strings.TrimPrefix(e.Target, "abilities."))
		if _, pinned := p.finalAbilities[ability]; pinned {
			return nil
		}
		local = p.state.Abilities.Score(ability)
		target = &local
	} else if strings.HasPrefix(e.Target, "speed.") {
		kinds := map[string]SpeedKind{"walking": Walking, "flying": Flying, "swimming": Swimming, "climbing": Climbing, "burrowing": Burrowing}
		kind := kinds[strings.TrimPrefix(e.Target, "speed.")]
		for i := range p.state.Base.Speeds {
			if p.state.Base.Speeds[i].Kind == kind {
				distance = &p.state.Base.Speeds[i].Distance
				break
			}
		}
		if distance == nil {
			p.state.Base.Speeds = append(p.state.Base.Speeds, Speed{Kind: kind})
			distance = &p.state.Base.Speeds[len(p.state.Base.Speeds)-1].Distance
		}
		local = int(*distance)
		target = &local
	} else {
		switch e.Target {
		case "status.armorClass":
			target = &p.state.Status.ArmorClass
		case "status.initiative":
			target = &p.state.Status.Initiative
		case "status.passivePerception":
			target = &p.state.Status.PassivePerception
		case "base.hitPoints.max":
			target = &p.state.Base.HitPoints.Max
		}
	}
	if target == nil {
		return fmt.Errorf("unsupported effect target %s", e.Target)
	}
	before := *target
	switch e.Op {
	case "add":
		*target += value
	case "max":
		*target = max(*target, value)
	case "set":
		*target = value
	default:
		return fmt.Errorf("unsupported effect %s", e.Op)
	}
	if ability != "" {
		p.state.Abilities.Scores[ability] = *target
	}
	if distance != nil {
		*distance = rules.Feet(*target)
	}
	p.state.Contributions = append(p.state.Contributions, Contribution{Rule: r.ID, Owner: r.Owner, Target: e.Target, Amount: *target - before})
	return nil
}

type ActionOffer struct {
	ID                rules.Slug
	Owner             rules.Ref
	Name              string
	Manual, Available bool
	Costs             map[rules.Slug]int
}

func actionOffers(s *State, cat *catalog.Catalog) error {
	s.PackActions = nil
	for _, a := range cat.Mechanics.Actions {
		active, err := activeRule(*s, cat, catalog.RuleDefinition{Owner: a.Owner, MinimumLevel: a.MinimumLevel, When: a.When})
		if err != nil {
			return err
		}
		if !active {
			continue
		}
		offer := ActionOffer{ID: a.Slug, Owner: a.Owner, Name: a.Name, Manual: a.Manual, Available: true, Costs: map[rules.Slug]int{}}
		for _, cost := range a.Costs {
			n, err := cost.Amount.Eval(variables(*s, cat))
			if err != nil {
				return err
			}
			if n < 0 {
				return fmt.Errorf("negative action cost")
			}
			offer.Costs[cost.Resource] += n
		}
		for id, n := range offer.Costs {
			pool, ok := s.Resources.Pools[id]
			if !ok || pool.Available() < n {
				offer.Available = false
			}
		}
		s.PackActions = append(s.PackActions, offer)
	}
	return nil
}
