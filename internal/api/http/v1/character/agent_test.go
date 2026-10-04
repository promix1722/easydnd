package character_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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

func (reviewModel) Respond(context.Context, charuc.AgentRequest, func(string)) (charuc.AgentResponse, error) {
	return charuc.AgentResponse{Calls: []charuc.AgentCall{{ID: "name", Name: "resolve_import_facts", Arguments: `{"path":"identity.name","value":"Hero"}`}, {ID: "plan", Name: "plan_import", Arguments: `{"expected":["identity.name"]}`}, {ID: "review", Name: "prepare_review", Arguments: `{"text":"Review","allow_incomplete":true}`}}}, nil
}
func TestImportHTTPUploadResumeOwnershipAndSSE(t *testing.T) {
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
	r.GET("/sessions/:id/events", h.AgentEvents)
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, _ = http.NewRequestWithContext(ctx, "GET", server.URL+"/sessions/"+view.Session.ID+"/events", nil)
	request.Header.Set("Last-Event-ID", "2")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatal("SSE proxy buffering enabled")
	}
	reader := bufio.NewReader(response.Body)
	frame := ""
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		frame += line
		if line == "\n" {
			break
		}
	}
	if !strings.Contains(frame, "event: snapshot") || !strings.Contains(frame, `"name":"Hero"`) || !strings.Contains(frame, `"status":"review"`) {
		t.Fatalf("bad recovery snapshot %s", frame)
	}
	cancel()
	// A saved source is metadata in browser responses, never a base64 blob.
	if strings.Contains(frame, "SGVyby") {
		t.Fatal("source file bytes leaked into transcript")
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
}

type testAgentAccess struct{}

func (testAgentAccess) AuthorizeLock(context.Context, user.ID, pack.Lock, pack.Lock) error {
	return nil
}
func (testAgentAccess) Default() pack.Lock { return pack.Lock{} }
