package agent_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	agentuc "github.com/promix1722/easydnd/internal/usecase/agent"
)

// imported runs one turn of tool calls and returns the sheet it leaves and the
// model that made them, whose outputs the test reads. The rules are the SRD as
// a pack, which is what carries the casting profiles spell prompts come from.
func imported(t *testing.T, calls ...agentuc.AgentCall) (domain.State, *script) {
	t.Helper()
	model := &script{turns: [][]agentuc.AgentCall{calls}}
	a := agentuc.NewAgent(namespacedService(t), model, agentuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s agentuc.AgentSession) bool { return s.Status == "paused" })
	sheet, err := a.Sheet(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	return sheet, model
}

const blankPlan = `"level":%LEVEL%,"skills":{},"saves":{},"coins":{},"items":[],"name":null,"personality_traits":[]`

func score(sheet domain.State, ability string) int {
	for key, total := range sheet.Abilities.Scores {
		if local(rules.Slug(key)) == ability {
			return total
		}
	}
	return 0
}

func has(list []rules.Slug, spell string) bool {
	return slices.ContainsFunc(list, func(slug rules.Slug) bool { return local(slug) == spell })
}

func plan(level, rest string) agentuc.AgentCall {
	return call("plan", "plan_import", `{`+strings.ReplaceAll(blankPlan, "%LEVEL%", level)+`,`+rest+`}`)
}

// A sheet never says which part of Strength 20 was an improvement. Left open,
// the improvements stay in the base score and are added again by whoever
// answers them later.
func TestAssignAbilitiesClosesImprovementsWithoutMovingATotal(t *testing.T) {
	sheet, model := imported(t,
		plan("8", `"scores":{"str":20,"dex":9,"con":16,"int":13,"wis":12,"cha":14},"spells":[]`),
		call("facts", "import_facts", `{"facts":[{"kind":"race","name":"Human"},{"kind":"background","name":"Acolyte"},{"kind":"class","name":"Fighter","level":8}]}`),
		call("abilities", "assign_abilities", `{}`),
		call("context", "get_build_context", `{}`),
	)
	want := map[string]int{"str": 20, "dex": 9, "con": 16, "int": 13, "wis": 12, "cha": 14}
	for ability, total := range want {
		if got := score(sheet, ability); got != total {
			t.Errorf("%s = %d, the sheet prints %d", ability, got, total)
		}
	}
	if out := model.output("abilities"); strings.Contains(out[strings.Index(out, `"open"`):], "ability-score-improvement") {
		t.Errorf("an improvement is still open: %s", out)
	}
}

// Four of a wizard's six first spells are still four spells the wizard has,
// whether or not the prompt for six can be answered with them.
func TestAssignSpellsKeepsTheSpellsOfAPromptItCannotFill(t *testing.T) {
	sheet, model := imported(t,
		plan("1", `"scores":{"str":8,"dex":14,"con":12,"int":16,"wis":12,"cha":10},"spells":["Fire Bolt","Light","Magic Missile","Shield","Sleep","Mage Armour"]`),
		call("facts", "import_facts", `{"facts":[{"kind":"race","name":"Human"},{"kind":"background","name":"Acolyte"},{"kind":"class","name":"Wizard","level":1}]}`),
		call("skills", "answer_choices", `{"answers":[{"prompt":"wizard/proficiency/0","picks":["Arcana","History"]}]}`),
		call("spells", "assign_spells", `{}`),
		call("context", "get_build_context", `{}`),
	)
	for _, spell := range []string{"magic-missile", "shield", "sleep", "mage-armor"} {
		if !has(sheet.Spells.Known, spell) {
			t.Errorf("%s was dropped with the prompt it did not fill: %s", spell, model.output("spells"))
		}
	}
	for _, spell := range []string{"fire-bolt", "light"} {
		if !has(sheet.Spells.Cantrips, spell) {
			t.Errorf("%s was dropped: %s", spell, model.output("spells"))
		}
	}
}

// A third-level wizard learns two second-level spells. A sheet with three --
// one scribed from a scroll -- still fills every prompt: the third is kept as
// known rather than getting the prompt that held it refused.
func TestAssignSpellsKeepsWithinTheTopLevelAllowance(t *testing.T) {
	sheet, model := imported(t,
		plan("3", `"scores":{"str":8,"dex":14,"con":12,"int":16,"wis":12,"cha":10},"spells":["Magic Missile","Shield","Sleep","Mage Armor","Detect Magic","Identify","Burning Hands","Invisibility","Hold Person","Web"]`),
		call("facts", "import_facts", `{"facts":[{"kind":"race","name":"Human"},{"kind":"background","name":"Acolyte"},{"kind":"class","name":"Wizard","level":3}]}`),
		call("skills", "answer_choices", `{"answers":[{"prompt":"wizard/proficiency/0","picks":["Arcana","History"]}]}`),
		call("spells", "assign_spells", `{}`),
		call("context", "get_build_context", `{}`),
	)
	out := model.output("spells")
	if !strings.Contains(out, `"prompt":"wizard/spell/spellbook/3"`) || strings.Contains(out, "allowance") {
		t.Errorf("a prompt was refused over the spell level allowance: %s", out)
	}
	if len(sheet.Spells.Known) != 10 {
		t.Errorf("the sheet lists ten spells, the draft has %d: %s", len(sheet.Spells.Known), out)
	}
}

// A cleric prepares from the whole class list, so a listed spell past the
// preparation limit is merely not prepared today -- and the ones the sheet
// marks are the ones that are.
func TestAssignSpellsLeavesAPreparedCastersOverflowUnprepared(t *testing.T) {
	sheet, model := imported(t,
		plan("1", `"scores":{"str":14,"dex":10,"con":12,"int":10,"wis":16,"cha":10},"spells":["Light","Sacred Flame","Thaumaturgy","Bane","Inflict Wounds","Guiding Bolt","Healing Word","Shield of Faith"],"prepared":["Shield of Faith"]`),
		call("facts", "import_facts", `{"facts":[{"kind":"race","name":"Human"},{"kind":"background","name":"Acolyte"},{"kind":"class","name":"Cleric","level":1},{"kind":"subclass","name":"Life"}]}`),
		call("skills", "answer_choices", `{"answers":[{"prompt":"cleric/proficiency/0","picks":["History","Medicine"]}]}`),
		call("spells", "assign_spells", `{}`),
		call("context", "get_build_context", `{}`),
	)
	out := model.output("spells")
	if !has(sheet.Spells.Prepared, "shield-of-faith") {
		t.Errorf("the spell the sheet marks as prepared is not: %v", sheet.Spells.Prepared)
	}
	if !strings.Contains(out, `"beyond":[]`) || strings.Contains(out, `"unprepared":[]`) {
		t.Errorf("the spell past the limit should be unprepared, not kept as an extra: %s", out)
	}
}

// The owner renaming the character while the import runs is not the owner
// taking over its ability scores.
func TestAgentKeepsThePrintedTotalsAcrossAnUnrelatedEdit(t *testing.T) {
	svc := namespacedService(t)
	model := &script{turns: [][]agentuc.AgentCall{{
		plan("1", `"scores":{"str":16,"dex":10,"con":10,"int":10,"wis":10,"cha":10},"spells":[]`),
		call("class", "import_facts", `{"facts":[{"kind":"class","name":"Fighter","level":1}]}`),
		call("ask", "ask_user", `{"text":"Which race?","options":["Half-Orc"]}`),
	}, {
		call("context", "get_build_context", `{}`),
		call("race", "import_facts", `{"facts":[{"kind":"race","name":"Half-Orc"}]}`),
	}}}
	a := agentuc.NewAgent(svc, model, agentuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s agentuc.AgentSession) bool { return s.Status == "waiting" })
	edit(t, svc, s.CharacterID, domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "identity.name", Op: domain.OpSet, Value: domain.StringValue("Grok")}}})
	if s, err = a.Control(testOwner, s.ID, "message", "Half-Orc", s.Revision); err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s agentuc.AgentSession) bool { return s.Status == "paused" })
	sheet, err := a.Sheet(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if score(sheet, "str") != 16 || sheet.Identity.Name != "Grok" {
		t.Errorf("after a rename and a half-orc: STR %d (the sheet prints 16), name %q", score(sheet, "str"), sheet.Identity.Name)
	}
}

// An alignment is the character's own question, and the build reads the
// answer from the sheet. Answered as a pick -- which is how a model answers
// every other open prompt -- it used to be filed under the question and settle
// nothing: the build went on asking, the model answered again, and the builder
// drew a decided block and an open one under one key, neither of which opened.
//
// It settles the question now, which is why it is refused until the owner has
// been asked something: before that the only one deciding is the model.
func TestAnsweringTheCharactersOwnQuestionSettlesIt(t *testing.T) {
	model := &script{turns: [][]agentuc.AgentCall{
		{
			plan("1", `"scores":{"str":15,"dex":14,"con":13,"int":11,"wis":12,"cha":10},"spells":[]`),
			call("facts", "import_facts", `{"facts":[{"kind":"race","name":"Human"},{"kind":"background","name":"Acolyte"},{"kind":"class","name":"Fighter","level":1}]}`),
			call("guess", "answer_choices", `{"answers":[{"prompt":"character/alignment","picks":["neutral-good"]}]}`),
			call("ask", "ask_user", `{"text":"What alignment?","options":["Lawful Good","Neutral"]}`),
		},
		{
			call("alignment", "answer_choices", `{"answers":[{"prompt":"character/alignment","picks":["lawful-good"]},{"prompt":"character/flaw","picks":["Cannot pass up a con."]}]}`),
			// Asked again, it is not a second answer beside the first.
			call("again", "answer_choices", `{"answers":[{"prompt":"character/alignment","picks":["Neutral"]}]}`),
		},
		{call("context", "get_build_context", `{}`)},
	}}
	a := agentuc.NewAgent(namespacedService(t), model, agentuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s agentuc.AgentSession) bool { return s.Status == "waiting" })
	if sheet, _ := a.Sheet(context.Background(), s); !sheet.Identity.Alignment.IsZero() {
		t.Fatalf("the model's own guess was written: %q", sheet.Identity.Alignment)
	}
	if s, err = a.Control(testOwner, s.ID, "message", "Lawful Good", s.Revision); err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s agentuc.AgentSession) bool { return s.Status == "paused" })
	if guess := model.output("guess"); !strings.Contains(guess, "ask_user") {
		t.Errorf("a guess before anybody was asked: %s", guess)
	}
	sheet, err := a.Sheet(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if local(sheet.Identity.Alignment) != "lawful-good" || len(sheet.Identity.Flaws) != 1 {
		t.Errorf("alignment %q, flaws %v: %s", sheet.Identity.Alignment, sheet.Identity.Flaws, model.output("alignment"))
	}
	out := model.output("alignment")
	if open := out[strings.Index(out, `"open"`):strings.Index(out, `"results"`)]; strings.Contains(open, "character/alignment") || strings.Contains(open, "character/flaw") {
		t.Errorf("answered and still asked: %s", open)
	}
	if again := model.output("again"); !strings.Contains(again, "not open") {
		t.Errorf("a second answer to a settled question was taken: %s", again)
	}
	// The transcript names what was chosen, not the key it was chosen by.
	if !slices.ContainsFunc(s.Events, func(e agentuc.AgentEvent) bool {
		return e.Kind == "progress" && strings.Contains(string(e.Data), `"identity.alignment"`) && strings.Contains(string(e.Data), `"Lawful Good"`)
	}) {
		t.Errorf("the alignment is not in the transcript by name: %+v", s.Events)
	}
}

// The transcript is read by the owner, in the language the chat is held in.
// What a tool tells a model is the name that matched, and a sheet printed in
// English matches the English one.
func TestTranscriptNamesEntriesInTheLanguageOfTheChat(t *testing.T) {
	model := &script{turns: [][]agentuc.AgentCall{{
		plan("1", `"scores":{"str":15,"dex":14,"con":13,"int":11,"wis":12,"cha":10},"spells":[],"items":[{"name":"Longsword","count":1,"placement":"equipped"}]`),
		call("facts", "import_facts", `{"facts":[{"kind":"race","name":"Tiefling"},{"kind":"class","name":"Barbarian","level":1},{"path":"base.languages","value":["Common","Infernal"]}]}`),
	}}}
	a := agentuc.NewAgent(namespacedService(t), model, agentuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.LocaleRU, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s agentuc.AgentSession) bool { return s.Status == "paused" })
	shown := ""
	for _, e := range s.Events {
		if e.Kind == "progress" {
			shown += string(e.Data) + "\n"
		}
	}
	for _, name := range []string{"Тифлинг", "Варвар", "Длинный меч", "Общий, Инфернальный"} {
		if !strings.Contains(shown, name) {
			t.Errorf("%s is not named in Russian in a Russian chat:\n%s", name, shown)
		}
	}
}

// What the owner has answered is not put to them again. An owner asked about
// a blank alignment who said to leave it blank used to be asked a message
// later, reworded: the review that followed was refused over the alignment
// "still unanswered", because only a question asked after that refusal
// counted. Their answers are kept now, shown to the model beside the prompt
// they were about, and review is held back only for what they were never
// asked.
func TestAnAnsweredQuestionIsNotAskedAgain(t *testing.T) {
	model := &script{turns: [][]agentuc.AgentCall{
		{
			plan("1", `"scores":{"str":15,"dex":14,"con":13,"int":11,"wis":12,"cha":10},"spells":[],"name":"Rurik"`),
			call("facts", "import_facts", `{"facts":[{"kind":"race","name":"Human"},{"kind":"background","name":"Acolyte"},{"kind":"class","name":"Fighter","level":1}]}`),
			call("picks", "answer_choices", `{"answers":[{"prompt":"fighter/proficiency/0","picks":["skill-athletics","skill-perception"]},{"prompt":"fighter-fighting-style/subfeature/0","picks":["fighter-fighting-style-defense"]}]}`),
			// Nothing says what it is about: it offers alignments.
			call("ask", "ask_user", `{"text":"The sheet leaves the alignment blank. Which one?","options":["Chaotic Good","Neutral Good","Neutral","Leave it blank"]}`),
		},
		{
			call("again", "ask_user", `{"text":"The alignment is still blank. Leave it so, or choose?","options":["Leave it blank","Neutral","Chaotic Good","Neutral Good"]}`),
			call("context", "get_build_context", `{}`),
			call("review", "prepare_review", `{"text":"Ready"}`),
			call("rest", "ask_user", `{"text":"A few details are blank. Shall we fill them in?","options":["Fill them in for me","One by one","Leave them blank"]}`),
		},
		{call("done", "prepare_review", `{"text":"Ready"}`)},
	}}
	a := agentuc.NewAgent(namespacedService(t), model, agentuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, reply := range []string{"Leave it blank", "Leave them blank"} {
		s = waitAgent(t, a, s.ID, func(s agentuc.AgentSession) bool { return s.Status == "waiting" })
		if s, err = a.Control(testOwner, s.ID, "message", reply, s.Revision); err != nil {
			t.Fatal(err)
		}
	}
	s = waitAgent(t, a, s.ID, func(s agentuc.AgentSession) bool { return s.Status == "review" })

	if again := model.output("again"); !strings.Contains(again, "asked twice") || !strings.Contains(again, "Leave it blank") {
		t.Errorf("the question just answered was put again: %s", again)
	}
	var shown struct {
		Prompts []struct {
			ID    string
			Asked struct{ Answer string }
		}
		UserAnswers []struct {
			Answer string
			About  []string
		}
	}
	if err := json.Unmarshal([]byte(model.output("context")), &shown); err != nil {
		t.Fatal(err)
	}
	marked := slices.ContainsFunc(shown.Prompts, func(p struct {
		ID    string
		Asked struct{ Answer string }
	}) bool {
		return p.ID == "character/alignment" && p.Asked.Answer == "Leave it blank"
	})
	if !marked || len(shown.UserAnswers) != 1 || shown.UserAnswers[0].Answer != "Leave it blank" || !slices.Contains(shown.UserAnswers[0].About, "character/alignment") {
		t.Errorf("the owner's answer is not in the model's context: %s", model.output("context"))
	}
	var refused struct {
		Ready      bool
		Unanswered []struct{ ID string }
	}
	if err := json.Unmarshal([]byte(model.output("review")), &refused); err != nil {
		t.Fatal(err)
	}
	left := []string{}
	for _, entry := range refused.Unanswered {
		left = append(left, entry.ID)
	}
	if refused.Ready || slices.Contains(left, "character/alignment") || !slices.Contains(left, "character/ideal") {
		t.Errorf("review is held back over %v: the alignment was answered, the ideal never asked", left)
	}
	asked := 0
	for _, e := range s.Events {
		if e.Kind == "question" {
			asked++
		}
	}
	if asked != 2 {
		t.Errorf("the owner was asked %d questions, want the alignment and the rest, once each", asked)
	}
}
