package file

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

// Override replaces one complete mechanics definition. Its target identity is
// retained so existing choices and resource usage continue to address it.
// Replacement order cannot choose a winner: two replacements always conflict.
type Override struct {
	Target   Ref                 `json:"target"`
	Version  string              `json:"version"`
	Resource *ResourceDefinition `json:"resource,omitempty"`
	Rule     *RuleDefinition     `json:"rule,omitempty"`
}

func applyOverrides(docs []*PackDocument, m *PackMechanics) error {
	versions := map[string]string{}
	for _, p := range docs {
		versions[p.Manifest.ID] = p.Manifest.Version
	}
	changed := map[rules.Ref]bool{}
	for _, p := range docs {
		for _, o := range p.Mechanics.Overrides {
			target, ok := rules.ParseRef(string(o.Target))
			if !ok {
				return fmt.Errorf("invalid replacement target")
			}
			// Canonical references name the target pack explicitly.
			targetPack := string(o.Target)
			for i, c := range targetPack {
				if c == ':' {
					targetPack = targetPack[:i]
					break
				}
			}
			if targetPack == p.Manifest.ID || versions[targetPack] == "" {
				return fmt.Errorf("replacement needs an installed dependency target")
			}
			constraint, err := semver.NewConstraint(o.Version)
			if err != nil {
				return err
			}
			version, err := semver.StrictNewVersion(versions[targetPack])
			if err != nil {
				return err
			}
			if !constraint.Check(version) {
				return fmt.Errorf("replacement version precondition failed: %s", o.Target)
			}
			if changed[target] {
				return fmt.Errorf("conflicting replacement of %s", o.Target)
			}
			changed[target] = true
			temp := *p
			temp.Mechanics = PackMechanics{}
			if o.Resource != nil {
				temp.Mechanics.Resources = []ResourceDefinition{*o.Resource}
			}
			if o.Rule != nil {
				temp.Mechanics.Rules = []RuleDefinition{*o.Rule}
			}
			normalized, err := normalizeMechanics(&temp)
			if err != nil {
				return err
			}
			found := false
			switch target.Kind {
			case rules.RefResource:
				for i, r := range m.Resources {
					if r.ID == target.Slug.String() {
						replacement := normalized.Resources[0]
						replacement.ID = r.ID
						m.Resources[i] = replacement
						found = true
						break
					}
				}
			case rules.RefRule:
				for i, r := range m.Rules {
					if r.ID == target.Slug.String() {
						replacement := normalized.Rules[0]
						replacement.ID = r.ID
						m.Rules[i] = replacement
						found = true
						break
					}
				}
			}
			if !found {
				return fmt.Errorf("replacement target does not exist: %s", o.Target)
			}
		}
	}
	return nil
}
