package profile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	"github.com/promix1722/easydnd/internal/api/http/middleware"
	"github.com/promix1722/easydnd/internal/domain/user"
	profileuc "github.com/promix1722/easydnd/internal/usecase/profile"
)

func TestProfileImageBoundary(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewUserRepository()
	alice, bob := repotest.Account("alice"), repotest.Account("bob")
	alice.Image, bob.Image = "old-alice", "old-bob"
	for _, account := range []user.User{alice, bob} {
		if err := repo.Create(ctx, account); err != nil {
			t.Fatal(err)
		}
	}
	handler := New(profileuc.NewService(repo))
	for _, test := range []struct {
		name, body string
		actor      user.User
		status     int
	}{
		{"missing session", `{"image":""}`, user.User{}, http.StatusUnauthorized},
		{"missing image", `{}`, alice, http.StatusBadRequest},
		{"null image", `{"image":null}`, alice, http.StatusBadRequest},
		{"wrong type", `{"image":42}`, alice, http.StatusBadRequest},
		{"invalid image", `{"image":"https://example.com/a.png"}`, alice, http.StatusBadRequest},
		{"oversized body", `{"image":"` + strings.Repeat("x", 301<<10) + `"}`, alice, http.StatusBadRequest},
		{"guest", `{"image":""}`, user.User{ID: "anon:guest", Anonymous: true}, http.StatusForbidden},
		{"actor ignores named victim", `{"image":"","user_id":"bob"}`, alice, http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			engine.PUT("/v1/profile/image", func(c *gin.Context) {
				if test.actor.ID != "" {
					middleware.SetUser(c, test.actor)
				}
				handler.PutImage(c)
			})
			request := httptest.NewRequest(http.MethodPut, "/v1/profile/image", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
			account, err := repo.ByID(ctx, bob.ID)
			if err != nil || account.Image != "old-bob" {
				t.Fatal("changed another account")
			}
			account, err = repo.ByID(ctx, alice.ID)
			expected := "old-alice"
			if test.status == http.StatusNoContent {
				expected = ""
			}
			if err != nil || account.Image != expected {
				t.Fatal("incorrect account mutation")
			}
		})
	}
}
