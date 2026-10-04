package character

import (
	"fmt"
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

func temporal(t EventType) bool {
	return t == EventResourceSpent || t == EventResourceRecovered || t == EventRest || t == EventAction
}

// Project validates temporal operations against the build at their position.
// Spending is carried across later build changes; it is never recalculated
// against the final capacity or erased when a grant temporarily disappears.
func Project(log Log, cat *catalog.Catalog) (State, error) {
	if err := log.Validate(); err != nil {
		return State{}, err
	}
	hasTemporal := slices.ContainsFunc(log.Events, func(e Event) bool { return temporal(e.Type) })
	if !hasTemporal {
		s, err := projectBuild(log, cat)
		if err != nil {
			return s, err
		}
		err = actionOffers(&s, cat)
		return s, err
	}
	build := Log{}
	used := map[rules.Slug]int{}
	var state State
	for _, e := range log.Events {
		if !temporal(e.Type) {
			e.Seq = 0
			if err := build.Append(e); err != nil {
				return State{}, err
			}
			s, err := projectBuild(build, cat)
			if err != nil {
				return State{}, err
			}
			state = s
			for id, pool := range state.Resources.Pools {
				pool.Used = used[id]
				state.Resources.Pools[id] = pool
			}
			continue
		}
		if e.Type == EventAction {
			if err := applyActionEvent(&state, cat, e); err != nil {
				return State{}, types.NewValidationError("event %d: %v", e.Seq, err)
			}
		} else if err := applyResourceEvent(&state, cat, e); err != nil {
			return State{}, types.NewValidationError("event %d: %v", e.Seq, err)
		}
		for id, pool := range state.Resources.Pools {
			used[id] = pool.Used
		}
	}
	if err := actionOffers(&state, cat); err != nil {
		return State{}, err
	}
	return state, nil
}

func applyResourceEvent(s *State, cat *catalog.Catalog, e Event) error {
	if len(e.Changes) > 0 || len(e.Choices) > 0 || !e.Ref.IsZero() {
		return fmt.Errorf("resource event contains unrelated changes")
	}
	if e.Type == EventRest {
		if e.Trigger == "" || e.Amount != 0 || !e.Resource.IsZero() {
			return fmt.Errorf("invalid rest payload")
		}
		budgets := map[string]int{}
		allocations := map[string]int{}
		matched := false
		accepted := map[rules.Slug]bool{}
		for id, pool := range s.Resources.Pools {
			selected := -1
			for i, policy := range pool.Recovery {
				if policy.Trigger == e.Trigger {
					if policy.When != nil {
						v, err := policy.When.Eval(variables(*s, cat))
						if err != nil {
							return err
						}
						if v == 0 {
							continue
						}
					}
					if selected >= 0 {
						return fmt.Errorf("ambiguous recovery policy for %s", id)
					}
					selected = i
				}
			}
			if selected < 0 {
				continue
			}
			matched = true
			policy := pool.Recovery[selected]
			switch policy.Operation {
			case "all":
				pool.Used = 0
			case "amount":
				amount, err := policy.Amount.Eval(variables(*s, cat))
				if err != nil {
					return err
				}
				if amount < 0 {
					return fmt.Errorf("negative recovery")
				}
				pool.Used = max(0, pool.Used-amount)
			case "budget":
				amount, err := policy.Amount.Eval(variables(*s, cat))
				if err != nil {
					return err
				}
				if old, ok := budgets[policy.Budget]; ok && old != amount {
					return fmt.Errorf("inconsistent shared recovery budget")
				}
				budgets[policy.Budget] = amount
				n := e.Allocations[id]
				if n < 0 || n > pool.Used {
					return fmt.Errorf("invalid recovery allocation for %s", id)
				}
				allocations[policy.Budget] += n
				accepted[id] = true
				pool.Used -= n
			default:
				return fmt.Errorf("unsupported recovery operation")
			}
			s.Resources.Pools[id] = pool
		}
		if !matched {
			return fmt.Errorf("unknown recovery trigger %q", e.Trigger)
		}
		for id := range e.Allocations {
			if !accepted[id] {
				return fmt.Errorf("unexpected recovery allocation %s", id)
			}
		}
		for key, n := range allocations {
			if n > budgets[key] {
				return fmt.Errorf("recovery budget %s exceeded", key)
			}
		}
		return nil
	}
	pool, ok := s.Resources.Pools[e.Resource]
	if !ok {
		return fmt.Errorf("resource %s is not granted", e.Resource)
	}
	if e.Amount <= 0 {
		return fmt.Errorf("resource amount must be positive")
	}
	if len(e.Allocations) > 0 {
		return fmt.Errorf("allocations belong to rest events")
	}
	if e.Type == EventResourceSpent {
		if e.Amount > pool.Available() {
			return fmt.Errorf("insufficient resource %s", e.Resource)
		}
		pool.Used += e.Amount
	} else {
		allowed := 0
		found := false
		for _, policy := range pool.Recovery {
			if policy.When != nil {
				v, err := policy.When.Eval(variables(*s, cat))
				if err != nil {
					return err
				}
				if v == 0 {
					continue
				}
			}
			if policy.Trigger != e.Trigger || policy.Operation == "budget" {
				continue
			}
			if found {
				return fmt.Errorf("ambiguous recovery")
			}
			found = true
			if policy.Operation == "all" {
				allowed = pool.Used
			} else {
				n, err := policy.Amount.Eval(variables(*s, cat))
				if err != nil {
					return err
				}
				allowed = min(n, pool.Used)
			}
		}
		if !found || e.Amount > allowed {
			return fmt.Errorf("recovery is not permitted by resource policy")
		}
		pool.Used -= e.Amount
	}
	s.Resources.Pools[e.Resource] = pool
	return nil
}

func applyActionEvent(s *State, cat *catalog.Catalog, e Event) error {
	if e.Ref.Kind != rules.RefAction || len(e.Changes) > 0 || len(e.Choices) > 0 || e.Amount != 0 || e.Trigger != "" || !e.Resource.IsZero() || len(e.Allocations) > 0 {
		return fmt.Errorf("invalid action event")
	}
	for _, a := range cat.Mechanics.Actions {
		if a.Slug != e.Ref.Slug {
			continue
		}
		active, err := activeRule(*s, cat, catalog.RuleDefinition{Owner: a.Owner, MinimumLevel: a.MinimumLevel, When: a.When})
		if err != nil {
			return err
		}
		if !active {
			return fmt.Errorf("action is not granted")
		}
		costs := map[rules.Slug]int{}
		for _, cost := range a.Costs {
			n, err := cost.Amount.Eval(variables(*s, cat))
			if err != nil {
				return err
			}
			if n < 0 {
				return fmt.Errorf("negative action cost")
			}
			costs[cost.Resource] += n
		}
		for id, n := range costs {
			pool, ok := s.Resources.Pools[id]
			if !ok || n > pool.Available() {
				return fmt.Errorf("insufficient action resource %s", id)
			}
		}
		for id, n := range costs {
			pool := s.Resources.Pools[id]
			pool.Used += n
			s.Resources.Pools[id] = pool
		}
		return nil
	}
	return fmt.Errorf("unknown action")
}
