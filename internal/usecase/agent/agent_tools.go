package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

type agentArgs struct {
	Scores          map[string]*int `json:"scores"`
	Skills          map[string]*int `json:"skills"`
	Saves           map[string]*int `json:"saves"`
	Spells          []string        `json:"spells"`
	Prepared        []string        `json:"prepared"`
	Proficient      []string        `json:"proficient"`
	Expertise       []string        `json:"expertise"`
	Coins           map[string]*int `json:"coins"`
	Alignment       string          `json:"alignment"`
	Traits          []string        `json:"personality_traits"`
	Ideals          []string        `json:"ideals"`
	Bonds           []string        `json:"bonds"`
	Flaws           []string        `json:"flaws"`
	HitPoints       *int            `json:"hit_points"`
	ArmorClass      *int            `json:"armor_class"`
	Facts           []agentArgs     `json:"facts"`
	Answers         []agentArgs     `json:"answers"`
	Items           []agentArgs     `json:"items"`
	Attacks         []string        `json:"attacks"`
	Expected        []string        `json:"expected"`
	Parent          string          `json:"parent"`
	Ability         string          `json:"ability"`
	Mode            string          `json:"mode"`
	Placement       string          `json:"placement"`
	HitDie          *int            `json:"hit_die"`
	Speed           *int            `json:"speed"`
	Count           int             `json:"count"`
	Selected        *bool           `json:"selected"`
	AllowIncomplete bool            `json:"allow_incomplete"`
	Options         []string        `json:"options"`
	About           []string        `json:"about"`
	Offset          int             `json:"offset"`
	Definition      string          `json:"definition"`
	Kind            string          `json:"kind"`
	Query           string          `json:"query"`
	Level           *int            `json:"level"`
	Ref             string          `json:"ref"`
	Prompt          string          `json:"prompt"`
	Picks           []string        `json:"picks"`
	Sequence        int             `json:"sequence"`
	Path            string          `json:"path"`
	Value           json.RawMessage `json:"value"`
	Assumption      string          `json:"assumption"`
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Source          string          `json:"source"`
	Text            string          `json:"text"`
}

var abilityNames = map[string]string{"strength": "str", "dexterity": "dex", "constitution": "con", "intelligence": "int", "wisdom": "wis", "charisma": "cha"}

// structuralEvents are the catalogue entries a character is made of. Each is
// one event naming the entry, which is why a sheet's "Sorcerer 3" is written
// as a fact rather than answered as a pick.
var structuralEvents = map[rules.RefKind]domain.EventType{rules.RefRace: domain.EventRace, rules.RefSubrace: domain.EventSubrace, rules.RefBackground: domain.EventBackground, rules.RefClass: domain.EventClass, rules.RefSubclass: domain.EventSubclass, rules.RefFeat: domain.EventFeat}

func agentValue(b json.RawMessage) (domain.Value, error) {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return domain.Value{}, err
	}
	switch x := v.(type) {
	case string:
		return domain.StringValue(x), nil
	case float64:
		if x != float64(int(x)) {
			break
		}
		return domain.IntValue(int(x)), nil
	case bool:
		return domain.BoolValue(x), nil
	case []any:
		out := []rules.Slug{}
		for _, v := range x {
			s, ok := v.(string)
			if !ok {
				return domain.Value{}, fmt.Errorf("expected string list")
			}
			out = append(out, rules.Slug(s))
		}
		return domain.SlugListValue(out), nil
	}
	return domain.Value{}, fmt.Errorf("unsupported value")
}

// agentToolName settles which handler a call means. The fact and answer tools
// each take one entry or a batch, and a model sends either shape to either
// name.
func agentToolName(name string, args agentArgs) string {
	switch {
	case name == "resolve_import_facts" && len(args.Facts) > 0:
		return "import_facts"
	case name == "import_facts" && len(args.Facts) == 0 && (args.Ref != "" || args.Path != "" || args.Kind != ""):
		return "resolve_import_facts"
	case name == "answer_choice" && len(args.Answers) > 0:
		return "answer_choices"
	case name == "answer_choices" && len(args.Answers) == 0 && args.Prompt != "":
		return "answer_choice"
	}
	return name
}

func (a *Agent) tool(ctx context.Context, s *AgentSession, name string, b []byte) (any, error) {
	var args agentArgs
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return nil, err
	}
	cat, err := a.Catalog(ctx, *s)
	if err != nil {
		return nil, err
	}
	switch agentToolName(name, args) {
	case "plan_import":
		return a.planImport(ctx, s, cat, args)
	case "import_facts":
		return a.importFacts(ctx, s, cat, args)
	case "resolve_import_facts":
		return a.importFact(ctx, s, cat, args)
	case "get_build_context":
		return a.buildContext(ctx, s, cat, args)
	case "read_source":
		return a.readSource(ctx, s, cat, args)
	case "search_catalog":
		return a.searchCatalog(ctx, s, cat, args)
	case "get_option_details":
		return a.optionDetails(ctx, s, cat, args)
	case "list_choice_options":
		return a.choiceOptions(ctx, s, cat, args)
	case "ask_user":
		return a.askUser(ctx, s, cat, args)
	case "prepare_review":
		return a.prepareReview(ctx, s, cat, args)
	case "upsert_custom_option":
		return a.customOption(ctx, s, cat, args)
	case "assign_skills":
		return a.assignSkills(ctx, s, cat, args)
	case "assign_spells":
		return a.assignSpells(ctx, s, cat, args)
	case "assign_abilities":
		return a.assignAbilities(ctx, s, cat)
	case "answer_choices":
		return a.answerChoices(ctx, s, cat, args)
	case "answer_choice":
		picks, err := a.answer(ctx, s, cat, args)
		if err != nil {
			return nil, err
		}
		return map[string]any{"applied": true, "picks": localKeys(cat, picks)}, nil
	case "revise_choice":
		return a.reviseChoice(ctx, s, cat, args)
	case "set_inventory":
		if len(args.Items) == 0 || len(args.Items) > 300 {
			return nil, fmt.Errorf("list 1-300 items as {name, count, placement}")
		}
		return a.setInventory(ctx, s, cat, args.Items), nil
	default:
		return nil, fmt.Errorf("unknown tool")
	}
}

// buildContext handles the get_build_context tool.
func (a *Agent) buildContext(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	state, err := domain.Project(s.Log, cat)
	if err != nil {
		return nil, err
	}
	open, err := a.openPrompts(s, cat)
	if err != nil {
		return nil, err
	}
	answered := []map[string]any{}
	for _, e := range s.Log.Events {
		for _, answer := range e.Choices {
			answered = append(answered, map[string]any{"seq": e.Seq, "prompt": answer.Prompt, "picks": localKeys(cat, answer.Picks)})
		}
	}
	custom := []map[string]any{}
	for _, c := range domain.CustomOptions(s.Log) {
		custom = append(custom, map[string]any{"id": c.ID, "kind": c.Kind, "name": c.Name, "ref": c.Reference, "selected": c.Selected})
	}
	// userAnswers is the owner's own choices in this chat: what they were
	// asked and what they said, so that nothing is put to them twice.
	return map[string]any{"sheet": agentSheet(state), "differences": a.differences(s, cat), "prompts": open, "answered": answered, "custom": custom, "checklistMissing": localChecklist(a.missing(ctx, s, cat, state)), "userAnswers": s.answers()}, nil
}

// readSource handles the read_source tool.
func (a *Agent) readSource(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	for _, f := range s.Files {
		if strings.EqualFold(f.Name, args.Name) || strings.EqualFold(f.Name, args.Name[strings.LastIndex(args.Name, "/")+1:]) || args.Name == "" && len(s.Files) == 1 {
			if f.MIME == "application/json" || f.MIME == "text/plain" {
				return string(f.Data), nil
			}
			return map[string]string{"attachment": f.Name, "hint": "Read the image or PDF already attached to the conversation."}, nil
		}
	}
	return nil, fmt.Errorf("source not found")
}

// searchCatalog handles the search_catalog tool.
func (a *Agent) searchCatalog(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	kind := args.Kind
	if mapped, ok := naturalKinds[kind]; ok {
		kind = mapped
	}
	if charuc.CatalogCandidates(cat, kind) == nil {
		return nil, fmt.Errorf("unknown kind %q; use spell, race, subrace, class, subclass, background, feat, feature, trait, item, magic-item, language, skill, proficiency or alignment", args.Kind)
	}
	candidates, err := a.search(ctx, s, kind, args.Query, args.Level, nil, false)
	for i := range candidates {
		candidates[i].Details = nil
	}
	return candidates, err
}

// optionDetails handles the get_option_details tool.
func (a *Agent) optionDetails(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	found, err := a.resolve(ctx, s, cat, nil, args.Ref, nil, false)
	if err != nil {
		return nil, err
	}
	if source, ok := a.service.Source().(interface {
		CustomExample(context.Context, pack.Lock, rules.Ref) (any, error)
	}); ok {
		ref, _ := rules.ParseRef(found.Ref)
		example, _ := source.CustomExample(ctx, s.Log.RulesLock(), ref)
		return map[string]any{"option": found, "customExample": example}, nil
	}
	return found, nil
}
