package profile

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	"github.com/promix1722/easydnd/internal/domain/user"
)

func TestProfileAvatar(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewUserRepository()
	account := repotest.Account("alice", "credential")
	other := repotest.Account("bob")
	for _, value := range []user.User{account, other} {
		if err := repo.Create(ctx, value); err != nil {
			t.Fatal(err)
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatal(err)
	}
	avatar := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
	svc := NewService(repo)
	if err := svc.PutImage(ctx, account, avatar); err != nil {
		t.Fatal(err)
	}
	if err := svc.PutImage(ctx, account, "https://example.com/avatar.svg"); err == nil {
		t.Fatal("invalid avatar accepted")
	}
	for _, guest := range []user.User{{ID: "anon:guest"}, {ID: account.ID, Anonymous: true}} {
		if err := svc.PutImage(ctx, guest, ""); err == nil {
			t.Fatal("guest changed an account avatar")
		}
	}
	loaded, err := repo.ByID(ctx, account.ID)
	if err != nil || loaded.Image != avatar || len(loaded.Credentials) != 1 {
		t.Fatalf("avatar did not persist or credentials changed: %v", err)
	}
	bob, err := repo.ByID(ctx, other.ID)
	if err != nil || bob.Image != "" {
		t.Fatal("another account changed")
	}
	if err := svc.PutImage(ctx, account, ""); err != nil {
		t.Fatal(err)
	}
	loaded, err = repo.ByID(ctx, account.ID)
	if err != nil || loaded.Image != "" {
		t.Fatal("avatar removal did not persist")
	}
}
