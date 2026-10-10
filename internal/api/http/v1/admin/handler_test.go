package admin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	"github.com/promix1722/easydnd/internal/api/http/middleware"
	adminapi "github.com/promix1722/easydnd/internal/api/http/v1/admin"
	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/user"
	adminuc "github.com/promix1722/easydnd/internal/usecase/admin"
)

// newEngine mounts the two routes the way the router does -- behind
// RequireSuperadmin -- with actor standing in for RequireSession.
func newEngine(t *testing.T, actor user.User) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	users := memory.NewUserRepository()
	characters := memory.NewCharacterRepository()
	for _, id := range []string{"alice", "bob"} {
		if err := users.Create(ctx, repotest.Account(id)); err != nil {
			t.Fatal(err)
		}
	}
	// Two of bob's, one of a guest who has no account row.
	for _, owner := range []string{"bob", "bob", "anon:ghost"} {
		if _, err := characters.Create(ctx, character.OwnerID(owner), ""); err != nil {
			t.Fatal(err)
		}
	}
	handler := adminapi.New(adminuc.NewService(users, characters))
	engine := gin.New()
	group := engine.Group("/v1/admin", func(c *gin.Context) {
		if actor.ID != "" {
			middleware.SetUser(c, actor)
		}
	}, middleware.RequireSuperadmin([]string{"alice", "root@example.test"}))
	group.GET("/players", handler.Players)
	group.GET("/characters", handler.Characters)
	return engine
}

func get(engine *gin.Engine, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	return response
}

func TestOnlyASuperadminReachesTheAdminRoutes(t *testing.T) {
	verified := user.User{ID: "carol", Identities: []user.Identity{{Email: "Root@example.test", EmailVerified: true}}}
	claimed := user.User{ID: "dave", Identities: []user.Identity{{Email: "root@example.test"}}}
	for _, test := range []struct {
		name   string
		actor  user.User
		status int
	}{
		{"superadmin by id", user.User{ID: "alice"}, http.StatusOK},
		{"superadmin by verified email", verified, http.StatusOK},
		{"unverified email", claimed, http.StatusNotFound},
		{"plain account", user.User{ID: "bob"}, http.StatusNotFound},
		{"guest", user.User{ID: "anon:guest", Anonymous: true}, http.StatusNotFound},
		{"no session", user.User{}, http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := newEngine(t, test.actor)
			for _, path := range []string{"/v1/admin/players", "/v1/admin/characters"} {
				if response := get(engine, path); response.Code != test.status {
					t.Fatalf("%s: status %d: %s", path, response.Code, response.Body.String())
				}
			}
		})
	}
}

func TestAdminListingsFilterAndPage(t *testing.T) {
	engine := newEngine(t, user.User{ID: "alice"})
	for _, test := range []struct {
		path   string
		status int
		rows   int
		total  int
	}{
		{"/v1/admin/players", http.StatusOK, 2, 2},
		{"/v1/admin/players?q=BO", http.StatusOK, 1, 1},
		{"/v1/admin/players?kind=guest", http.StatusOK, 0, 0},
		{"/v1/admin/players?limit=1&offset=1", http.StatusOK, 1, 2},
		{"/v1/admin/players?kind=robot", http.StatusBadRequest, 0, 0},
		{"/v1/admin/players?limit=0", http.StatusBadRequest, 0, 0},
		{"/v1/admin/characters", http.StatusOK, 3, 3},
		{"/v1/admin/characters?limit=2", http.StatusOK, 2, 3},
		{"/v1/admin/characters?owner=bob", http.StatusOK, 2, 2},
		{"/v1/admin/characters?owner=anon:ghost", http.StatusOK, 1, 1},
		{"/v1/admin/characters?owner=nobody", http.StatusOK, 0, 0},
		{"/v1/admin/characters?public=true", http.StatusOK, 0, 0},
		{"/v1/admin/characters?public=maybe", http.StatusBadRequest, 0, 0},
	} {
		response := get(engine, test.path)
		if response.Code != test.status {
			t.Fatalf("%s: status %d: %s", test.path, response.Code, response.Body.String())
		}
		if test.status != http.StatusOK {
			continue
		}
		var body struct {
			Players    []json.RawMessage `json:"players"`
			Characters []struct {
				Owner     string `json:"owner"`
				OwnerName string `json:"owner_name"`
			} `json:"characters"`
			Total int `json:"total"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: %v", test.path, err)
		}
		if rows := len(body.Players) + len(body.Characters); rows != test.rows || body.Total != test.total {
			t.Errorf("%s: %d rows of %d, want %d of %d", test.path, rows, body.Total, test.rows, test.total)
		}
		for _, c := range body.Characters {
			if want := map[string]string{"bob": "bob"}[c.Owner]; c.OwnerName != want {
				t.Errorf("%s: owner %q named %q, want %q", test.path, c.Owner, c.OwnerName, want)
			}
		}
	}
}
