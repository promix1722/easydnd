package httpapi_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
	packapi "github.com/promix1722/easydnd/internal/api/http/v1/pack"
	"github.com/promix1722/easydnd/internal/config"
)

func TestPackHTTPPrivateSelectionAndRetainedCharacter(t *testing.T) {
	r, owner, _, _ := newFullRouterInEnv(t, config.EnvDevelopment, true)
	outsider := guest(t, r, helpers.CookieOptions{Secure: false})
	raw, err := os.ReadFile("../../../data/packs/examples/tactician.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	icon, err := os.ReadFile("../../../data/srd_5.1/spell-icons/magic-missile.webp")
	if err != nil {
		t.Fatal(err)
	}
	doc["icons"] = map[string]any{"spells": map[string][]byte{"guiding-mark": icon}}
	created := send(t, r, owner, http.MethodPost, "/v1/packs/import?title=My%20rules", doc)
	if created.Code != 200 {
		t.Fatal(created.Body.String())
	}
	p := decode[packapi.Record](t, created)
	published := send(t, r, owner, http.MethodPost, "/v1/packs/"+p.ID+"/publish", map[string]any{"expectedRevision": p.Revision})
	if published.Code != 200 {
		t.Fatal(published.Body.String())
	}
	p = decode[packapi.Record](t, published)
	resolved := send(t, r, owner, http.MethodPost, "/v1/packs/resolve", map[string]any{"packs": p.Releases})
	if resolved.Code != 200 {
		t.Fatal(resolved.Body.String())
	}
	lock := decode[helpers.RulesLock](t, resolved)
	for _, path := range []string{"/v1/packs/" + p.ID, "/v1/packs/" + p.ID + "/export?version=1.0.0", "/v1/packs/catalog/spells?packs=" + p.ID + "@1.0.0"} {
		rec := send(t, r, outsider, http.MethodGet, path, nil)
		if rec.Code != 404 {
			t.Fatalf("private read %s = %d", path, rec.Code)
		}
	}
	forged := send(t, r, outsider, http.MethodPost, "/v1/characters", map[string]any{"name": "Intruder", "rules": lock})
	if forged.Code != 404 {
		t.Fatalf("forged character lock: %d %s", forged.Code, forged.Body)
	}
	g := createGroup(t, r, owner, "Homebrew table")
	invite := inviteToken(t, r, owner, g.ID, "player")
	joined := send(t, r, outsider, http.MethodPost, "/v1/invites/accept", map[string]any{"token": invite})
	if joined.Code != 200 {
		t.Fatal(joined.Body.String())
	}
	shared := send(t, r, owner, http.MethodPost, "/v1/groups/"+g.ID+"/packs", map[string]any{"pack": p.ID, "version": "1.0.0"})
	if shared.Code != 200 {
		t.Fatal(shared.Body.String())
	}
	ch := send(t, r, outsider, http.MethodPost, "/v1/characters", map[string]any{"name": "Homebrew hero", "rules": lock})
	if ch.Code != 201 {
		t.Fatal(ch.Body.String())
	}
	hero := decode[characterapi.CreateResponse](t, ch)
	catalog := send(t, r, outsider, http.MethodGet, "/v1/packs/catalog/spells?packs="+p.ID+"@1.0.0", nil)
	if catalog.Code != 200 || catalog.Header().Get("Cache-Control") != "no-store" || !strings.Contains(catalog.Body.String(), p.ID+"/") {
		t.Fatalf("catalog: %d", catalog.Code)
	}
	if !strings.Contains(catalog.Body.String(), "data:image/webp;base64,") {
		t.Fatal("shared catalog lost artwork")
	}
	exported := send(t, r, owner, http.MethodGet, "/v1/packs/"+p.ID+"/export?version=1.0.0", nil)
	var exportedDoc struct {
		Icons struct {
			Spells map[string][]byte `json:"spells"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(exported.Body.Bytes(), &exportedDoc); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(icon, exportedDoc.Icons.Spells["guiding-mark"]) {
		t.Fatal("published export lost artwork")
	}
	for _, path := range []string{"/v1/packs/spells?pack=" + p.ID, "/v1/packs/spell-filters"} {
		rec := send(t, r, outsider, http.MethodGet, path, nil)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), p.ID) {
			t.Fatalf("shared browse %s: %d %s", path, rec.Code, rec.Body)
		}
	}
	unshared := send(t, r, owner, http.MethodDelete, "/v1/groups/"+g.ID+"/packs?pack="+p.ID, nil)
	if unshared.Code != 200 {
		t.Fatal(unshared.Body.String())
	}
	denied := send(t, r, outsider, http.MethodGet, "/v1/packs/catalog/spells?packs="+p.ID+"@1.0.0", nil)
	if denied.Code != 404 {
		t.Fatal("cached catalogue bypassed authorization")
	}
	for _, path := range []string{"/v1/packs/spells?pack=" + p.ID, "/v1/packs/spell-filters"} {
		rec := send(t, r, outsider, http.MethodGet, path, nil)
		if rec.Code != 200 || strings.Contains(rec.Body.String(), p.ID) {
			t.Fatalf("revoked pack remained in browse %s: %d %s", path, rec.Code, rec.Body)
		}
	}
	for _, path := range []string{"/v1/characters/" + hero.ID + "/sheet", "/v1/characters/" + hero.ID + "/catalog/spells"} {
		rec := send(t, r, outsider, http.MethodGet, path, nil)
		if rec.Code != 200 {
			t.Fatalf("retained character: %d %s", rec.Code, rec.Body)
		}
	}
	migration := send(t, r, owner, http.MethodPost, "/v1/characters", map[string]any{"name": "Default"})
	defaultHero := decode[characterapi.CreateResponse](t, migration)
	// An unrelated guest cannot migrate another user's character either.
	attempt := send(t, r, outsider, http.MethodPost, "/v1/characters/"+defaultHero.ID+"/rules?dryRun=true", map[string]any{"expectedRevision": 1, "rules": lock})
	if attempt.Code == 200 {
		t.Fatal("foreign migration allowed")
	}
}

func TestAggregateSpellBrowsePrivateSourcesAndVersions(t *testing.T) {
	r, owner, _, _ := newFullRouterInEnv(t, config.EnvDevelopment, true)
	outsider := guest(t, r, helpers.CookieOptions{Secure: false})
	raw, err := os.ReadFile("../../../data/packs/examples/tactician.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc["manifest"].(map[string]any)["sources"] = map[string]string{"phb": "Player's Handbook", "xge": "Xanathar"}
	spells := doc["entities"].(map[string]any)["spells"].([]any)
	slug := spells[0].(map[string]any)["slug"].(string)
	doc["provenance"] = map[string]any{"spells": map[string]any{slug: []string{"phb", "xge"}}}
	created := send(t, r, owner, "POST", "/v1/packs/import?title=Private", doc)
	if created.Code != 200 {
		t.Fatal(created.Body.String())
	}
	p := decode[packapi.Record](t, created)
	published := send(t, r, owner, "POST", "/v1/packs/"+p.ID+"/publish", map[string]any{"expectedRevision": p.Revision})
	if published.Code != 200 {
		t.Fatal(published.Body.String())
	}
	p = decode[packapi.Record](t, published)
	path := "/v1/packs/spells?pack=" + p.ID + "&source=" + p.ID + ":xge&limit=1"
	page := send(t, r, owner, "GET", path, nil)
	if page.Code != 200 || !strings.Contains(page.Body.String(), p.ID+"/"+slug) || !strings.Contains(page.Body.String(), "catalogPacks") {
		t.Fatalf("browse: %d %s", page.Code, page.Body)
	}
	if page.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private response is cacheable")
	}
	outside := send(t, r, outsider, "GET", path, nil)
	if strings.Contains(outside.Body.String(), p.ID+"/"+slug) {
		t.Fatal("private spell leaked")
	}
	filters := send(t, r, owner, "GET", "/v1/packs/spell-filters", nil)
	if !strings.Contains(filters.Body.String(), p.ID+":phb") || !strings.Contains(filters.Body.String(), p.ID+":xge") {
		t.Fatal(filters.Body.String())
	}
	for _, version := range []string{"1.9.0", "1.10.0"} {
		var draft map[string]any
		_ = json.Unmarshal(p.Draft, &draft)
		draft["manifest"].(map[string]any)["version"] = version
		saved := send(t, r, owner, "PUT", "/v1/packs/"+p.ID+"/draft", map[string]any{"title": p.Title, "document": draft, "expectedRevision": p.Revision})
		if saved.Code != 200 {
			t.Fatal(saved.Body.String())
		}
		p = decode[packapi.Record](t, saved)
		published = send(t, r, owner, "POST", "/v1/packs/"+p.ID+"/publish", map[string]any{"expectedRevision": p.Revision})
		if published.Code != 200 {
			t.Fatal(published.Body.String())
		}
		p = decode[packapi.Record](t, published)
	}
	latest := send(t, r, owner, "GET", path, nil)
	if !strings.Contains(latest.Body.String(), `"version":"1.10.0"`) {
		t.Fatal(latest.Body.String())
	}
	previous := send(t, r, owner, "GET", path+"&versions="+p.ID+"@1.9.0", nil)
	if !strings.Contains(previous.Body.String(), `"version":"1.9.0"`) {
		t.Fatal(previous.Body.String())
	}
	denied := send(t, r, outsider, "GET", path+"&versions="+p.ID+"@1.9.0", nil)
	if denied.Code != 404 {
		t.Fatal("private release override accepted")
	}
}

func TestZIPHTTPImportIsAtomicAndPreservesSources(t *testing.T) {
	r, owner, _, _ := newFullRouterInEnv(t, config.EnvDevelopment, true)
	before := send(t, r, owner, "GET", "/v1/packs", nil)
	var initial struct {
		Packs []packapi.Record `json:"packs"`
	}
	if err := json.Unmarshal(before.Body.Bytes(), &initial); err != nil {
		t.Fatal(err)
	}
	upload := func(data []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/v1/packs/import?title=Archive", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/zip")
		req.Header.Set("X-Request-Id", "zip-import-test")
		req.AddCookie(owner)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	if rec := upload([]byte("broken ZIP")); rec.Code != 400 {
		t.Fatalf("bad archive: %d", rec.Code)
	}
	after := send(t, r, owner, "GET", "/v1/packs", nil)
	var final struct {
		Packs []packapi.Record `json:"packs"`
	}
	_ = json.Unmarshal(after.Body.Bytes(), &final)
	if len(initial.Packs) != len(final.Packs) {
		t.Fatal("bad upload created a draft")
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	files := map[string]string{
		"pack/manifest.json":      `{"ruleset":"legacy catalogue index"}`,
		"pack/pack-manifest.json": `{"schemaVersion":1,"id":"example","version":"2.0.0","edition":"2014","semantics":"1","defaultLocale":"en","sources":{"phb":"Player's Handbook"},"files":{"locales/en/sources":"sources.json"}}`,
		"pack/sources.json":       `{"phb":{"name":"Player's Handbook"}}`,
	}
	for name, body := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	imported := upload(buf.Bytes())
	if imported.Code != 200 {
		t.Fatal(imported.Body.String())
	}
	record := decode[packapi.Record](t, imported)
	if record.ID == "example" || !strings.Contains(string(record.Draft), "Player's Handbook") || !strings.Contains(string(record.Draft), `"version":"1.0.0"`) {
		t.Fatal("ZIP did not follow normal independent draft import")
	}
}
