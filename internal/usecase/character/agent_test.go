package character_test

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

type modelFunc func(context.Context, charuc.AgentRequest, func(string)) (charuc.AgentResponse, error)

func (f modelFunc) Respond(ctx context.Context, r charuc.AgentRequest, d func(string)) (charuc.AgentResponse, error) {
	return f(ctx, r, d)
}

// A finished import ends on a question: what to do with what is still blank.
// Where a test is not about that question, answering asks it and the wait
// below replies "leave it blank", so the test reaches the review it is about.
const (
	holdQuestion = "Some details are still blank. Fill them in?"
	holdReply    = "Leave them blank"
)

type answering struct{ charuc.AgentModel }

var holdCalls atomic.Int64

func (m answering) Respond(ctx context.Context, r charuc.AgentRequest, d func(string)) (charuc.AgentResponse, error) {
	call := func(name, arguments string) (charuc.AgentResponse, error) {
		id := "hold-" + strconv.FormatInt(holdCalls.Add(1), 10)
		return charuc.AgentResponse{Calls: []charuc.AgentCall{{ID: id, Name: name, Arguments: arguments}}}, nil
	}
	for i := len(r.Input) - 1; i >= 0; i-- {
		item := string(r.Input[i])
		if !strings.Contains(item, "function_call_output") {
			// The refusal quotes the reply too, so only a message says it.
			if strings.Contains(item, holdReply) {
				return call("prepare_review", `{"text":"Ready","allow_incomplete":true}`)
			}
			break
		}
		if strings.Contains(item, `\"unanswered\"`) {
			return call("ask_user", `{"text":"`+holdQuestion+`","options":["`+holdReply+`"]}`)
		}
	}
	return m.AgentModel.Respond(ctx, r, d)
}

func agentFile() []charuc.AgentFile {
	return []charuc.AgentFile{{Name: "sheet.txt", MIME: "text/plain", Data: []byte("A hero")}}
}
func waitAgent(t *testing.T, a *charuc.Agent, id string, condition func(charuc.AgentSession) bool) charuc.AgentSession {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		s, err := a.Get(testOwner, id)
		if err != nil {
			t.Fatal(err)
		}
		if condition(s) {
			return s
		}
		if s.Status == "waiting" {
			// The question is followed by the record of the call that asked it.
			for i := len(s.Events) - 1; i >= 0 && s.Events[i].Kind != "user"; i-- {
				if s.Events[i].Kind == "question" && s.Events[i].Text == holdQuestion {
					_, _ = a.Control(testOwner, id, "message", holdReply, s.Revision)
					break
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
	s, _ := a.Get(testOwner, id)
	b, _ := json.Marshal(s.Events)
	t.Fatalf("agent did not settle: %s %s", s.Status, b)
	return charuc.AgentSession{}
}
func TestAgentCancelFencesLateResponseAndOwner(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	model := modelFunc(func(ctx context.Context, r charuc.AgentRequest, d func(string)) (charuc.AgentResponse, error) {
		once.Do(func() { close(started) })
		<-release
		d("late text")
		return charuc.AgentResponse{Calls: []charuc.AgentCall{{ID: "late", Name: "resolve_import_facts", Arguments: `{"path":"identity.name","value":"Wrong"}`}}}, nil
	})
	a := charuc.NewAgent(newService(t), answering{model}, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if _, err := a.Get("other", s.ID); err == nil {
		t.Fatal("session leaked across owners")
	}
	s, _ = a.Get(testOwner, s.ID)
	s, err = a.Control(testOwner, s.ID, "stop", "", s.Revision)
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	time.Sleep(20 * time.Millisecond)
	s, _ = a.Get(testOwner, s.ID)
	sheet, err := a.Sheet(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Identity.Name != "…" || s.Status != "paused" {
		t.Fatalf("late response applied: %#v", s)
	}
	for _, e := range s.Events {
		if e.Text == "late text" {
			t.Fatal("late stream published")
		}
	}
}
func TestAgentWorkersBoundConcurrency(t *testing.T) {
	var running, peak atomic.Int32
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	model := modelFunc(func(ctx context.Context, r charuc.AgentRequest, d func(string)) (charuc.AgentResponse, error) {
		n := running.Add(1)
		defer running.Add(-1)
		for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
		}
		entered <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return charuc.AgentResponse{}, nil
	})
	a := charuc.NewAgent(newService(t), answering{model}, charuc.AgentConfig{Workers: 2})
	defer a.Close()
	for i := 0; i < 5; i++ {
		if _, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), ""); err != nil {
			t.Fatal(err)
		}
	}
	<-entered
	<-entered
	select {
	case <-entered:
		t.Fatal("exceeded worker bound")
	case <-time.After(15 * time.Millisecond):
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for len(a.List(testOwner)) == 5 && time.Now().Before(deadline) {
		all := true
		for _, s := range a.List(testOwner) {
			if s.Status != "waiting" {
				all = false
			}
		}
		if all {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if peak.Load() != 2 {
		t.Fatalf("peak=%d", peak.Load())
	}
}
func TestAgentFactsFinalScoresManualAndIdempotentSave(t *testing.T) {
	svc := newService(t)
	model := modelFunc(func(ctx context.Context, r charuc.AgentRequest, d func(string)) (charuc.AgentResponse, error) {
		return charuc.AgentResponse{Calls: []charuc.AgentCall{
			{ID: "name", Name: "resolve_import_facts", Arguments: `{"path":"identity.name","value":"Imported"}`},
			{ID: "race", Name: "resolve_import_facts", Arguments: `{"ref":"race:half-elf"}`},
			{ID: "cha", Name: "resolve_import_facts", Arguments: `{"path":"finalAbilities.cha","value":18}`},
			{ID: "custom", Name: "upsert_custom_option", Arguments: `{"id":"spell","kind":"spell","name":"Unknown spell","description":"Original mechanics","source":"page 2"}`},
			{ID: "plan", Name: "plan_import", Arguments: `{"expected":["identity.name"]}`}, {ID: "review", Name: "prepare_review", Arguments: `{"text":"Ready","allow_incomplete":true}`},
		}}, nil
	})
	a := charuc.NewAgent(svc, answering{model}, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
	sheet, err := a.Sheet(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Abilities.Score(rules.Ability("cha")) != 18 {
		t.Fatalf("bonuses applied twice: %#v", sheet.Abilities)
	}
	if len(sheet.ImportedNotes) != 1 {
		t.Fatal("custom content lost")
	}
	// The character is real and its owner's: an edit made in the builder is
	// an ordinary edit, with no draft to go through.
	edit(t, svc, s.CharacterID, domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "identity.name", Op: domain.OpSet, Value: domain.StringValue("User edit")}}})
	list, err := svc.List(context.Background(), testOwner, "", rules.DefaultLocale)
	if err != nil || len(list) != 1 || list[0].Name != "User edit" {
		t.Fatalf("characters: %#v %v", list, err)
	}
	b, _ := json.Marshal(s)
	var public map[string]any
	_ = json.Unmarshal(b, &public)
	if _, ok := public["Owner"]; ok {
		t.Fatal("private state exposed")
	}
}

func TestAgentSavedScoresCanBeImprovedNormally(t *testing.T) {
	svc := newService(t)
	model := modelFunc(func(context.Context, charuc.AgentRequest, func(string)) (charuc.AgentResponse, error) {
		return charuc.AgentResponse{Calls: []charuc.AgentCall{
			{ID: "n", Name: "resolve_import_facts", Arguments: `{"path":"identity.name","value":"Leveling hero"}`},
			{ID: "r", Name: "resolve_import_facts", Arguments: `{"ref":"race:half-elf"}`},
			{ID: "a", Name: "resolve_import_facts", Arguments: `{"path":"finalAbilities.cha","value":16}`},
			{ID: "plan", Name: "plan_import", Arguments: `{"expected":["identity.name"]}`}, {ID: "done", Name: "prepare_review", Arguments: `{"text":"Ready","allow_incomplete":true}`},
		}}, nil
	})
	a := charuc.NewAgent(svc, answering{model}, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
	saved, err := a.Get(testOwner, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	c, err := svc.Get(context.Background(), testOwner, saved.CharacterID)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range c.Log.Events {
		for _, change := range event.Changes {
			if change.Path == "finalAbilities.cha" {
				t.Fatal("ordinary saved character still has a frozen score")
			}
		}
	}
	// A later ordinary ability edit retains the racial bonus and increases the
	// final total, rather than silently being hidden behind the source snapshot.
	_, err = svc.Apply(context.Background(), testOwner, c.ID, rules.DefaultLocale, c.Log.LastSeq(), domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "abilities.cha", Op: domain.OpSet, Value: domain.IntValue(16)}}})
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := svc.Sheet(context.Background(), testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Abilities.Score(rules.Ability("cha")) != 18 {
		t.Fatalf("saved score did not progress: %v", sheet.Abilities)
	}
}

func TestAgentMatchesTranslatedSpellWithoutChangingIdentity(t *testing.T) {
	var turn atomic.Int32
	results := make(chan string, 1)
	model := modelFunc(func(ctx context.Context, r charuc.AgentRequest, d func(string)) (charuc.AgentResponse, error) {
		if turn.Add(1) == 1 {
			return charuc.AgentResponse{Calls: []charuc.AgentCall{{ID: "search", Name: "search_catalog", Arguments: `{"kind":"spell","query":"Волшебная стрела","level":1}`}}}, nil
		}
		var output struct {
			Output string `json:"output"`
		}
		_ = json.Unmarshal(r.Input[len(r.Input)-1], &output)
		results <- output.Output
		return charuc.AgentResponse{}, nil
	})
	a := charuc.NewAgent(newService(t), answering{model}, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	_, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-results:
		var candidates []charuc.AgentCandidate
		if err := json.Unmarshal([]byte(result), &candidates); err != nil {
			t.Fatalf("search: %s %v", result, err)
		}
		if len(candidates) == 0 || candidates[0].Ref != "srd-2014:spell:magic-missile" || candidates[0].Score != 1 {
			t.Fatalf("wrong translated match: %s", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("search did not complete")
	}
}

// edit changes a wizard's character the way the builder does.
func edit(t *testing.T, svc *charuc.Service, id domain.ID, events ...domain.Event) {
	t.Helper()
	c, err := svc.Get(context.Background(), testOwner, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Apply(charuc.WithRevision(context.Background(), c.Revision), testOwner, id, rules.DefaultLocale, c.Log.LastSeq(), events...); err != nil {
		t.Fatal(err)
	}
}

// The wizard writes to a real character, so what it imports is in the
// builder's own terms: nothing printed is pinned over the build where the
// builder has a question for it.
func TestAgentWritesTheBuildersOwnEntries(t *testing.T) {
	svc := newService(t)
	model := modelFunc(func(context.Context, charuc.AgentRequest, func(string)) (charuc.AgentResponse, error) {
		return charuc.AgentResponse{Calls: []charuc.AgentCall{
			{ID: "plan", Name: "plan_import", Arguments: `{"expected":["identity.name"],"level":3,"scores":{"str":8,"dex":14,"con":15,"int":10,"wis":12,"cha":17}}`},
			{ID: "facts", Name: "import_facts", Arguments: `{"facts":[{"path":"identity.name","value":"Vas Pup"},{"kind":"race","name":"Half-Elf"},{"kind":"class","name":"Rogue","level":3},{"kind":"subclass","name":"Thief"},{"path":"identity.personalityTraits","value":"Always has a plan."},{"path":"identity.flaws","value":"Cannot pass up a con."}]}`},
			{ID: "done", Name: "prepare_review", Arguments: `{"text":"Ready","allow_incomplete":true}`},
		}}, nil
	})
	a := charuc.NewAgent(svc, answering{model}, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	if s.CharacterID == "" {
		t.Fatal("no character behind the chat")
	}
	// The rules were asked and answered before there was a session; the
	// transcript opens with them all the same.
	if first := s.Events[0]; first.Kind != "rules" || !strings.Contains(string(first.Data), `"packs":`) {
		t.Fatalf("the transcript does not open with the rules: %+v %s", first, first.Data)
	}
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
	c, err := svc.Get(context.Background(), testOwner, s.CharacterID)
	if err != nil {
		t.Fatal(err)
	}
	levels, method := map[domain.EventType]int{}, false
	for _, e := range c.Log.Events {
		if e.Type == domain.EventClass || e.Type == domain.EventSubclass {
			levels[e.Type] = e.Level
		}
		for _, ch := range e.Changes {
			path := string(ch.Path)
			if strings.HasPrefix(path, "finalAbilities.") || e.Observed && strings.HasPrefix(path, "identity.") {
				t.Errorf("%s is pinned over the build", path)
			}
			if e.Type == domain.EventInit && path == "identity.ruleset" {
				t.Error("the name and the rules share an entry")
			}
			method = method || path == "abilities.method" && ch.Value.Slug == "manual"
		}
	}
	if levels[domain.EventClass] != 1 || levels[domain.EventSubclass] != 3 || !method {
		t.Errorf("class at %d, subclass at %d, ability scores answered = %v", levels[domain.EventClass], levels[domain.EventSubclass], method)
	}
	sheet, err := svc.Sheet(context.Background(), testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Identity.Name != "Vas Pup" || sheet.Identity.Level() != 3 || sheet.Abilities.Score("cha") != 17 || len(sheet.Identity.PersonalityTraits) != 1 || len(sheet.Identity.Flaws) != 1 {
		t.Errorf("sheet = %+v, cha %d", sheet.Identity, sheet.Abilities.Score("cha"))
	}
	// Discarding the chat discards what it made.
	if _, err = a.Control(testOwner, s.ID, "discard", "", s.Revision); err != nil {
		t.Fatal(err)
	}
	if list, _ := svc.List(context.Background(), testOwner, "", rules.DefaultLocale); len(list) != 0 {
		t.Errorf("discarded character kept: %+v", list)
	}
}

func TestAgentQuestionsKeepRequiredChoicesInChat(t *testing.T) {
	var turn atomic.Int32
	model := modelFunc(func(_ context.Context, request charuc.AgentRequest, _ func(string)) (charuc.AgentResponse, error) {
		switch turn.Add(1) {
		case 1:
			return charuc.AgentResponse{Calls: []charuc.AgentCall{
				{ID: "name", Name: "resolve_import_facts", Arguments: `{"path":"identity.name","value":"Hero"}`},
				{ID: "plan", Name: "plan_import", Arguments: `{"expected":["identity.name"]}`}, {ID: "review", Name: "prepare_review", Arguments: `{"text":"Ready"}`},
			}}, nil
		case 2:
			// A partial sheet cannot skip required choices straight into review.
			b, _ := json.Marshal(request.Input)
			if !strings.Contains(string(b), "remainingChoices") {
				t.Error("missing required-choice feedback")
			}
			return charuc.AgentResponse{Calls: []charuc.AgentCall{{ID: "question", Name: "ask_user", Arguments: `{"text":"Which class should we use?","options":["Rogue","Wizard"]}`}}}, nil
		default:
			return charuc.AgentResponse{Text: "I will use your answer."}, nil
		}
	})
	a := charuc.NewAgent(newService(t), answering{model}, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "Import my hero")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "waiting" })
	if len(s.Events[1].Files) != 1 || s.Events[1].Text != "Import my hero" || len(s.Events[1].Files[0].Data) != 0 {
		t.Fatal("user attachment not recorded with its message")
	}
	var question *charuc.AgentEvent
	for i := range s.Events {
		if s.Events[i].Kind == "question" {
			question = &s.Events[i]
		}
	}
	if question == nil || len(question.Options) != 2 || question.Options[0] != "Rogue" {
		t.Fatal("question missing suggested answers")
	}
	question.Options[0] = "Mutated"
	fresh, _ := a.Get(testOwner, s.ID)
	for _, event := range fresh.Events {
		if event.Kind == "question" && event.Options[0] != "Rogue" {
			t.Fatal("snapshot aliases question options")
		}
	}
	if _, err = a.Control(testOwner, s.ID, "message", "Rogue", s.Revision); err != nil {
		t.Fatal(err)
	}
	// A reply with nothing to press is a response cut short, not a question.
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "paused" && len(s.Events) > len(fresh.Events) })
	found := false
	for _, event := range s.Events {
		if event.Kind == "user" && event.Text == "Rogue" {
			found = true
		}
	}
	if !found {
		t.Fatal("answer not recorded in chat")
	}
}
