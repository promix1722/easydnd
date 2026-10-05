package character_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// script is a model that plays fixed turns and keeps what each tool answered.
// An output reaches the model one request later, so a turn whose answers a
// test reads is never the last one.
type script struct {
	mu      sync.Mutex
	turns   [][]charuc.AgentCall
	outputs map[string]string
}

func (sc *script) Respond(_ context.Context, r charuc.AgentRequest, _ func(string)) (charuc.AgentResponse, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.outputs == nil {
		sc.outputs = map[string]string{}
	}
	for _, raw := range r.Input {
		var item struct {
			CallID string `json:"call_id"`
			Output string `json:"output"`
		}
		if json.Unmarshal(raw, &item) == nil && item.CallID != "" {
			sc.outputs[item.CallID] = item.Output
		}
	}
	if len(sc.turns) == 0 {
		return charuc.AgentResponse{}, nil
	}
	calls := sc.turns[0]
	sc.turns = sc.turns[1:]
	return charuc.AgentResponse{Calls: calls}, nil
}

func (sc *script) output(id string) string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.outputs[id]
}

func call(id, name, arguments string) charuc.AgentCall {
	return charuc.AgentCall{ID: id, Name: name, Arguments: arguments}
}

// namespacedService serves the SRD under another pack id, which is how the
// development pack is installed: every slug then carries the pack's name, and
// nothing a model writes does.
func namespacedService(t *testing.T) *charuc.Service {
	t.Helper()
	registry, err := namespacedRegistry()
	if err != nil {
		t.Fatal(err)
	}
	return charuc.NewService(memory.NewCharacterRepository(), memory.NewFolderRepository(), registry, nil, nil, slog.New(slog.DiscardHandler))
}

// Loaded once: reading the pack is most of what a test using it costs.
var namespacedRegistry = sync.OnceValues(func() (*catalogfile.Registry, error) {
	path := filepath.Join("..", "..", "..", "data", "srd_5.1")
	base, err := catalogfile.LoadPack(path)
	if err != nil {
		return nil, err
	}
	return catalogfile.NewRegistry([]string{path}, []catalogfile.Dependency{{ID: "dnd-2014", Version: base.Manifest.Version}}, "", catalogfile.PackFolder{Path: path, ID: "dnd-2014"})
})

func local(slug rules.Slug) string {
	return slug.String()[strings.LastIndex(slug.String(), "/")+1:]
}

// A sheet is rebuilt as the build that produces it: catalogue entries, the
// build's own choices answered, and nothing custom or pinned where the rules
// already say the same thing.
//
// The printed numbers are written *before* the choices here, on purpose. That
// is the order a model reads a sheet in, and the one that used to make every
// native pick "already held".
func TestAgentRebuildsASheetAsANativeBuild(t *testing.T) {
	for name, newService := range map[string]func(*testing.T) *charuc.Service{"srd": newService, "namespaced pack": namespacedService} {
		t.Run(name, func(t *testing.T) {
			svc := newService(t)
			model := &script{turns: [][]charuc.AgentCall{{
				// The sheet's numbers, as a model reads them. Perception is the
				// sheet's one real disagreement with the rules: an elf is
				// proficient, and this sheet prints the bare Wisdom modifier.
				// The hit points are a misread: 14 is the armor class box.
				call("plan", "plan_import", `{"scores":{"str":8,"dex":16,"con":14,"int":14,"wis":12,"cha":12},"level":3,"hit_points":14,"armor_class":14,"speed":30,
					"skills":{"Acrobatics":5,"Animal Handling":1,"Arcana":2,"Athletics":-1,"Deception":5,"History":2,"Insight":3,"Intimidation":1,"Investigation":2,"Medicine":1,"Nature":2,"Perception":1,"Performance":1,"Persuasion":1,"Religion":4,"Sleight of Hand":5,"Stealth":7,"Survival":1},
					"saves":{"str":-1,"dex":5,"con":2,"int":4,"wis":1,"cha":1},
					"spells":["Mage Hand","Shield","Sleep"],
					"expected":["identity.name","race:High Elf","class:Rogue","subclass:Thief","background:Acolyte","feature:Sneak Attack","item:Leather Armor","item:Hooded Lantern","language:Dwarvish","no such thing"]}`),
				call("numbers", "import_facts", `{"facts":[{"path":"base.languages","value":["Common","Elvish","Dwarvish","Giant"]},{"path":"character/desired-level","value":3},{"path":"base.senses","value":60}]}`),
				// The class is named without its level, as a model answering the
				// open prompt does. The level is the one the sheet printed.
				call("build", "answer_choices", `{"answers":[{"prompt":"character/race","picks":["High Elf"]},{"prompt":"character/class","picks":["Rogue"]},{"prompt":"rogue/subclass","picks":["Thief"]},{"prompt":"character/background","picks":["Acolyte"]}]}`),
				// Written the way the entries above it are, which is how a model
				// writes it. A personality trait is not a racial one.
				call("name", "import_facts", `{"facts":[{"kind":"name","name":"Arya"},{"kind":"alignment","name":"Chaotic Neutral"},{"kind":"trait","name":"Sneaky"}]}`),
			}, {
				// A pick made by hand, and wrong: Arcana is not a skill this
				// character has to double. assign_skills then does the whole
				// distribution, over this and every other skill prompt.
				call("answers", "answer_choices", `{"answers":[{"prompt":"rogue-expertise-1/expertise/0","picks":["Stealth","Arcana"]},{"prompt":"rogue/proficiency/0","picks":["Athletics","Intimidation","Performance","Persuasion"]},{"prompt":"high-elf-cantrip/spell/0","picks":["Mage Hand"]},{"prompt":"acolyte/language/0","picks":["Dwarvish","Giant"]}]}`),
				call("skills", "assign_skills", `{}`),
				// The same for spells. A thief casts nothing: the high elf's
				// cantrip takes Mage Hand, and the two spells the sheet lists
				// past that are kept as known.
				call("spells", "assign_spells", `{}`),
				call("gear", "set_inventory", `{"items":[{"name":"Leather Armor","placement":"equipped"},{"name":"Dagger","count":2},{"name":"Hooded Lantern"},{"name":"Arrows","count":20},{"name":"Fire Shield"}]}`),
				call("held", "upsert_custom_option", `{"id":"sneak","kind":"feature","name":"Sneak Attack (2d6)","description":"Extra damage once per turn"}`),
				call("known", "upsert_custom_option", `{"id":"thief","kind":"subclass","name":"Thief","parent":"Rogue"}`),
				// Spells the sheet lists beyond what the build's prompts offer are
				// catalogue spells all the same, and go where the builder keeps
				// such spells -- one at a time or several, in either tool.
				call("extra", "upsert_custom_option", `{"id":"shield","kind":"spell","name":"Shield","mode":"known"}`),
				call("more", "answer_choices", `{"answers":[{"prompt":"custom/spell/known","picks":["Magic Missile"]},{"prompt":"custom/spell/known","picks":["Mage Hand"]}]}`),
				// One number is kept on purpose, as a user asks for a rolled hit
				// point total; one repeats what the build computes anyway.
				call("kept", "import_facts", `{"facts":[{"path":"base.hitPoints.max","value":26},{"path":"status.armorClass","value":14}]}`),
				call("context", "get_build_context", `{}`),
			}, {
				// The sheet says nothing of personality. Review is refused over
				// what is unanswered until the owner has been asked about it:
				// calling again is not consent, asking is.
				call("unanswered", "prepare_review", `{"text":"Ready"}`),
			}, {
				call("again", "prepare_review", `{"text":"Ready"}`),
			}, {
				call("ask", "ask_user", `{"text":"A few details are blank. Fill them in?","options":["Fill them in for me","One by one","Leave them blank"]}`),
			}, {
				call("review", "prepare_review", `{"text":"Ready"}`),
			}}}
			a := charuc.NewAgent(svc, model, charuc.AgentConfig{Workers: 1})
			defer a.Close()
			s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "Import my sheet")
			if err != nil {
				t.Fatal(err)
			}
			s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "waiting" })
			if s, err = a.Control(testOwner, s.ID, "message", "Leave them blank", s.Revision); err != nil {
				t.Fatal(err)
			}
			s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
			if out := model.output("again"); !strings.Contains(out, `"ready":false`) {
				t.Errorf("review passed without the owner being asked: %s", out)
			}
			if out := model.output("unanswered"); !strings.Contains(out, `"ready":false`) || !strings.Contains(out, "character/personality-trait") {
				t.Errorf("review did not hold for the unanswered questions: %s", out)
			}

			// What the build derives is not owed, and neither is a key nothing
			// could ever check.
			if out := model.output("plan"); !strings.Contains(out, `{"key":"no such thing"`) || !strings.Contains(out, `{"key":"language:Dwarvish"`) || strings.Contains(out, `{"key":"feature:Sneak Attack"`) || strings.Contains(out, "dnd-2014/") {
				t.Errorf("checklist keys nothing can cover were tracked: %s", out)
			}
			// The bonuses say who is proficient; the model is handed the list
			// rather than left to read dots off a scan.
			if out := model.output("plan"); !strings.Contains(out, `"expertise":["Deception","Stealth"]`) || !strings.Contains(out, `"saves":["dex","int"]`) || !strings.Contains(out, `"skills":["Acrobatics","Deception","Insight","Religion","Sleight of Hand","Stealth"]`) {
				t.Errorf("proficiencies were not derived from the printed bonuses: %s", out)
			}
			if out := model.output("numbers"); !strings.Contains(out, "unsupported fact path") || !strings.Contains(out, `"applied":2`) {
				t.Errorf("one bad path took the batch with it, or passed: %s", out)
			}
			if out := model.output("name"); !strings.Contains(out, `"applied":2`) || !strings.Contains(out, "identity.personalityTraits") {
				t.Errorf("identity facts by kind: %s", out)
			}
			if out := model.output("build"); !strings.Contains(out, `"picks":["elf"]`) || !strings.Contains(out, "rogue/proficiency/0") || !strings.Contains(out, "Skill: Stealth") || strings.Contains(out, "dnd-2014/") {
				t.Errorf("answers did not say what they resolved to and opened: %s", out)
			}
			// The model is told which pick was wrong and why, not that
			// "some answers are not valid".
			if out := model.output("answers"); !strings.Contains(out, "field.answer.notProficient") || !strings.Contains(out, "skill-arcana") {
				t.Errorf("rejected answer does not name its pick: %s", out)
			}
			// The sheet's six proficient skills, two of them from the background
			// and none of the four wrong ones picked by hand.
			if out := model.output("skills"); !strings.Contains(out, `"picks":["Acrobatics","Deception","Sleight of Hand","Stealth"],"prompt":"rogue/proficiency/0"`) || !strings.Contains(out, `"picks":["Deception","Stealth"],"prompt":"rogue-expertise-1/expertise/0"`) || !strings.Contains(out, `"unplaced":[]`) {
				t.Errorf("skills were not distributed over the prompts: %s", out)
			}
			// The printed numbers are a reference: where the draft differs from
			// them is reported, and nothing is pinned to make it agree. The
			// misread hit points show against the number the user had kept.
			if out := model.output("context"); !strings.Contains(out, `{"computed":3,"path":"skills.perception.bonus","printed":1}`) || !strings.Contains(out, `{"computed":26,"path":"base.hitPoints.max","printed":14}`) || strings.Contains(out, `"path":"skills.stealth.bonus"`) {
				t.Errorf("differences from the printed numbers are not what the draft shows: %s", out)
			}
			if out := model.output("gear"); !strings.Contains(out, `"unmatched":[{`) || !strings.Contains(out, "Fire Shield") || strings.Contains(out, `:item:shield"`+`,"count"`) {
				t.Errorf("an unknown item was matched to a known one: %s", out)
			}
			if out := model.output("spells"); !strings.Contains(out, `{"picks":["Mage Hand"],"prompt":"high-elf-cantrip/spell/0"}`) || !strings.Contains(out, `"beyond":["Shield","Sleep"]`) || !strings.Contains(out, `"partial":[]`) {
				t.Errorf("spells were not distributed over the prompts: %s", out)
			}
			if out := model.output("more"); !strings.Contains(out, `"picks":["magic-missile"]`) || !strings.Contains(out, "already granted by the build") {
				t.Errorf("extra spells: %s", out)
			}
			for _, id := range []string{"held", "known", "extra"} {
				if out := model.output(id); !strings.Contains(out, `"native":true`) {
					t.Errorf("%s: a custom copy was made of what the build has: %s", id, out)
				}
			}
			if out := model.output("context"); !strings.Contains(out, `"skills.stealth.proficiency":"expertise"`) || strings.Contains(out, "dnd-2014/") {
				t.Errorf("context is not in local names: %s", out)
			}

			saved, err := a.Get(testOwner, s.ID)
			if err != nil {
				t.Fatal(err)
			}
			character, err := svc.Get(context.Background(), testOwner, saved.CharacterID)
			if err != nil {
				t.Fatal(err)
			}
			sheet, err := svc.Sheet(context.Background(), testOwner, character.ID, rules.DefaultLocale)
			if err != nil {
				t.Fatal(err)
			}
			class := sheet.Identity.Classes[0]
			if sheet.Identity.Name != "Arya" || local(sheet.Identity.Alignment) != "chaotic-neutral" {
				t.Errorf("name and alignment = %q, %q", sheet.Identity.Name, sheet.Identity.Alignment)
			}
			if local(sheet.Identity.Race) != "elf" || local(sheet.Identity.Subrace) != "high-elf" || local(sheet.Identity.Background) != "acolyte" || local(class.Class) != "rogue" || class.Level != 3 || local(class.Subclass) != "thief" {
				t.Errorf("identity = %+v", sheet.Identity)
			}
			if sheet.Status.ArmorClass != 14 || sheet.Base.HitPoints.Max != 26 {
				t.Errorf("AC %d, HP %d; want 14 from the build and the 26 kept on purpose", sheet.Status.ArmorClass, sheet.Base.HitPoints.Max)
			}
			skills := map[string]domain.SkillState{}
			for slug, state := range sheet.Skills.BySkill {
				skills[local(slug)] = state
			}
			if skills["stealth"].Proficiency != rules.Expertise || skills["stealth"].Bonus != 7 || skills["deception"].Proficiency != rules.Expertise || skills["acrobatics"].Proficiency != rules.Proficient {
				t.Errorf("skills = %+v", skills)
			}
			if len(sheet.Spells.Cantrips) != 1 || local(sheet.Spells.Cantrips[0]) != "mage-hand" {
				t.Errorf("cantrips = %v", sheet.Spells.Cantrips)
			}
			known := []string{}
			for _, spell := range sheet.Spells.Known {
				known = append(known, local(spell))
			}
			if strings.Join(known, ",") != "shield,sleep,magic-missile" {
				t.Errorf("spells kept outside the build's count = %v", known)
			}
			carried := map[string]int{}
			for _, stack := range append(sheet.Equipment.Equipped, sheet.Equipment.Backpack...) {
				carried[local(stack.Item)] = stack.Count
			}
			if len(carried) != 4 || carried["leather-armor"] != 1 || carried["dagger"] != 2 || carried["lantern-hooded"] != 1 || carried["arrow"] != 20 {
				t.Errorf("inventory = %v", carried)
			}
			if len(sheet.CustomOptions) != 0 {
				t.Errorf("custom options = %+v, want none", sheet.CustomOptions)
			}

			// Only the number imported on purpose is pinned. The armor class
			// imported beside it repeated the build and is gone, and none of the
			// sheet's printed numbers was ever written: the elf keeps the
			// Perception the rules give it.
			var pinned []string
			for _, e := range character.Log.Events {
				for _, ch := range e.Changes {
					for _, prefix := range []string{"skills.", "savingThrows.", "status.", "base.", "proficiencies"} {
						if e.Observed && strings.HasPrefix(string(ch.Path), prefix) {
							pinned = append(pinned, localPath(string(ch.Path)))
						}
					}
				}
			}
			if len(pinned) != 1 || pinned[0] != "base.hitPoints.max" || skills["perception"].Bonus != 3 {
				t.Errorf("printed values still pinned = %v", pinned)
			}
			prompts, err := svc.Prompts(context.Background(), testOwner, character.ID, rules.DefaultLocale)
			if err != nil {
				t.Fatal(err)
			}
			if !domain.Complete(prompts) {
				for _, p := range prompts {
					if !p.Optional {
						t.Errorf("left open: %s", p.Choice.Prompt)
					}
				}
			}
		})
	}
}

func localPath(path string) string {
	parts := strings.Split(path, ".")
	for i, part := range parts {
		parts[i] = local(rules.Slug(part))
	}
	return strings.Join(parts, ".")
}

// A name on a sheet says more than the catalogue's does, and means the entry
// its class offers -- not the nearest spelling anywhere in the rules.
func TestAgentResolvesSubclassNamesWithinTheirClass(t *testing.T) {
	for class, test := range map[string]struct{ printed, subclass string }{
		"Sorcerer":  {"Draconic Bloodline", "draconic"},
		"Barbarian": {"Path of the Berserker", "berserker"},
		"Wizard":    {"School of Evocation", "evocation"},
		"Cleric":    {"Life Domain", "life"},
	} {
		model := &script{turns: [][]charuc.AgentCall{{
			call("facts", "import_facts", `{"facts":[{"kind":"class","name":"`+class+`","level":3},{"kind":"subclass","name":"`+test.printed+`"}]}`),
		}}}
		a := charuc.NewAgent(newService(t), answering{model}, charuc.AgentConfig{Workers: 1})
		s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
		if err != nil {
			t.Fatal(err)
		}
		s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "paused" })
		sheet, err := a.Sheet(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		if len(sheet.Identity.Classes) != 1 || sheet.Identity.Classes[0].Subclass.String() != test.subclass || sheet.Identity.DesiredLevel != 3 {
			t.Errorf("%s %q: %+v", class, test.printed, sheet.Identity)
		}
		a.Close()
	}
}

// A ref the model wrote names something or it is a mistake it can fix. It is
// never dropped in silence, and neither is the turn's remaining work once the
// turn has ended.
func TestAgentRejectsWhatItCannotResolveAndStopsAtAQuestion(t *testing.T) {
	model := &script{turns: [][]charuc.AgentCall{{
		call("bad", "upsert_custom_option", `{"id":"x","kind":"spell","name":"Mystery","ref":"spell:no-such-spell"}`),
		call("ask", "ask_user", `{"text":"Which totem?","options":["Bear","Wolf"]}`),
		call("late", "import_facts", `{"facts":[{"path":"identity.name","value":"Too late"}]}`),
	}}}
	a := charuc.NewAgent(newService(t), answering{model}, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "waiting" })
	sheet, err := a.Sheet(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Identity.Name != "…" || len(sheet.CustomOptions) != 0 {
		t.Fatalf("work ran after the turn ended, or a bad ref was kept: %+v %+v", sheet.Identity, sheet.CustomOptions)
	}
	if _, err = a.Control(testOwner, s.ID, "message", "Bear", s.Revision); err != nil {
		t.Fatal(err)
	}
	waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return model.output("late") != "" })
	if out := model.output("bad"); !strings.Contains(out, "candidates") {
		t.Errorf("unresolvable ref was not reported with candidates: %s", out)
	}
	if out := model.output("late"); !strings.Contains(out, "not run") {
		t.Errorf("a call after the question ran: %s", out)
	}
}

// What the owner changes in the draft is theirs. Pruning removes only what the
// import itself wrote down and the build then reproduced.
func TestAgentPruningLeavesTheOwnersEditsAlone(t *testing.T) {
	svc := newService(t)
	model := &script{turns: [][]charuc.AgentCall{{
		call("plan", "plan_import", `{"expected":["identity.name"],"scores":{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}}`),
		call("facts", "import_facts", `{"facts":[{"path":"identity.name","value":"Hero"},{"kind":"class","name":"Fighter","level":1},{"path":"base.hitPoints.max","value":10},{"path":"status.armorClass","value":10}]}`),
		call("review", "prepare_review", `{"text":"Ready","allow_incomplete":true}`),
	}}}
	a := charuc.NewAgent(svc, answering{model}, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
	// The owner pins the armor class to the very number the build computes.
	edit(t, svc, s.CharacterID, domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "status.armorClass", Op: domain.OpSet, Value: domain.IntValue(10)}}})
	saved, err := a.Get(testOwner, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	character, err := svc.Get(context.Background(), testOwner, saved.CharacterID)
	if err != nil {
		t.Fatal(err)
	}
	var observed, owned int
	for _, e := range character.Log.Events {
		for _, ch := range e.Changes {
			if ch.Path == "status.armorClass" || ch.Path == "base.hitPoints.max" {
				if e.Observed {
					observed++
				} else {
					owned++
				}
			}
		}
	}
	if observed != 0 || owned != 1 {
		t.Fatalf("observed overrides kept = %d, owner's edits kept = %d", observed, owned)
	}
}

// The checklist reminds a model of what it forgot. It does not hold a finished
// draft hostage over an entry nothing can satisfy: that is how an import ends
// in a loop of invented custom content.
func TestAgentChecklistRefusesReviewOnceThenLetsItPass(t *testing.T) {
	model := &script{turns: [][]charuc.AgentCall{{
		call("plan", "plan_import", `{"expected":["identity.name","spell:Wish","item:Dagger"],"scores":{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}}`),
		call("facts", "import_facts", `{"facts":[{"path":"identity.name","value":"Hero"}]}`),
		call("first", "prepare_review", `{"text":"Ready","allow_incomplete":true}`),
	}, {
		// The model fixes what the sheet really has, and leaves the rest.
		call("gear", "set_inventory", `{"items":[{"name":"Dagger"}]}`),
		call("second", "prepare_review", `{"text":"Ready","allow_incomplete":true}`),
	}, {
		call("third", "prepare_review", `{"text":"Ready","allow_incomplete":true}`),
	}}}
	a := charuc.NewAgent(newService(t), answering{model}, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
	if out := model.output("first"); !strings.Contains(out, "spell:Wish") || !strings.Contains(out, "item:Dagger") {
		t.Errorf("first review did not list what is missing: %s", out)
	}
	// A different list is a new reminder; the same list again is an answer.
	if out := model.output("second"); !strings.Contains(out, "spell:Wish") || strings.Contains(out, "item:Dagger") {
		t.Errorf("second review did not re-check: %s", out)
	}
}

// A sheet names its feat outright, and that is written down before the
// question that grants a feat is open. Answering the question with the same
// feat is the feat finding its place, not a second one -- it used to be
// refused as already held, and a model went and picked a different feat.
// The same sheet says "wielded" where the build says "equipped".
func TestAgentPlacesAStatedFeatAndReadsTheSheetsWords(t *testing.T) {
	svc := newService(t)
	model := &script{turns: [][]charuc.AgentCall{{
		call("plan", "plan_import", `{"expected":["identity.name"],"level":4}`),
		call("facts", "import_facts", `{"facts":[{"path":"identity.name","value":"Viktor"},{"kind":"class","name":"Fighter","level":4},{"kind":"feat","name":"Grappler"}]}`),
		call("branch", "answer_choices", `{"answers":[{"prompt":"fighter/ability-score-improvement/4","picks":["feat"]}]}`),
	}, {
		call("feat", "answer_choices", `{"answers":[{"prompt":"fighter/ability-score-improvement/4/1","picks":["Grappler"]}]}`),
		call("gear", "set_inventory", `{"items":[{"name":"Longsword","placement":"wielded"}]}`),
		call("review", "prepare_review", `{"text":"Ready","allow_incomplete":true}`),
	}}}
	a := charuc.NewAgent(svc, answering{model}, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
	if out := model.output("feat"); strings.Contains(out, "error") || strings.Contains(out, "alreadyHeld") {
		t.Fatalf("the stated feat was refused its own question: %s", out)
	}
	if out := model.output("gear"); strings.Contains(out, "placement is") {
		t.Fatalf("a wielded weapon was refused: %s", out)
	}
	sheet, err := svc.Sheet(context.Background(), testOwner, s.CharacterID, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	if len(sheet.Feats) != 1 || sheet.Feats[0] != "grappler" {
		t.Errorf("feats = %v", sheet.Feats)
	}
	if len(sheet.Equipment.Equipped) != 1 {
		t.Errorf("equipped = %+v", sheet.Equipment.Equipped)
	}
}

// An owner who asked not to be asked gets what the sheet states and a list of
// what it does not, never a question -- and a blank name box stays blank
// rather than taking its own caption for a name.
func TestAgentUnattendedLeavesOpenWhatTheSheetDoesNotState(t *testing.T) {
	model := &script{turns: [][]charuc.AgentCall{{
		call("plan", "plan_import", `{"name":"Character Name","personality_traits":["I watch.","I place no stock in wealthy folk."],"scores":{},"level":3,"skills":{},"saves":{},"coins":{},"items":[],"spells":[],"expected":["class:Sorcerer"]}`),
		call("facts", "import_facts", `{"facts":[{"kind":"class","name":"Sorcerer","level":3}]}`),
		call("ask", "ask_user", `{"text":"What is the character called?","options":["Brom"]}`),
		call("review", "prepare_review", `{"text":"Done."}`),
	}, {}}}
	a := charuc.NewAgent(newService(t), model, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.CreateUnattended(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
	sheet, err := a.Sheet(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Identity.Name != "…" || len(sheet.Identity.PersonalityTraits) != 2 {
		t.Fatalf("identity: %q %q", sheet.Identity.Name, sheet.Identity.PersonalityTraits)
	}
	last := s.Events[len(s.Events)-2]
	for _, event := range s.Events {
		if event.Kind == "question" {
			t.Fatal("asked a question")
		}
		if event.Kind == "assistant" {
			last = event
		}
	}
	if !strings.Contains(string(last.Data), `"open"`) {
		t.Fatalf("handoff does not list what is open: %s", last.Data)
	}
}

// Dwarven Toughness is one hit point per level, which the sheet of a hill
// dwarf prints and a build of one used to come up short of.
func TestHillDwarfBuildsWithDwarvenToughness(t *testing.T) {
	hitPoints := func(facts string) int {
		model := &script{turns: [][]charuc.AgentCall{{call("facts", "import_facts", `{"facts":[`+facts+`]}`)}}}
		a := charuc.NewAgent(namespacedService(t), model, charuc.AgentConfig{Workers: 1})
		defer a.Close()
		s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
		if err != nil {
			t.Fatal(err)
		}
		s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "paused" })
		sheet, err := a.Sheet(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		return sheet.Base.HitPoints.Max
	}
	dwarf := `{"kind":"race","name":"Dwarf"},{"kind":"class","name":"Cleric","level":8}`
	if plain, hill := hitPoints(dwarf), hitPoints(dwarf+`,{"kind":"subrace","name":"Hill Dwarf"}`); hill-plain != 8 {
		t.Errorf("hill dwarf cleric 8 has %d hit points, a dwarf without the subrace %d; want 8 apart", hill, plain)
	}
}
