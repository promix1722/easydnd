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
	agentuc "github.com/promix1722/easydnd/internal/usecase/agent"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

type reviewModel struct{}

// An import ends on a question about what is still blank; this model asks it
// and, once the test has answered, finishes.
func (reviewModel) Respond(_ context.Context, r agentuc.AgentRequest, _ func(string)) (agentuc.AgentResponse, error) {
	last := string(r.Input[len(r.Input)-1])
	one := func(id, name, arguments string) (agentuc.AgentResponse, error) {
		return agentuc.AgentResponse{Calls: []agentuc.AgentCall{{ID: id, Name: name, Arguments: arguments}}}, nil
	}
	switch {
	case !strings.Contains(last, "function_call_output") && strings.Contains(last, "Leave them blank"):
		return one("done", "prepare_review", `{"text":"Review","allow_incomplete":true}`)
	case strings.Contains(last, `\"unanswered\"`):
		return one("ask", "ask_user", `{"text":"Fill in what is blank?","options":["Leave them blank"]}`)
	}
	return agentuc.AgentResponse{Calls: []agentuc.AgentCall{{ID: "name", Name: "resolve_import_facts", Arguments: `{"path":"identity.name","value":"Hero"}`}, {ID: "plan", Name: "plan_import", Arguments: `{"expected":["identity.name"]}`}, {ID: "review", Name: "prepare_review", Arguments: `{"text":"Review","allow_incomplete":true}`}}}, nil
}
func TestImportHTTPUploadResumeOwnershipAndPoll(t *testing.T) {
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
	agent := agentuc.NewAgent(svc, reviewModel{}, agentuc.AgentConfig{Workers: 1})
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
		Session agentuc.AgentSession `json:"session"`
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
	// poll is one poll from a reader holding (revision, after).
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
	status, body := poll(current.Revision-1, 3)
	if status != 200 || !strings.Contains(body, `"session"`) || !strings.Contains(body, `"characterId":"chr_`) || !strings.Contains(body, `"status":"review"`) {
		t.Fatalf("bad recovery snapshot %d %s", status, body)
	}
	// A saved source is metadata in browser responses, never a base64 blob.
	if strings.Contains(body, "SGVyby") {
		t.Fatal("source file bytes leaked into transcript")
	}
	// On the current revision it gets only the events it lacks.
	var tail struct {
		Events []agentuc.AgentEvent `json:"events"`
	}
	status, body = poll(current.Revision, 3)
	if err := json.Unmarshal([]byte(body), &tail); status != 200 || err != nil || strings.Contains(body, `"session"`) ||
		len(tail.Events) != len(current.Events)-3 || tail.Events[0].ID != 4 {
		t.Fatalf("bad tail %d %s", status, body)
	}
	// Up to date, it is told nothing, and at once: nothing is held.
	started := time.Now()
	if status, body = poll(current.Revision, len(current.Events)); status != 204 || body != "" || time.Since(started) > 500*time.Millisecond {
		t.Fatalf("idle poll = %d %q after %s", status, body, time.Since(started))
	}
	// A change is in the next poll's answer.
	if _, err = agent.Control(domain.OwnerID("owner"), view.Session.ID, "message", "One more thing", current.Revision); err != nil {
		t.Fatal(err)
	}
	if status, body = poll(current.Revision, len(current.Events)); status != 200 || !strings.Contains(body, "One more thing") {
		t.Fatalf("poll after a change = %d %q", status, body)
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
	// A discarded chat answers its next poll with 404.
	// The message above started turns; a discard is only accepted at the
	// revision the page holds, so wait for them to end.
	for current, _ = agent.Get(domain.OwnerID("owner"), view.Session.ID); current.Status == "queued" || current.Status == "running"; current, _ = agent.Get(domain.OwnerID("owner"), view.Session.ID) {
		time.Sleep(time.Millisecond)
	}
	if _, err = agent.Control(domain.OwnerID("owner"), view.Session.ID, "discard", "", current.Revision); err != nil {
		t.Fatal(err)
	}
	if status, body = poll(current.Revision, len(current.Events)); status != 404 {
		t.Fatalf("poll of a discarded chat = %d %s", status, body)
	}
}

// The wizard as the page drives it: a chat is opened with one question in it,
// the rules are its answer, and the first message is where the character
// begins. Every step is an event the page reads back.
func TestImportHTTPOpenedChatIsAnsweredInSteps(t *testing.T) {
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
	agent := agentuc.NewAgent(svc, reviewModel{}, agentuc.AgentConfig{Workers: 1})
	defer agent.Close()
	h := api.New(svc, slog.New(slog.DiscardHandler)).WithAgent(agent)
	r := gin.New()
	r.Use(func(c *gin.Context) { middleware.SetUser(c, user.User{ID: "owner"}) })
	r.POST("/sessions", h.AgentCreate)
	r.POST("/sessions/:id/files", h.AgentFiles)
	r.POST("/sessions/:id/control", h.AgentControl)
	server := httptest.NewServer(r)
	defer server.Close()
	var view struct {
		Session agentuc.AgentSession `json:"session"`
	}
	send := func(path, contentType string, body io.Reader) int {
		t.Helper()
		response, err := http.Post(server.URL+path, contentType, body)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode == 200 {
			if err := json.NewDecoder(response.Body).Decode(&view); err != nil {
				t.Fatal(err)
			}
		}
		return response.StatusCode
	}
	message := func(text string) (string, io.Reader) {
		var form bytes.Buffer
		writer := multipart.NewWriter(&form)
		_ = writer.WriteField("revision", fmt.Sprint(view.Session.Revision))
		_ = writer.WriteField("instructions", text)
		_ = writer.Close()
		return writer.FormDataContentType(), &form
	}
	if status := send("/sessions", "application/json", nil); status != 200 || view.Session.Status != "opening" ||
		len(view.Session.Events) != 1 || view.Session.Events[0].Kind != "opening" || view.Session.CharacterID != "" {
		t.Fatalf("open: %d %+v", status, view.Session)
	}
	id := view.Session.ID
	if listed := agent.List("owner"); len(listed) != 1 {
		t.Fatalf("an opened chat is not offered to be reopened: %v", listed)
	}
	lock, _ := json.Marshal(map[string]any{"action": "rules", "revision": view.Session.Revision, "rules": helpers.RulesLockOf(cat.Lock)})
	if status := send("/sessions/"+id+"/control", "application/json", bytes.NewReader(lock)); status != 200 || view.Session.Status != "opening" ||
		len(view.Session.Events) != 2 || view.Session.Events[1].Kind != "rules" {
		t.Fatalf("rules: %d %+v", status, view.Session)
	}
	contentType, form := message("A hero")
	if status := send("/sessions/"+id+"/files", contentType, form); status != 200 || view.Session.CharacterID == "" ||
		len(view.Session.Events) != 3 || view.Session.Events[2].Kind != "user" || view.Session.Events[2].Text != "A hero" {
		t.Fatalf("first message: %d %+v", status, view.Session)
	}
	// The rules are final from the first message on.
	lock, _ = json.Marshal(map[string]any{"action": "rules", "revision": view.Session.Revision, "rules": helpers.RulesLockOf(cat.Lock)})
	if status := send("/sessions/"+id+"/control", "application/json", bytes.NewReader(lock)); status != 400 {
		t.Fatalf("rules after the first message = %d", status)
	}
	if _, err := svc.Repository().Get(context.Background(), view.Session.CharacterID); err != nil {
		t.Fatal(err)
	}

	// The opening question may go unanswered: a first message is an answer
	// too. The chat then starts under the deployment's own packs and says so
	// after the message -- and it is held in the language that message was
	// sent in, not the one the page was in when the chat was opened, which
	// is the chat the wizard goes on reopening for days.
	view.Session = agentuc.AgentSession{}
	if status := send("/sessions?locale=en", "application/json", nil); status != 200 || view.Session.Status != "opening" {
		t.Fatalf("open: %d %+v", status, view.Session)
	}
	id = view.Session.ID
	contentType, form = message("Герой")
	if status := send("/sessions/"+id+"/files?locale=ru", contentType, form); status != 200 || view.Session.CharacterID == "" ||
		len(view.Session.Events) != 3 || view.Session.Events[1].Kind != "user" || view.Session.Events[2].Kind != "rules" ||
		!strings.Contains(string(view.Session.Events[2].Data), `"assumed":true`) {
		t.Fatalf("a first message before the rules: %d %+v", status, view.Session)
	}
	if started, _ := agent.Get("owner", id); started.Locale != rules.LocaleRU {
		t.Fatalf("the chat is held in %q, not in the language of its first message", started.Locale)
	}
}

type testAgentAccess struct{}

func (testAgentAccess) AuthorizeLock(context.Context, user.ID, pack.Lock, pack.Lock) error {
	return nil
}
func (testAgentAccess) Default() pack.Lock { return pack.Lock{} }
