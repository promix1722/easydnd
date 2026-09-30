package file

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

var localIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,119}$`)

func validLocalID(id string) bool { return localIDPattern.MatchString(id) }

// A pack can reference itself and explicitly declared dependencies only. An
// unrelated root in the same installation must not make a dangling pack valid.
func validatePackNamespaces(p *PackDocument) error {
	allowed := map[string]bool{p.Manifest.ID: true}
	for _, d := range p.Manifest.Dependencies {
		allowed[d.ID] = true
	}
	check := func(value string) error {
		if value == "" {
			return nil
		}
		if strings.HasPrefix(value, "abilities.") {
			value = strings.TrimPrefix(value, "abilities.")
		}
		if strings.HasPrefix(value, "modifier:") {
			value = strings.TrimPrefix(value, "modifier:")
		}
		parts := strings.Split(value, ":")
		owner := ""
		if len(parts) == 3 {
			owner = parts[0]
		}
		if len(parts) == 4 {
			owner = parts[1]
		}
		if owner == "" {
			id := parts[len(parts)-1]
			if at := strings.Index(id, "/"); at >= 0 {
				owner = id[:at]
			}
		}
		if owner != "" && !allowed[owner] {
			return fmt.Errorf("%s references undeclared dependency %s", p.Manifest.ID, owner)
		}
		return nil
	}
	var walk func(string, any) error
	walk = func(key string, v any) error {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				if err := walk(k, v); err != nil {
					return err
				}
			}
		case []any:
			for _, v := range x {
				if err := walk(key, v); err != nil {
					return err
				}
			}
		case string:
			if slugFields[key] || key == "ref" || key == "reference" || key == "owner" || key == "target" || key == "input" || key == "sharedKey" || key == "resource" {
				return check(x)
			}
		}
		return nil
	}
	raw, _ := json.Marshal(struct {
		Entities  map[string]json.RawMessage
		Mechanics PackMechanics
	}{p.Entities, p.Mechanics})
	var obj any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return err
	}
	if err := walk("", obj); err != nil {
		return err
	}
	for id := range p.Mechanics.Casting {
		if err := check(id); err != nil {
			return err
		}
		if !validLocalID(id) {
			return fmt.Errorf("casting profiles must belong to their pack: %s", id)
		}
	}
	seen := map[string]bool{}
	for _, r := range p.Mechanics.Resources {
		if !validLocalID(r.ID) {
			return fmt.Errorf("invalid local resource ID %s", r.ID)
		}
		seen[r.ID] = true
		if p.Locales[p.Manifest.DefaultLocale]["resources"][r.ID].Name == "" {
			return fmt.Errorf("missing default resource name %s", r.ID)
		}
	}
	for tag, collections := range p.Locales {
		for id := range collections["resources"] {
			if !seen[id] {
				return fmt.Errorf("unknown resource translation %s/%s", tag, id)
			}
		}
		for collection, bundle := range collections {
			if collection != "terms" && collection != "actions" && collection != "resources" && len(bundle) > 0 && p.Entities[collection] == nil {
				return fmt.Errorf("translations for absent collection %s", collection)
			}
		}
	}
	actionIDs := map[string]bool{}
	for _, a := range p.Mechanics.Actions {
		if !validLocalID(a.ID) || actionIDs[a.ID] {
			return fmt.Errorf("invalid or duplicate action ID")
		}
		actionIDs[a.ID] = true
		if p.Locales[p.Manifest.DefaultLocale]["actions"][a.ID].Name == "" {
			return fmt.Errorf("missing default action name %s", a.ID)
		}
	}
	for _, bundles := range p.Locales {
		for id := range bundles["actions"] {
			if !actionIDs[id] {
				return fmt.Errorf("unknown action translation %s", id)
			}
		}
	}
	for _, r := range p.Mechanics.Rules {
		if !validLocalID(r.ID) {
			return fmt.Errorf("invalid local rule ID %s", r.ID)
		}
	}
	// Base aliases exist only in the base release. Other packs cannot smuggle
	// unqualified definitions into that namespace with a slash in their ID.
	return nil
}

func validateRecovery(policies []RecoveryPolicy) error {
	for _, p := range policies {
		if p.Trigger == "" {
			return fmt.Errorf("empty recovery trigger")
		}
		if p.When != nil {
			if err := p.When.domain().Validate(0); err != nil {
				return err
			}
		}
		switch p.Operation {
		case "all":
		case "amount", "budget":
			if err := p.Amount.domain().Validate(0); err != nil {
				return err
			}
			if p.Operation == "budget" && p.Budget == "" {
				return fmt.Errorf("missing recovery budget")
			}
		default:
			return fmt.Errorf("unknown recovery operation %s", p.Operation)
		}
	}
	return nil
}

func grantKind(kind rules.RefKind) bool {
	switch kind {
	case rules.RefFeature, rules.RefTrait, rules.RefFeat, rules.RefSpell, rules.RefProficiency, rules.RefLanguage, rules.RefItem, rules.RefMagicItem:
		return true
	}
	return false
}

func validEffectTarget(s string) bool {
	if strings.HasPrefix(s, "abilities.") {
		return len(s) > len("abilities.")
	}
	switch s {
	case "status.armorClass", "status.initiative", "status.passivePerception", "base.hitPoints.max", "speed.walking", "speed.flying", "speed.swimming", "speed.climbing", "speed.burrowing":
		return true
	}
	return false
}

// Stat effects have a declared combination operation; mixed operations are
// rejected rather than arbitrated by file order. Ability reads form a DAG.
func orderRules(m *PackMechanics) error {
	slices.SortFunc(m.Rules, func(a, b RuleDefinition) int { return strings.Compare(a.ID, b.ID) })
	writers := map[string][]int{}
	operations := map[string]string{}
	owners := map[string][]int{}
	for i, r := range m.Rules {
		owners[string(r.Owner)] = append(owners[string(r.Owner)], i)
		for _, e := range r.Effects {
			if e.Op == "grant" {
				continue
			}
			if op, ok := operations[e.Target]; ok && (op != e.Op || op == "set") {
				return fmt.Errorf("conflicting effects on %s", e.Target)
			}
			operations[e.Target] = e.Op
			if strings.HasPrefix(e.Target, "abilities.") {
				writers[strings.TrimPrefix(e.Target, "abilities.")] = append(writers[strings.TrimPrefix(e.Target, "abilities.")], i)
			}
		}
	}
	deps := make([]map[int]bool, len(m.Rules))
	for i := range deps {
		deps[i] = map[int]bool{}
	}
	var reads func(int, Expression)
	reads = func(i int, e Expression) {
		if e.Op == "read" {
			for _, prefix := range []string{"ability:", "modifier:"} {
				if strings.HasPrefix(e.Ref, prefix) {
					for _, writer := range writers[strings.TrimPrefix(e.Ref, prefix)] {
						deps[i][writer] = true
					}
				}
			}
		}
		for _, arg := range e.Args {
			reads(i, arg)
		}
	}
	for i, r := range m.Rules {
		if r.When != nil {
			reads(i, *r.When)
		}
		for _, e := range r.Effects {
			reads(i, e.Value)
			if e.Op == "grant" {
				for _, child := range owners[string(e.Ref)] {
					deps[child][i] = true
				}
			}
		}
	}
	state := make([]int, len(m.Rules))
	ordered := []RuleDefinition{}
	var visit func(int) error
	visit = func(i int) error {
		if state[i] == 1 {
			return fmt.Errorf("cyclic rule evaluation at %s", m.Rules[i].ID)
		}
		if state[i] == 2 {
			return nil
		}
		state[i] = 1
		for j := range m.Rules {
			if deps[i][j] {
				if err := visit(j); err != nil {
					return err
				}
			}
		}
		state[i] = 2
		ordered = append(ordered, m.Rules[i])
		return nil
	}
	for i := range m.Rules {
		if err := visit(i); err != nil {
			return err
		}
	}
	m.Rules = ordered
	shared := map[string]ResourceDefinition{}
	for _, r := range m.Resources {
		if r.SharedKey == "" {
			continue
		}
		if old, ok := shared[r.SharedKey]; ok {
			if old.Combine != r.Combine || !reflect.DeepEqual(old.Recovery, r.Recovery) {
				return fmt.Errorf("incompatible shared resource policies %s", r.SharedKey)
			}
		}
		shared[r.SharedKey] = r
	}
	return nil
}

// Validate every choice in the decoded graph, including nested entity choices.
// Duplicate option keys must fail at load, not select whichever appears first.
func validateChoices(entities map[string][]any, m PackMechanics) error {
	prompts := map[rules.Slug]bool{}
	var check func(Choice, bool) error
	check = func(w Choice, mechanical bool) error {
		c := &conv{where: "pack choice"}
		choice := c.choiceValue(w)
		if err := c.Err(); err != nil {
			return err
		}
		if choice.Prompt.IsZero() || choice.Choose < 1 {
			return fmt.Errorf("invalid choice prompt/count")
		}
		if mechanical && prompts[choice.Prompt] {
			return fmt.Errorf("duplicate rule prompt %s", choice.Prompt)
		}
		if mechanical {
			prompts[choice.Prompt] = true
		}
		seen := map[rules.Slug]bool{}
		for _, o := range choice.From.Options {
			key := rules.OptionKey(o)
			if key.IsZero() || seen[key] {
				return fmt.Errorf("duplicate or empty option key in %s", choice.Prompt)
			}
			seen[key] = true
		}
		if !mechanical {
			return nil
		}
		if choice.From.Kind != rules.OptionsExplicit && !grantKind(choice.From.Collection) {
			return fmt.Errorf("unsupported rule choice collection")
		}
		var option func(Option) error
		option = func(o Option) error {
			switch o.Kind {
			case OptionRef:
				ref, ok := rules.ParseRef(string(o.Ref))
				if !ok || !grantKind(ref.Kind) {
					return fmt.Errorf("unsupported rule choice grant")
				}
			case OptionAbilityBonus:
			case OptionNested:
				if o.Choice == nil {
					return fmt.Errorf("empty nested choice")
				}
				return check(*o.Choice, true)
			case OptionBundle:
				for _, v := range o.Items {
					if err := option(v); err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("unsupported rule choice effect %s", o.Kind)
			}
			return nil
		}
		for _, o := range w.From.Options {
			if err := option(o); err != nil {
				return err
			}
		}
		return nil
	}
	var walk func(any) error
	walk = func(v any) error {
		switch x := v.(type) {
		case []any:
			for _, v := range x {
				if err := walk(v); err != nil {
					return err
				}
			}
		case map[string]any:
			if _, ok := x["prompt"]; ok {
				raw, _ := json.Marshal(x)
				var w Choice
				if err := json.Unmarshal(raw, &w); err != nil {
					return err
				}
				if err := check(w, false); err != nil {
					return err
				}
			}
			for _, v := range x {
				if err := walk(v); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, rows := range entities {
		for _, row := range rows {
			if err := walk(row); err != nil {
				return err
			}
		}
	}
	for _, r := range m.Rules {
		for _, ch := range r.Choices {
			if err := check(ch, true); err != nil {
				return err
			}
		}
	}
	return nil
}
