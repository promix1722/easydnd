package httpapi_test

import (
	"encoding/json"
	"net/http"
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
	created := send(t, r, owner, http.MethodPost, "/v1/packs/import?title=My%20rules", json.RawMessage(raw))
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
	unshared := send(t, r, owner, http.MethodDelete, "/v1/groups/"+g.ID+"/packs?pack="+p.ID, nil)
	if unshared.Code != 200 {
		t.Fatal(unshared.Body.String())
	}
	denied := send(t, r, outsider, http.MethodGet, "/v1/packs/catalog/spells?packs="+p.ID+"@1.0.0", nil)
	if denied.Code != 404 {
		t.Fatal("cached catalogue bypassed authorization")
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
