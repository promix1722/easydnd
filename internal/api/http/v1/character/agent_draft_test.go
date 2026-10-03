package character_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/api/http/middleware"
	api "github.com/promix1722/easydnd/internal/api/http/v1/character"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

func TestAgentDraftStandardEditor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := memory.NewCharacterRepository()
	svc := charuc.NewService(repo, memory.NewFolderRepository(), catalogfile.NewSource(filepath.Join("..", "..", "..", "..", "..", "data", "srd_5.1")), nil, nil, slog.New(slog.DiscardHandler))
	agent := charuc.NewAgent(svc, reviewModel{}, charuc.AgentConfig{Workers: 1})
	defer agent.Close()
	s, err := agent.Create(context.Background(), "owner", "", rules.DefaultLocale, []charuc.AgentFile{{Name: "hero.txt", MIME: "text/plain", Data: []byte("Hero")}}, "")
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		s, _ = agent.Get("owner", s.ID)
		if s.Status == "review" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if s.Status != "review" {
		t.Fatal(s.Status)
	}
	h := api.New(svc, slog.New(slog.DiscardHandler)).WithAgent(agent)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		owner := c.GetHeader("Test-Owner")
		if owner == "" {
			owner = "owner"
		}
		middleware.SetUser(c, user.User{ID: user.ID(owner)})
	})
	r.Any("/sessions/:id/draft/*path", h.AgentDraft)
	request := func(method, path, body, owner string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/sessions/"+s.ID+"/draft/"+path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Test-Owner", owner)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	for _, path := range []string{"sheet", "events", "prompts", "catalog/spells"} {
		if w := request("GET", path, "", ""); w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
		if w := request("GET", path, "", "intruder"); w.Code != 404 {
			t.Fatalf("owner isolation: %d", w.Code)
		}
	}
	var log api.EventsResponse
	if err := json.Unmarshal(request("GET", "events", "", "").Body.Bytes(), &log); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"expectedSeq":%d,"expectedRevision":%d,"events":[{"type":"change","changes":[{"path":"identity.name","op":"set","value":{"kind":"string","string":"Edited Hero"}}]}]}`, log.Seq, log.Revision)
	w := request("POST", "events", body, "")
	if w.Code != 200 {
		t.Fatalf("edit: %d %s", w.Code, w.Body)
	}
	if w := request("POST", "events", body, ""); w.Code == 200 {
		t.Fatal("stale write accepted")
	}
	s, _ = agent.Get("owner", s.ID)
	sheet, err := agent.Sheet(context.Background(), s)
	if err != nil || sheet.Identity.Name != "Edited Hero" || s.Status != "paused" {
		t.Fatalf("edit not reflected: %v %s %s", err, sheet.Identity.Name, s.Status)
	}
	var current api.EventsResponse
	if err := json.Unmarshal(request("GET", "events", "", "").Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	replacement := fmt.Sprintf(`{"expectedSeq":%d,"expectedRevision":%d,"event":{"type":"change","changes":[{"path":"identity.name","op":"set","value":{"kind":"string","string":"Revised Hero"}}]}}`, current.Seq, current.Revision)
	target := fmt.Sprintf("events/%d", current.Seq)
	if w := request("PUT", target+"?dryRun=true", replacement, ""); w.Code != 200 {
		t.Fatalf("preview: %d %s", w.Code, w.Body)
	}
	afterPreview, _ := agent.Get("owner", s.ID)
	if afterPreview.Revision != s.Revision {
		t.Fatal("dry run changed draft")
	}
	if w := request("PUT", target, replacement, ""); w.Code != 200 {
		t.Fatalf("revision: %d %s", w.Code, w.Body)
	}
	if w := request("PUT", target, replacement, ""); w.Code == 200 {
		t.Fatal("stale same-sequence revision accepted")
	}
	s, _ = agent.Get("owner", s.ID)
	chars, _ := repo.List(context.Background(), domain.OwnerID("owner"))
	if len(chars) != 0 {
		t.Fatal("editing published a character")
	}
	if w := request("POST", "copy", "", ""); w.Code != 404 {
		t.Fatal("unsupported operation exposed")
	}
	saved, err := agent.Finalize(context.Background(), "owner", s.ID, s.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if w := request("POST", "events", body, ""); w.Code == 200 {
		t.Fatal("saved draft editable")
	}
	final, err := svc.Sheet(context.Background(), "owner", saved.CharacterID, rules.DefaultLocale)
	if err != nil || final.Identity.Name != "Revised Hero" {
		t.Fatal("save lost editor changes", err)
	}
}
