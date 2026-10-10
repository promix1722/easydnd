package postgres_test

import (
	"context"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/postgres"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	domain "github.com/promix1722/easydnd/internal/domain/character"
)

// TestCharacterRepository runs the shared port contract against a real
// database. Same skip as TestUserRepository.
func TestCharacterRepository(t *testing.T) {
	cfg := testConfig(t)

	repotest.RunCharacterRepository(t, func(t *testing.T) domain.Repository {
		pool := testPool(t, cfg)
		if _, err := pool.Exec(context.Background(), `TRUNCATE characters`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		return postgres.NewCharacterRepository(pool)
	})
}

// TestFolderRepository runs the folder contract against a real database.
func TestFolderRepository(t *testing.T) {
	cfg := testConfig(t)

	repotest.RunFolderRepository(t, func(t *testing.T) domain.FolderRepository {
		pool := testPool(t, cfg)
		if _, err := pool.Exec(context.Background(), `TRUNCATE folders`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		return postgres.NewFolderRepository(pool)
	})
}

// TestCharactersSurviveTheProcess is the whole reason the two stores above
// exist: a restart discards the process and everything it held, and a
// character and its folder must not go with it. Two pools stand in for two
// processes, as in TestAccountsSurviveTheProcess.
func TestCharactersSurviveTheProcess(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	before := testPool(t, cfg)
	if _, err := before.Exec(ctx, `TRUNCATE characters, folders`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	folder, err := postgres.NewFolderRepository(before).EnsureDefault(ctx, "usr_durable")
	if err != nil {
		t.Fatalf("EnsureDefault: %v", err)
	}
	chars := postgres.NewCharacterRepository(before)
	c, err := chars.Create(ctx, "usr_durable", folder.ID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := chars.Append(ctx, c.ID, 0, domain.Event{Type: domain.EventInit, Note: "kept"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	before.Close()

	after := testPool(t, cfg)
	got, err := postgres.NewCharacterRepository(after).Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get after the restart: %v", err)
	}
	if got.Folder != folder.ID || got.Log.Len() != 1 || got.Log.Events[0].Note != "kept" || got.Revision != 1 {
		t.Errorf("after the restart = %+v, want the character as written", got)
	}
	// And the next character does not reuse the id -- the property every
	// durable reference to a character depends on.
	again, err := postgres.NewCharacterRepository(after).Create(ctx, "usr_durable", folder.ID)
	if err != nil {
		t.Fatalf("Create after the restart: %v", err)
	}
	if again.ID == c.ID {
		t.Errorf("a restart handed out %q again", c.ID)
	}
}
