package character_test

import (
	"context"
	"encoding/json"
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
		time.Sleep(time.Millisecond)
	}
	t.Fatal("agent did not settle")
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
	a := charuc.NewAgent(newService(t), model, charuc.AgentConfig{Workers: 1})
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
	a := charuc.NewAgent(newService(t), model, charuc.AgentConfig{Workers: 2})
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
			{ID: "review", Name: "prepare_review", Arguments: `{"text":"Ready","allow_incomplete":true}`},
		}}, nil
	})
	a := charuc.NewAgent(svc, model, charuc.AgentConfig{Workers: 1})
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
	s, err = a.Edit(context.Background(), testOwner, s.ID, s.Revision, []domain.Event{{Type: domain.EventChange, Changes: []domain.Change{{Path: "identity.name", Op: domain.OpSet, Value: domain.StringValue("User edit")}}}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := a.Finalize(context.Background(), testOwner, s.ID, s.Revision)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.Finalize(context.Background(), testOwner, s.ID, s.Revision)
	if err != nil || first.CharacterID != second.CharacterID {
		t.Fatal("duplicate finalization")
	}
	list, err := svc.List(context.Background(), testOwner, "", rules.DefaultLocale)
	if err != nil || len(list) != 1 || list[0].Name != "User edit" {
		t.Fatalf("saved characters: %#v %v", list, err)
	}
	if _, err = a.Control(testOwner, s.ID, "message", "modify after save", second.Revision); err == nil {
		t.Fatal("saved history was mutable")
	}
	b, _ := json.Marshal(first)
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
			{ID: "done", Name: "prepare_review", Arguments: `{"text":"Ready","allow_incomplete":true}`},
		}}, nil
	})
	a := charuc.NewAgent(svc, model, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
	saved, err := a.Finalize(context.Background(), testOwner, s.ID, s.Revision)
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
	a := charuc.NewAgent(newService(t), model, charuc.AgentConfig{Workers: 1})
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

func TestAgentDraftEditorRejectsActiveTurn(t *testing.T) {
	started := make(chan struct{})
	model := modelFunc(func(ctx context.Context, _ charuc.AgentRequest, _ func(string)) (charuc.AgentResponse, error) {
		close(started)
		<-ctx.Done()
		return charuc.AgentResponse{}, ctx.Err()
	})
	a := charuc.NewAgent(newService(t), model, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	if err != nil {
		t.Fatal(err)
	}
	<-started
	called := false
	if err := a.WithDraft(testOwner, s.ID, true, func(*charuc.Service) { called = true }); err == nil || called {
		t.Fatal("active draft editor write admitted")
	}
	if err := a.WithDraft("other", s.ID, false, func(*charuc.Service) { called = true }); err == nil || called {
		t.Fatal("foreign draft read admitted")
	}
	if err := a.WithDraft(testOwner, s.ID, false, func(svc *charuc.Service) {
		_, err = svc.Sheet(context.Background(), testOwner, domain.ID(s.ID), rules.DefaultLocale)
	}); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestAgentQuestionsKeepRequiredChoicesInChat(t *testing.T) {
	var turn atomic.Int32
	model := modelFunc(func(_ context.Context, request charuc.AgentRequest, _ func(string)) (charuc.AgentResponse, error) {
		switch turn.Add(1) {
		case 1:
			return charuc.AgentResponse{Calls: []charuc.AgentCall{
				{ID: "name", Name: "resolve_import_facts", Arguments: `{"path":"identity.name","value":"Hero"}`},
				{ID: "review", Name: "prepare_review", Arguments: `{"text":"Ready"}`},
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
	a := charuc.NewAgent(newService(t), model, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "Import my hero")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "waiting" })
	if len(s.Events[0].Files) != 1 || s.Events[0].Text != "Import my hero" || len(s.Events[0].Files[0].Data) != 0 {
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
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "waiting" && len(s.Events) > len(fresh.Events) })
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
