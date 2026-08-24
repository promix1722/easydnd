package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// These exercise the import route end to end: the real router, the real
// importer and the real compendium, over the committed reference export.

func referenceExport(t *testing.T) json.RawMessage {
	t.Helper()
	// json.RawMessage so that send's json.Marshal hands the bytes back
	// untouched -- the route takes the export itself as the body, not a
	// wrapper object.
	path := filepath.Join("..", "..", "..",
		"docs", "reference_hexsheet", "rouge_3_level.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the reference export: %v", err)
	}
	return json.RawMessage(raw)
}

type importResponse struct {
	ID    string `json:"id"`
	Seq   int    `json:"seq"`
	Sheet struct {
		Identity struct {
			Name string `json:"name"`
			Race string `json:"race"`
		} `json:"identity"`
		Status struct {
			ArmorClass int `json:"armorClass"`
		} `json:"status"`
	} `json:"sheet"`
	Report struct {
		Unresolved []struct {
			Field  string `json:"field"`
			Detail string `json:"detail"`
		} `json:"unresolved"`
		Skipped []struct {
			Field  string `json:"field"`
			Detail string `json:"detail"`
		} `json:"skipped"`
		Open []string `json:"open"`
	} `json:"report"`
}

func TestImportCharacter(t *testing.T) {
	r, session := newFullRouter(t)

	rec := send(t, r, session, http.MethodPost, "/v1/characters/import", referenceExport(t))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	got := decode[importResponse](t, rec)

	if got.ID == "" {
		t.Error("no character id came back")
	}
	if got.Seq == 0 {
		t.Error("seq = 0; a client needs it to post the next event")
	}
	if got.Sheet.Identity.Name != "Сахарок" {
		t.Errorf("name = %q, want Сахарок", got.Sheet.Identity.Name)
	}
	if got.Sheet.Identity.Race != "half-elf" {
		t.Errorf("race = %q, want half-elf", got.Sheet.Identity.Race)
	}
	if got.Sheet.Status.ArmorClass != 14 {
		t.Errorf("armor class = %d, want 14", got.Sheet.Status.ArmorClass)
	}

	// The report is the reason this route returns more than a character.
	if len(got.Report.Unresolved) == 0 {
		t.Error("the report should name Urchin")
	}
	if len(got.Report.Skipped) == 0 {
		t.Error("the report should name the purse and the class resource")
	}
	if len(got.Report.Open) == 0 {
		t.Error("an import answers nothing, so prompts must be open")
	}

	// And the character is a character: it lists like any other.
	list := send(t, r, session, http.MethodGet, "/v1/characters", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", list.Code)
	}
	if !strings.Contains(list.Body.String(), got.ID) {
		t.Errorf("the imported character is missing from the listing: %s", list.Body.String())
	}
}

// The import route creates a character, so it must be guarded exactly as
// creating one is. A route added a line above the authenticated group would
// have no authentication at all, which is the mistake router.go warns about.
func TestImportRequiresASession(t *testing.T) {
	r, _ := newFullRouter(t)

	rec := send(t, r, nil, http.MethodPost, "/v1/characters/import", referenceExport(t))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestImportRejectsRubbish(t *testing.T) {
	r, session := newFullRouter(t)

	tests := []struct {
		name string
		body string
	}{
		// Truncated JSON cannot go through send, which marshals its body, so
		// these are posted as raw bytes -- which is what a browser uploading a
		// half-written file would send anyway.
		{"malformed", `{"nope"`},
		{"not an object", `[]`},
		{"another tool", `{"exportedFrom":"Roll20","character":{"name":"x"}}`},
		{"empty", `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := sendRaw(t, r, session, "/v1/characters/import", tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
			}
			// The error envelope is the shared one, not something this route
			// invented.
			if !strings.Contains(rec.Body.String(), `"error"`) {
				t.Errorf("body is not the standard envelope: %s", rec.Body.String())
			}
		})
	}
}

// sendRaw posts a body verbatim. send marshals whatever it is given, which is
// the right default everywhere else and exactly wrong for testing what happens
// to a file that is not valid JSON.
func sendRaw(
	t *testing.T, r *gin.Engine, session *http.Cookie, path, body string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("X-Request-Id", "test")
	if session != nil {
		req.AddCookie(session)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// An imported character must be answerable, or the import is a dead end.
func TestImportedCharacterCanBeBuiltOn(t *testing.T) {
	r, session := newFullRouter(t)

	rec := send(t, r, session, http.MethodPost, "/v1/characters/import", referenceExport(t))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	imported := decode[importResponse](t, rec)

	prompts := send(t, r, session, http.MethodGet,
		"/v1/characters/"+imported.ID+"/prompts", nil)
	if prompts.Code != http.StatusOK {
		t.Fatalf("prompts status = %d, want 200", prompts.Code)
	}
	if !strings.Contains(prompts.Body.String(), "half-elf/ability-bonus/0") {
		t.Error("the half-elf's ability bonuses should still be open")
	}
}
