package agent_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	agentuc "github.com/promix1722/easydnd/internal/usecase/agent"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

func TestMemoryStore(t *testing.T) {
	repotest.RunAgentStore(t, func(*testing.T) agentuc.Store { return agentuc.NewMemoryStore() })
}

// Two processes over one store: the first stops in the middle of a turn, and
// the second finishes the import the first began.
func TestAgentTurnIsTakenOverByAnotherProcess(t *testing.T) {
	service, store := newService(t), agentuc.NewMemoryStore()
	started, stopped := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	model := answering{modelFunc(func(ctx context.Context, _ agentuc.AgentRequest, _ func(string)) (agentuc.AgentResponse, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-ctx.Done()
			close(stopped)
			return agentuc.AgentResponse{}, ctx.Err()
		}
		return agentuc.AgentResponse{Calls: []agentuc.AgentCall{
			{ID: "name", Name: "resolve_import_facts", Arguments: `{"path":"identity.name","value":"Hero"}`},
			{ID: "plan", Name: "plan_import", Arguments: `{"expected":["identity.name"]}`},
			{ID: "review", Name: "prepare_review", Arguments: `{"text":"Ready","allow_incomplete":true}`}}}, nil
	})}
	first := agentuc.NewAgent(service, model, agentuc.AgentConfig{Workers: 1, Store: store})
	s, err := first.Create(context.Background(), testOwner, "", "en", agentFile(), "Import")
	if err != nil {
		t.Fatal(err)
	}
	<-started
	first.Close()
	<-stopped
	// Released, not failed: the turn was the process's to give up.
	if rec, _ := store.Get(context.Background(), s.ID); rec.Status != "queued" {
		t.Fatalf("a stopping process left its turn %s", rec.Status)
	}
	second := agentuc.NewAgent(service, model, agentuc.AgentConfig{Workers: 1, Store: store})
	defer second.Close()
	done := waitAgent(t, second, s.ID, func(s agentuc.AgentSession) bool { return s.Status == "review" })
	if done.CharacterID != s.CharacterID {
		t.Fatal("the import changed character on the way")
	}
}

// A session outlives the process; a character id does not. After a restart
// the id a session kept names whatever character was made next, and the
// assistant must not write to it.
func TestAgentRefusesACharacterItDidNotMake(t *testing.T) {
	store := agentuc.NewMemoryStore()
	hold := make(chan struct{})
	model := modelFunc(func(ctx context.Context, _ agentuc.AgentRequest, _ func(string)) (agentuc.AgentResponse, error) {
		select {
		case <-hold:
		case <-ctx.Done():
			return agentuc.AgentResponse{}, ctx.Err()
		}
		return agentuc.AgentResponse{Calls: []agentuc.AgentCall{{ID: "name", Name: "resolve_import_facts", Arguments: `{"path":"identity.name","value":"Intruder"}`}}}, nil
	})
	first := agentuc.NewAgent(newService(t), model, agentuc.AgentConfig{Workers: 1, Store: store})
	s, err := first.Create(context.Background(), testOwner, "", "en", agentFile(), "Import")
	if err != nil {
		t.Fatal(err)
	}
	first.Close()

	// The restart: a new repository, whose first character gets the same id.
	service := newService(t)
	mine, err := service.Create(context.Background(), testOwner, "", charuc.NewCharacter{Name: "Mine"})
	if err != nil || mine.ID != s.CharacterID {
		t.Fatalf("expected the id to be reused: %v %v %v", mine.ID, s.CharacterID, err)
	}
	second := agentuc.NewAgent(service, model, agentuc.AgentConfig{Workers: 1, Store: store})
	defer second.Close()
	close(hold)
	for deadline := time.Now().Add(4 * time.Second); ; time.Sleep(time.Millisecond) {
		if rec, _ := store.Get(context.Background(), s.ID); rec.Status == "failed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the session went on with a character that is not its own")
		}
	}
	after, err := service.Repository().Get(context.Background(), mine.ID)
	if err != nil || after.Revision != mine.Revision {
		t.Fatalf("the assistant wrote to a character it did not make: %v", err)
	}
	// And a chat whose character is gone is not offered to be reopened.
	if listed := second.List(testOwner); len(listed) != 0 {
		t.Fatal(listed)
	}
}

// The page learns that a turn ended by the session's revision moving. A
// question ends the turn inside a batch of calls, and the save after it must
// not cost the turn its lease: the save that moves the revision comes last.
func TestAgentTurnEndingOnAQuestionMovesTheRevision(t *testing.T) {
	hold := make(chan struct{})
	model := modelFunc(func(ctx context.Context, _ agentuc.AgentRequest, _ func(string)) (agentuc.AgentResponse, error) {
		<-hold
		return agentuc.AgentResponse{Calls: []agentuc.AgentCall{{ID: "ask", Name: "ask_user", Arguments: `{"text":"Which alignment?","options":["Neutral"]}`}}}, nil
	})
	a := agentuc.NewAgent(newService(t), model, agentuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", "en", agentFile(), "Import")
	if err != nil {
		t.Fatal(err)
	}
	running := waitAgent(t, a, s.ID, func(s agentuc.AgentSession) bool { return s.Status == "running" })
	close(hold)
	// What the page does: hold the running session and wait for it to move.
	after := len(running.Events)
	for deadline := time.Now().Add(4 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		got, full, _, err := a.Poll(context.Background(), testOwner, s.ID, running.Revision, after)
		if err != nil {
			t.Fatal(err)
		}
		if full {
			if got.Status != "waiting" {
				t.Fatalf("status %s", got.Status)
			}
			return
		}
		after += len(got.Events)
	}
	t.Fatal("the page was never told the turn ended")
}
