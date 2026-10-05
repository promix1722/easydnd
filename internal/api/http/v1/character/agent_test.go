package character_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/api/http/middleware"
	api "github.com/promix1722/easydnd/internal/api/http/v1/character"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

type reviewModel struct{}

// An import ends on a question about what is still blank; this model asks it
// and, once the test has answered, finishes.
func (reviewModel) Respond(_ context.Context, r charuc.AgentRequest, _ func(string)) (charuc.AgentResponse, error) {
	last := string(r.Input[len(r.Input)-1])
	one := func(id, name, arguments string) (charuc.AgentResponse, error) {
		return charuc.AgentResponse{Calls: []charuc.AgentCall{{ID: id, Name: name, Arguments: arguments}}}, nil
	}
	switch {
	case !strings.Contains(last, "function_call_output") && strings.Contains(last, "Leave them blank"):
		return one("done", "prepare_review", `{"text":"Review","allow_incomplete":true}`)
	case strings.Contains(last, `\"unanswered\"`):
		return one("ask", "ask_user", `{"text":"Fill in what is blank?","options":["Leave them blank"]}`)
	}
	return charuc.AgentResponse{Calls: []charuc.AgentCall{{ID: "name", Name: "resolve_import_facts", Arguments: `{"path":"identity.name","value":"Hero"}`}, {ID: "plan", Name: "plan_import", Arguments: `{"expected":["identity.name"]}`}, {ID: "review", Name: "prepare_review", Arguments: `{"text":"Review","allow_incomplete":true}`}}}, nil
}
func TestImportHTTPUploadResumeOwnershipAndLongPoll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source, err := catalogfile.NewRegistry([]string{filepath.Join("..", "..", "..", "..", "..", "data", "srd_5.1")}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	svc := charuc.NewService(memory.NewCharacterRepository(), memory.NewFolderRepository(), source, nil, nil, slog.New(slog.DiscardHandler))
	cat, err := source.Load(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetPackAccess(testAgentAccess{})
	agent := charuc.NewAgent(svc, reviewModel{}, charuc.AgentConfig{Workers: 1})
	defer agent.Close()
	h := api.New(svc, slog.New(slog.DiscardHandler)).WithAgent(agent)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		id := c.GetHeader("Test-Owner")
		if id == "" {
			id = "owner"
		}
		middleware.SetUser(c, user.User{ID: user.ID(id)})
	})
	r.POST("/sessions", h.AgentCreate)
	r.GET("/sessions/:id", h.AgentGet)
	r.POST("/sessions/:id/files", h.AgentFiles)
	server := httptest.NewServer(r)
	defer server.Close()
	var upload bytes.Buffer
	writer := multipart.NewWriter(&upload)
	file, err := writer.CreateFormFile("files", "sheet.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("Hero, half elf"))
	lock, _ := json.Marshal(helpers.RulesLockOf(cat.Lock))
	_ = writer.WriteField("rules", string(lock))
	_ = writer.Close()
	response, err := http.Post(server.URL+"/sessions", writer.FormDataContentType(), &upload)
	if err != nil {
		t.Fatal(err)
	}
	var view struct {
		Session charuc.AgentSession `json:"session"`
	}
	if err := json.NewDecoder(response.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 200 || view.Session.ID == "" {
		t.Fatalf("create: %d %+v", response.StatusCode, view)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s, _ := agent.Get(domain.OwnerID("owner"), view.Session.ID)
		if s.Status == "review" {
			break
		}
		if s.Status == "waiting" {
			_, _ = agent.Control(domain.OwnerID("owner"), view.Session.ID, "message", "Leave them blank", s.Revision)
		}
		time.Sleep(time.Millisecond)
	}
	request, _ := http.NewRequest("GET", server.URL+"/sessions/"+view.Session.ID, nil)
	request.Header.Set("Test-Owner", "intruder")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatalf("foreign session read=%d", response.StatusCode)
	}
	// poll is one long-poll request from a reader holding (revision, after).
	poll := func(revision, after int) (int, string) {
		t.Helper()
		response, err := http.Get(fmt.Sprintf("%s/sessions/%s?revision=%d&after=%d", server.URL, view.Session.ID, revision, after))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		b, _ := io.ReadAll(response.Body)
		return response.StatusCode, string(b)
	}
	current, _ := agent.Get(domain.OwnerID("owner"), view.Session.ID)
	// A reader on an older revision is behind on more than events: it gets
	// the whole session, at once.
	status, body := poll(current.Revision-1, 2)
	if status != 200 || !strings.Contains(body, `"session"`) || !strings.Contains(body, `"characterId":"chr_`) || !strings.Contains(body, `"status":"review"`) {
		t.Fatalf("bad recovery snapshot %d %s", status, body)
	}
	// A saved source is metadata in browser responses, never a base64 blob.
	if strings.Contains(body, "SGVyby") {
		t.Fatal("source file bytes leaked into transcript")
	}
	// On the current revision it gets only the events it lacks.
	var tail struct {
		Events []charuc.AgentEvent `json:"events"`
	}
	status, body = poll(current.Revision, 2)
	if err := json.Unmarshal([]byte(body), &tail); status != 200 || err != nil || strings.Contains(body, `"session"`) ||
		len(tail.Events) != len(current.Events)-2 || tail.Events[0].ID != 3 {
		t.Fatalf("bad tail %d %s", status, body)
	}
	// Up to date, it is held for the wait and told nothing.
	started := time.Now()
	if status, body = poll(current.Revision, len(current.Events)); status != 204 || body != "" || time.Since(started) < 900*time.Millisecond {
		t.Fatalf("idle poll = %d %q after %s", status, body, time.Since(started))
	}
	// A change during the wait answers it early.
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = agent.Control(domain.OwnerID("owner"), view.Session.ID, "message", "One more thing", current.Revision)
	}()
	started = time.Now()
	if status, body = poll(current.Revision, len(current.Events)); status != 200 || !strings.Contains(body, "One more thing") || time.Since(started) > 900*time.Millisecond {
		t.Fatalf("woken poll = %d %q after %s", status, body, time.Since(started))
	}
	request, _ = http.NewRequest("GET", server.URL+"/sessions/"+view.Session.ID, nil)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != 200 || !bytes.Contains(b, []byte("Hero")) {
		t.Fatalf("reload failed %s", b)
	}
	// A discarded chat answers the poll that was waiting on it, with 404.
	current, _ = agent.Get(domain.OwnerID("owner"), view.Session.ID)
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = agent.Control(domain.OwnerID("owner"), view.Session.ID, "discard", "", current.Revision)
	}()
	for status = 200; status == 200; current, _ = agent.Get(domain.OwnerID("owner"), view.Session.ID) {
		status, body = poll(current.Revision, len(current.Events))
	}
	if status != 404 {
		t.Fatalf("poll of a discarded chat = %d %s", status, body)
	}
}

type testAgentAccess struct{}

func (testAgentAccess) AuthorizeLock(context.Context, user.ID, pack.Lock, pack.Lock) error {
	return nil
}
func (testAgentAccess) Default() pack.Lock { return pack.Lock{} }
