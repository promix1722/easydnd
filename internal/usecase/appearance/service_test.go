package appearance

import (
	"context"
	"errors"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

func TestAppearance(t *testing.T) {
	repo := memory.NewUserRepository()
	svc := NewService(repo)
	ctx := context.Background()
	account := repotest.Account("alice", "cred")
	if err := repo.Create(ctx, account); err != nil {
		t.Fatal(err)
	}
	for _, palette := range []string{"dragon", "parchment", "midnight", "moss"} {
		for _, scheme := range []string{"auto", "light", "dark"} {
			a := user.Appearance{Palette: palette, ColorScheme: scheme}
			saved, err := svc.Put(ctx, account, a)
			loaded, readErr := svc.Get(ctx, account)
			if readErr != nil || loaded != a {
				t.Fatalf("read: %v, %v", loaded, readErr)
			}
			if err != nil || saved != a {
				t.Fatalf("save: %v, %v", saved, err)
			}
		}
	}
	for _, a := range []user.Appearance{{}, {Palette: "invalid", ColorScheme: "auto"}, {Palette: "dragon", ColorScheme: "invalid"}, {Palette: "dragon"}} {
		_, err := svc.Put(ctx, account, a)
		var validation *types.ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("invalid preference accepted: %v", a)
		}
	}
	stored, err := repo.ByID(ctx, account.ID)
	if err != nil || stored.Appearance != (user.Appearance{Palette: "moss", ColorScheme: "dark"}) {
		t.Fatalf("invalid write changed stored value: %v, %v", stored.Appearance, err)
	}
	for _, guest := range []user.User{{ID: "anon:guest"}, {ID: "guest", Anonymous: true}} {
		_, err := svc.Put(ctx, guest, user.Appearance{Palette: "dragon", ColorScheme: "auto"})
		_, readErr := svc.Get(ctx, guest)
		var deniedRead *types.AccessDeniedError
		if !errors.As(readErr, &deniedRead) {
			t.Fatalf("guest read: %v", readErr)
		}
		var denied *types.AccessDeniedError
		if !errors.As(err, &denied) {
			t.Fatalf("guest write: %v", err)
		}
	}
}
