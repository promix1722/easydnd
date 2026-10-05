package agent_test

import (
	"context"
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
