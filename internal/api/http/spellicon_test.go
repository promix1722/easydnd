package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/sheet/hexsheet"
	"github.com/promix1722/easydnd/internal/adapter/token"
	httpapi "github.com/promix1722/easydnd/internal/api/http"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	authapi "github.com/promix1722/easydnd/internal/api/http/v1/auth"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
	folderapi "github.com/promix1722/easydnd/internal/api/http/v1/folder"
	gameapi "github.com/promix1722/easydnd/internal/api/http/v1/game"
	groupapi "github.com/promix1722/easydnd/internal/api/http/v1/group"
	packapi "github.com/promix1722/easydnd/internal/api/http/v1/pack"
	spelliconapi "github.com/promix1722/easydnd/internal/api/http/v1/spellicon"
	"github.com/promix1722/easydnd/internal/api/http/v1/system"
	"github.com/promix1722/easydnd/internal/config"
	"github.com/promix1722/easydnd/internal/domain/imageasset"
	"github.com/promix1722/easydnd/internal/types"
	authuc "github.com/promix1722/easydnd/internal/usecase/auth"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
	gameuc "github.com/promix1722/easydnd/internal/usecase/game"
	groupuc "github.com/promix1722/easydnd/internal/usecase/group"
	packuc "github.com/promix1722/easydnd/internal/usecase/pack"
	spelliconuc "github.com/promix1722/easydnd/internal/usecase/spellicon"
)

// The wire shape the frontend's IconGenerationState declares. Decoded here
// rather than imported because the usecase's State is the canonical type --
// this struct exists to pin the field names the client reads.
type iconJob struct {
	State    string `json:"state"`
	Reason   string `json:"reason,omitempty"`
	Revision string `json:"revision,omitempty"`
}
type iconState struct {
	Configured bool               `json:"configured"`
	Running    bool               `json:"running"`
	Total      int                `json:"total"`
	Completed  int                `json:"completed"`
	Skipped    int                `json:"skipped"`
	Failed     int                `json:"failed"`
	Items      map[string]iconJob `json:"items"`
}

type testImageReader struct {
	slug  string
	image imageasset.Image
}

func (r testImageReader) Image(_ context.Context, slug string) (imageasset.Image, error) {
	if slug != r.slug {
		return imageasset.Image{}, types.NewNotFoundError("spell icon not found")
	}
	return r.image.Clone(), nil
}

// iconStore is the in-memory stand-in for the artwork store: Existing answers
// from a preset map, Save records the slug it was asked to publish. The real
// store writes WebP files; the handler tests only need to see which qualified
// slugs reached it.
type iconStore struct {
	mu       sync.Mutex
	existing map[string]string
	saved    []string
}

func (s *iconStore) Existing(_ context.Context, slug string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	revision, ok := s.existing[slug]
	return revision, ok, nil
}
func (s *iconStore) Save(_ context.Context, slug string, _ []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saved = append(s.saved, slug)
	return "rev-" + slug, nil
}

// instantGenerator answers every prompt with a real (if tiny) PNG, so a run
// completes on its own without timing games.
type instantGenerator struct{ png []byte }

func (g instantGenerator) Generate(_ context.Context, _ string) ([]byte, error) {
	return g.png, nil
}

// blockedGenerator holds Generate open until released, which is what a
// "run already active" test needs to keep the first job in flight.
type blockedGenerator struct {
	png     []byte
	release chan struct{}
}

func (g *blockedGenerator) Generate(ctx context.Context, _ string) ([]byte, error) {
	select {
	case <-g.release:
		return g.png, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// newIconRouter builds the full route table exactly as newFullRouterInEnv
// does, plus the development spell-icon handler wired the way internal/app
// wires it. It exists alongside that helper rather than inside it because the
// generator is the thing under test -- every other caller wants the
// production-like nil.
func newIconRouter(
	t *testing.T,
	env string,
	generator spelliconuc.Generator,
) (*gin.Engine, *http.Cookie, *iconStore, *packuc.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Env:  env,
		HTTP: config.HTTPConfig{TrustedProxies: []string{"127.0.0.1", "::1"}},
		Auth: config.AuthConfig{
			RPID:            "easydnd.test",
			RPDisplayName:   "easydnd",
			RPOrigins:       []string{testOrigin},
			SessionSecret:   []byte("0123456789abcdef0123456789abcdef"),
			SessionTTL:      time.Hour,
			GuestSessionTTL: 15 * time.Minute,
			CeremonyTTL:     5 * time.Minute,
			SecureCookies:   false,
		},
	}
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))

	users := memory.NewUserRepository()
	ceremony := &stubCeremony{}
	signer := token.NewSigner(cfg.Auth.SessionSecret, cfg.Auth.SessionTTL)
	authService := authuc.NewService(users, ceremony, signer, nil, authuc.Config{
		SessionTTL:      cfg.Auth.SessionTTL,
		GuestSessionTTL: cfg.Auth.GuestSessionTTL,
		CeremonyTTL:     cfg.Auth.CeremonyTTL,
	}, log)
	cookies := helpers.NewCookieOptions(cfg)

	base, err := catalogfile.NewRegistry([]string{"../../../data/srd_5.1"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	packRepo := memory.NewPackRepository()
	source := catalogfile.NewAuthoring(base, packRepo)
	groupRepo := memory.NewGroupRepository(users)
	packs := packuc.NewService(packRepo, source, groupRepo, users)

	characterRepo := memory.NewCharacterRepository()
	gameService := gameuc.NewService(
		memory.NewGameRepository(), memory.NewSharedRepository(),
		groupRepo, characterRepo, source, log)
	characterService := charuc.NewService(characterRepo,
		memory.NewFolderRepository(), source, hexsheet.NewImporter(), gameService, log)
	characterService.SetPackAccess(packs)
	groupService := groupuc.NewService(groupRepo, users, signer, gameService, log)

	store := &iconStore{existing: map[string]string{}}
	icons := spelliconuc.NewService(generator, store, 10, time.Minute, log)
	t.Cleanup(icons.Close)

	r, err := httpapi.NewRouter(cfg, log, httpapi.Handlers{
		System:        system.New(testVersion),
		Version:       testVersion,
		Auth:          authapi.New(authService, cookies),
		Authenticator: authService,
		Pack:          packapi.New(packs, source),
		Catalog:       catalogapi.New(source, log),
		Character:     characterapi.New(characterService, log),
		Folder:        folderapi.New(characterService, log),
		Game:          gameapi.New(gameService, log),
		Group:         groupapi.New(groupService, log),
		SpellIcons:    spelliconapi.New(icons, spelliconuc.NewSelector(packs, source)),
		SpellImages: spelliconapi.NewImages(testImageReader{
			slug: "srd-2014/fireball",
			image: imageasset.Image{
				Slug: "srd-2014/fireball", Revision: "revision-1",
				ContentType: "image/webp", Data: []byte("webp-bytes"),
			},
		}),
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return r, register(t, r, cookies), store, packs
}

// waitIcons polls the queue until it stops running, the way the screen's own
// poller does, and fails the test if a finished state never arrives.
func waitIcons(t *testing.T, r *gin.Engine, session *http.Cookie) iconState {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		rec := send(t, r, session, http.MethodGet, "/v1/dev/spell-icons", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("queue status = %d: %s", rec.Code, rec.Body)
		}
		state := decode[iconState](t, rec)
		if !state.Running {
			return state
		}
		if time.Now().After(deadline) {
			t.Fatalf("icon run never finished: %s", rec.Body)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// publishPack imports and publishes the example pack over HTTP and returns
// its record, so every assertion runs against the qualified slugs the real
// importer assigns.
func publishPack(t *testing.T, r *gin.Engine, session *http.Cookie) packapi.Record {
	t.Helper()
	raw, err := os.ReadFile("../../../data/packs/examples/tactician.json")
	if err != nil {
		t.Fatal(err)
	}
	created := send(t, r, session, http.MethodPost, "/v1/packs/import?title=Tactician", json.RawMessage(raw))
	if created.Code != http.StatusOK {
		t.Fatal(created.Body.String())
	}
	p := decode[packapi.Record](t, created)
	published := send(t, r, session, http.MethodPost, "/v1/packs/"+p.ID+"/publish",
		map[string]any{"expectedRevision": p.Revision})
	if published.Code != http.StatusOK {
		t.Fatal(published.Body.String())
	}
	return decode[packapi.Record](t, published)
}

// The initial GET is the state a freshly loaded screen restores: nothing
// running, and whether a provider is configured, as a flag rather than an
// error -- the POST is where the missing key is reported.
func TestSpellIconsGetReportsInitialState(t *testing.T) {
	r, session, _, _ := newIconRouter(t, config.EnvDevelopment, nil)

	rec := send(t, r, session, http.MethodGet, "/v1/dev/spell-icons", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("queue state is cacheable")
	}
	state := decode[iconState](t, rec)
	if state.Configured || state.Running || state.Total != 0 || len(state.Items) != 0 {
		t.Fatalf("initial state = %+v", state)
	}
}

// Both verbs sit behind RequireSession like every other resource route: a
// generator that spent money on an unsigned request would be a bug the
// anonymous-session test suite is right to notice.
func TestSpellIconsRequireSession(t *testing.T) {
	r, _, _, _ := newIconRouter(t, config.EnvDevelopment, instantGenerator{tinyPNG(t)})

	if rec := send(t, r, nil, http.MethodGet, "/v1/dev/spell-icons", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned GET = %d, want 401", rec.Code)
	}
	if rec := send(t, r, nil, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope": "browse", "search": map[string]any{}, "replace": false,
	}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned POST = %d, want 401", rec.Code)
	}
}

// The SameOrigin group middleware is the second lock on a state-changing
// request, and this route must not sit outside it: a browser-origin POST with
// a foreign Origin is refused before the handler runs.
func TestSpellIconsPostRejectsForeignOrigin(t *testing.T) {
	r, session, _, _ := newIconRouter(t, config.EnvDevelopment, instantGenerator{tinyPNG(t)})

	rec := send(t, r, session, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope": "browse", "search": map[string]any{}, "replace": false,
	}, map[string]string{"Origin": "https://evil.example"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign-origin POST = %d: %s", rec.Code, rec.Body)
	}

	// A POST with no X-Request-Id at all -- what an HTML form could send --
	// is refused on the same grounds.
	req := httptest.NewRequest(http.MethodPost, "/v1/dev/spell-icons",
		strings.NewReader(`{"scope":"browse","search":{},"replace":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", testOrigin)
	req.AddCookie(session)
	bare := httptest.NewRecorder()
	r.ServeHTTP(bare, req)
	if bare.Code != http.StatusForbidden {
		t.Fatalf("form-shaped POST = %d: %s", bare.Code, bare.Body)
	}
}

// The scope allowlist is the boundary that keeps a POST from naming an
// arbitrary internal route: anything that is not "browse", "" or an exact
// /packs/catalog URL is a 400, as are malformed selectors inside it.
func TestSpellIconsPostRejectsMalformedRequests(t *testing.T) {
	r, session, _, _ := newIconRouter(t, config.EnvDevelopment, instantGenerator{tinyPNG(t)})

	cases := []map[string]any{
		{"search": map[string]any{}, "replace": false},
		{"scope": "browse", "search": nil, "replace": false},
		{"scope": "/v1/packs", "search": map[string]any{}, "replace": false},
		{"scope": "/packs/catalog", "search": map[string]any{}, "replace": false},
		{"scope": "/packs/catalog?packs=home-abc", "search": map[string]any{}, "replace": false},
		{"scope": "/packs/catalog?packs=a@1.0.0&x=y", "search": map[string]any{}, "replace": false},
		{"scope": "browse", "search": map[string]any{"level": 99}, "replace": false},
		{"scope": "browse", "search": map[string]any{"limit": 0}, "replace": false},
		{"scope": "browse", "search": map[string]any{}, "slug": "../etc/passwd", "replace": false},
		{"scope": "browse", "search": map[string]any{}, "slug": "not a slug", "replace": false},
		{"scope": "browse", "search": map[string]any{}, "slug": "no-such-spell", "replace": false},
		{"scope": "browse", "search": map[string]any{}, "slugs": []any{}, "replace": false},
		{"scope": "browse", "search": map[string]any{}, "slugs": nil, "replace": false},
		{"scope": "browse", "search": map[string]any{}, "slugs": []any{"not a slug"}, "replace": false},
		{"scope": "browse", "search": map[string]any{}, "slugs": []any{"../etc/passwd"}, "replace": false},
		{"scope": "browse", "search": map[string]any{}, "slugs": "fireball", "replace": false},
		{"scope": "browse", "search": map[string]any{}, "slug": "fireball", "slugs": []any{"acid-splash"}, "replace": false},
	}
	for _, body := range cases {
		rec := send(t, r, session, http.MethodPost, "/v1/dev/spell-icons", body)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Fatalf("%v = %d: %s", body, rec.Code, rec.Body)
		}
	}
}

// A scoped POST selects only what the caller may see: the pack's own spells
// land in the queue under their qualified slugs, and another account's POST
// of the same scope is refused at the resolver, not by a check duplicated
// here.
func TestSpellIconsPostSelectsAuthorizedSpells(t *testing.T) {
	r, owner, store, _ := newIconRouter(t, config.EnvDevelopment, instantGenerator{tinyPNG(t)})
	p := publishPack(t, r, owner)

	scope := "/packs/catalog?packs=" + p.ID + "@" + p.Releases[0].Version
	rec := send(t, r, owner, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope":   scope,
		"search":  map[string]any{"pack": p.ID},
		"replace": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST = %d: %s", rec.Code, rec.Body)
	}
	state := waitIcons(t, r, owner)
	if state.Total == 0 || state.Failed != 0 {
		t.Fatalf("run state = %+v", state)
	}
	for slug, job := range state.Items {
		if !strings.HasPrefix(slug, p.ID+"/") {
			t.Fatalf("unqualified item key %q", slug)
		}
		if job.State != "done" || job.Revision == "" {
			t.Fatalf("job %s = %+v", slug, job)
		}
	}
	store.mu.Lock()
	saved := len(store.saved)
	store.mu.Unlock()
	if saved != state.Total {
		t.Fatalf("store saved %d icons for %d spells", saved, state.Total)
	}

	// Another account's queue cannot reach into Alice's pack through a
	// spelled-out scope either.
	outside := guest(t, r, helpers.CookieOptions{Secure: false})
	denied := send(t, r, outside, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope":   scope,
		"search":  map[string]any{},
		"replace": true,
	})
	if denied.Code != http.StatusNotFound {
		t.Fatalf("foreign pack scope = %d: %s", denied.Code, denied.Body)
	}

	// And browse narrows the same way: the qualified slug resolves for the
	// owner but does not exist in the guest's catalogue.
	row := send(t, r, owner, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope": "browse", "search": map[string]any{},
		"slug": p.ID + "/guiding-mark", "replace": true,
	})
	if row.Code != http.StatusOK {
		t.Fatalf("owner row POST = %d: %s", row.Code, row.Body)
	}
	done := waitIcons(t, r, owner)
	if done.Items[p.ID+"/guiding-mark"].State != "done" {
		t.Fatalf("row job = %+v", done.Items)
	}
	refused := send(t, r, outside, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope": "browse", "search": map[string]any{},
		"slug": p.ID + "/guiding-mark", "replace": true,
	})
	if refused.Code == http.StatusOK {
		t.Fatal("guest generated artwork for a private pack's spell")
	}
}

// Page controls belong to the interactive catalogue query, not to a paid
// generation run. Even when the body carries a one-row window, the backend
// queues every spell satisfying the filter.
func TestSpellIconsIgnoresPageWindow(t *testing.T) {
	r, session, _, _ := newIconRouter(t, config.EnvDevelopment, instantGenerator{tinyPNG(t)})

	rec := send(t, r, session, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope": "browse",
		"search": map[string]any{
			"pack": "srd-2014", "limit": 1, "offset": 1,
		},
		"replace": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST = %d: %s", rec.Code, rec.Body)
	}
	state := decode[iconState](t, rec)
	if state.Total < 300 {
		t.Fatalf("one-page request queued only %d spells", state.Total)
	}
	finished := waitIcons(t, r, session)
	if finished.Completed != finished.Total || finished.Failed != 0 {
		t.Fatalf("batch state = %+v", finished)
	}
}

// A named spell goes straight to the queue: the filter the screen happens to
// hold must not veto the row the user clicked.
func TestSpellIconsSlugRunsIndependentOfFilter(t *testing.T) {
	r, session, store, _ := newIconRouter(t, config.EnvDevelopment, instantGenerator{tinyPNG(t)})

	rec := send(t, r, session, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope":   "browse",
		"search":  map[string]any{"q": "zz-no-such-name"},
		"slug":    "fireball",
		"replace": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST = %d: %s", rec.Code, rec.Body)
	}
	state := waitIcons(t, r, session)
	job, ok := state.Items["fireball"]
	if !ok || job.State != "done" {
		t.Fatalf("fireball job = %+v", state.Items)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.saved) != 1 || store.saved[0] != "fireball" {
		t.Fatalf("saved = %v", store.saved)
	}
}

// The slugs form is the resume path: exactly the listed spells, in order,
// nothing else the scope could match -- and a repeated name is queued and
// saved once, so a duplicated entry cannot double a paid generation.
func TestSpellIconsSlugsRunExplicitSet(t *testing.T) {
	r, session, store, _ := newIconRouter(t, config.EnvDevelopment, instantGenerator{tinyPNG(t)})

	rec := send(t, r, session, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope":   "browse",
		"search":  map[string]any{"q": "zz-no-such-name"},
		"slugs":   []string{"acid-splash", "fireball", "acid-splash"},
		"replace": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST = %d: %s", rec.Code, rec.Body)
	}
	accepted := decode[iconState](t, rec)
	if accepted.Total != 2 || len(accepted.Items) != 2 {
		t.Fatalf("explicit set queued %+v", accepted.Items)
	}
	state := waitIcons(t, r, session)
	for _, slug := range []string{"acid-splash", "fireball"} {
		if job := state.Items[slug]; job.State != "done" {
			t.Fatalf("%s job = %+v", slug, job)
		}
	}
	store.mu.Lock()
	saved := slices.Clone(store.saved)
	store.mu.Unlock()
	slices.Sort(saved)
	if !slices.Equal(saved, []string{"acid-splash", "fireball"}) {
		t.Fatalf("saved = %v", saved)
	}

	// The filtered bulk path the screen normally sends is unchanged: a run
	// without slugs still queues every spell the filter matches.
	bulk := send(t, r, session, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope": "browse", "search": map[string]any{"level": 0}, "replace": true,
	})
	if bulk.Code != http.StatusOK {
		t.Fatalf("bulk POST = %d: %s", bulk.Code, bulk.Body)
	}
	bulkState := waitIcons(t, r, session)
	if bulkState.Total <= 2 || bulkState.Items["acid-splash"].State != "done" {
		t.Fatalf("filtered run = %+v", bulkState)
	}
}

// One name in the list that the scope cannot hold -- a misspelling or
// another account's private pack spell -- fails the whole request: nothing
// is queued and nothing is paid for, rather than generating the reachable
// subset.
func TestSpellIconsSlugsRejectWholeRequest(t *testing.T) {
	r, session, store, _ := newIconRouter(t, config.EnvDevelopment, instantGenerator{tinyPNG(t)})

	rec := send(t, r, session, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope":   "browse",
		"search":  map[string]any{},
		"slugs":   []string{"fireball", "no-such-spell"},
		"replace": true,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown-in-list POST = %d: %s", rec.Code, rec.Body)
	}

	// A private pack's qualified slug is just another unresolvable name to
	// an outsider: listed alongside a public spell it still sinks the batch.
	p := publishPack(t, r, session)
	outside := guest(t, r, helpers.CookieOptions{Secure: false})
	denied := send(t, r, outside, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope":   "browse",
		"search":  map[string]any{},
		"slugs":   []string{"fireball", p.ID + "/guiding-mark"},
		"replace": true,
	})
	if denied.Code == http.StatusOK {
		t.Fatal("guest generated artwork alongside a private pack's spell")
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.saved) != 0 {
		t.Fatalf("rejected requests still saved %v", store.saved)
	}
}

// One run at a time is the cost control: while a generator holds a job open,
// a second POST is refused with the reason the client maps rather than
// silently queued twice.
func TestSpellIconsRejectsConcurrentRuns(t *testing.T) {
	release := make(chan struct{})
	r, session, _, _ := newIconRouter(t, config.EnvDevelopment,
		&blockedGenerator{png: tinyPNG(t), release: release})
	t.Cleanup(func() { close(release) })

	first := send(t, r, session, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope": "browse", "search": map[string]any{}, "slug": "fireball", "replace": true,
	})
	if first.Code != http.StatusOK {
		t.Fatalf("first POST = %d: %s", first.Code, first.Body)
	}
	// The job only leaves "running" when the generator answers, so a running
	// state here is also proof the response did not wait on paid work.
	if state := decode[iconState](t, first); !state.Running {
		t.Fatalf("accepted run not running: %+v", state)
	}
	// Queue status is global only in the running flag: another owner must
	// not see the first account's spell slugs or per-item progress.
	outsider := guest(t, r, helpers.CookieOptions{Secure: false})
	private := send(t, r, outsider, http.MethodGet, "/v1/dev/spell-icons", nil)
	if private.Code != http.StatusOK {
		t.Fatalf("other owner's GET = %d: %s", private.Code, private.Body)
	}
	other := decode[iconState](t, private)
	if !other.Running || other.Total != 0 || len(other.Items) != 0 {
		t.Fatalf("another owner saw queue contents: %+v", other)
	}

	second := send(t, r, session, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope": "browse", "search": map[string]any{}, "slug": "acid-splash", "replace": true,
	})
	if second.Code != http.StatusConflict {
		t.Fatalf("concurrent run = %d, want 409: %s", second.Code, second.Body)
	}
	var body struct {
		Error struct {
			Reason string `json:"reason"`
		} `json:"error"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Reason != "icon_generation_busy" {
		t.Fatalf("busy reason = %q: %s", body.Error.Reason, second.Body)
	}
}

// Without a generator, a row that already has artwork still succeeds and is
// recorded as skipped. The missing key is only a problem if work would call the
// provider.
func TestSpellIconsExistingArtNeedsNoProvider(t *testing.T) {
	r, session, store, _ := newIconRouter(t, config.EnvDevelopment, nil)
	store.mu.Lock()
	store.existing["fireball"] = "existing-revision"
	store.mu.Unlock()

	rec := send(t, r, session, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope": "browse", "search": map[string]any{}, "slug": "fireball", "replace": false,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("existing-art POST = %d: %s", rec.Code, rec.Body)
	}
	state := waitIcons(t, r, session)
	job := state.Items["fireball"]
	if state.Configured || state.Total != 1 || state.Skipped != 1 ||
		state.Completed != 1 || job.State != "skipped" || job.Reason != "icon_generation_exists" ||
		job.Revision != "existing-revision" {
		t.Fatalf("existing-art state = %+v", state)
	}
}

// With no provider configured the POST reports it as a refusal, not a fake
// success -- the same answer the Vite middleware gave.
func TestSpellIconsReportsUnconfiguredProvider(t *testing.T) {
	r, session, _, _ := newIconRouter(t, config.EnvDevelopment, nil)

	rec := send(t, r, session, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope": "browse", "search": map[string]any{}, "slug": "fireball", "replace": true,
	})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured run = %d, want 503: %s", rec.Code, rec.Body)
	}
	var body struct {
		Error struct {
			Reason string `json:"reason"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Reason != "icon_generation_not_configured" {
		t.Fatalf("reason = %q: %s", body.Error.Reason, rec.Body)
	}
}

// The whole of the gate: in production the route does not exist, rather than
// existing and declining -- the same contract the character stub carries.
func TestSpellIconsAbsentInProduction(t *testing.T) {
	r, session, _, _ := newIconRouter(t, config.EnvProduction, instantGenerator{tinyPNG(t)})

	if rec := send(t, r, session, http.MethodGet, "/v1/dev/spell-icons", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("production GET = %d: %s", rec.Code, rec.Body)
	}
	if rec := send(t, r, session, http.MethodPost, "/v1/dev/spell-icons", map[string]any{
		"scope": "browse", "search": map[string]any{}, "replace": false,
	}); rec.Code != http.StatusNotFound {
		t.Fatalf("production POST = %d: %s", rec.Code, rec.Body)
	}
	// The artwork is different: it is public seed content, so even an
	// unauthenticated production request can fetch a WebP and revalidate it.
	image := send(t, r, nil, http.MethodGet, "/v1/spell-icons/srd-2014/fireball.webp", nil)
	if image.Code != http.StatusOK {
		t.Fatalf("production public image = %d: %s", image.Code, image.Body)
	}
	if image.Header().Get("Cache-Control") != "no-cache" ||
		image.Header().Get("ETag") != `"revision-1"` ||
		image.Body.String() != "webp-bytes" {
		t.Fatalf("image headers/body = %q, %q, %q",
			image.Header().Get("Cache-Control"), image.Header().Get("ETag"), image.Body.String())
	}
}
