package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

type agentProgress struct {
	Operation string `json:"operation"`
	Field     string `json:"field"`
	Value     string `json:"value"`
	Level     int    `json:"level,omitempty"`
}

// Public progress describes validated writes. Catalogue reads and model/tool
// implementation details remain in the internal transcript.
func (a *Agent) recordProgress(ctx context.Context, s *AgentSession, tool string, args agentArgs, result json.RawMessage) {
	p := agentProgress{Operation: "imported"}
	switch tool {
	case "resolve_import_facts":
		// The write itself says what it settled on: the catalogue entry a
		// printed name resolved to, or the path a value landed at.
		var written struct{ Ref, Name, Path string }
		_ = json.Unmarshal(result, &written)
		if ref, ok := rules.ParseRef(written.Ref); ok {
			p.Field, p.Value = ref.Kind.String(), written.Name
			if args.Level != nil {
				p.Level = *args.Level
			}
			break
		}
		p.Field = localPath(written.Path)
		if p.Field == "" {
			p.Field = args.Path
		}
		var value any
		_ = json.Unmarshal(args.Value, &value)
		if values, ok := value.([]any); ok {
			parts := []string{}
			for _, v := range values {
				parts = append(parts, fmt.Sprint(v))
			}
			p.Value = strings.Join(parts, ", ")
		} else {
			p.Value = fmt.Sprint(value)
		}
	case "set_inventory":
		p.Field, p.Value = "item", args.Name+" × "+string(args.Value)
	case "upsert_custom_option":
		// Content the build or the pack already has was imported as itself,
		// and said so on its own line.
		var outcome struct{ Native bool }
		if _ = json.Unmarshal(result, &outcome); outcome.Native {
			return
		}
		p.Operation = "custom"
		p.Field = args.Kind
		p.Value = args.Name
		if p.Value == "" {
			p.Field = "definitions"
			p.Value = ""
		}
	case "answer_choice", "revise_choice":
		p.Field = "choice"
		p.Value = strings.Join(args.Picks, ", ")
		if args.Ref != "" {
			p.Value = args.Ref
		}
		if tool == "revise_choice" {
			p.Operation = "updated"
		}
		cat, err := a.Catalog(ctx, *s)
		if err == nil {
			names := map[string]string{}
			for _, kind := range []string{"spell", "feature", "proficiency", "race", "class", "subrace", "subclass", "background", "item", "feat"} {
				for _, entry := range charuc.CatalogCandidates(cat, kind) {
					ref, _ := rules.ParseRef(entry.Ref)
					names[ref.Slug.String()] = entry.Name
					names[entry.Ref] = entry.Name
				}
			}
			values := []string{}
			for _, pick := range args.Picks {
				if name := names[pick]; name != "" {
					values = append(values, name)
				} else {
					values = append(values, pick)
				}
			}
			if len(values) > 0 {
				p.Value = strings.Join(values, ", ")
			}
		}
	default:
		return
	}
	if p.Field != "" {
		addAgentEvent(s, "progress", "", "", p)
	}
}
