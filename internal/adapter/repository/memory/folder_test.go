package memory_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
)

// The assignment is the conformance proof: if the repository ever drifts from
// the port, this fails to compile rather than failing at wiring time.
var _ domain.FolderRepository = (*memory.FolderRepository)(nil)

func TestEnsureDefaultIsIdempotent(t *testing.T) {
	repo := memory.NewFolderRepository()
	ctx := context.Background()

	first, err := repo.EnsureDefault(ctx, owner)
	if err != nil {
		t.Fatalf("EnsureDefault() error = %v", err)
	}
	if !first.Default {
		t.Error("EnsureDefault() returned a folder that is not the default")
	}
	if first.Name != domain.DefaultFolderName {
		t.Errorf("EnsureDefault() name = %q, want %q", first.Name, domain.DefaultFolderName)
	}

	second, err := repo.EnsureDefault(ctx, owner)
	if err != nil {
		t.Fatalf("EnsureDefault() error = %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("EnsureDefault() issued a second default %q, want %q", second.ID, first.ID)
	}

	folders, err := repo.List(ctx, owner)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(folders) != 1 {
		t.Errorf("List() length = %d, want 1", len(folders))
	}
}

// The reason EnsureDefault is a repository method rather than a get-or-create
// in the usecase. Two requests arriving together for a new account must not
// each make a default folder: an account would be left with two folders it can
// never delete.
func TestEnsureDefaultIsSafeUnderConcurrentCallers(t *testing.T) {
	repo := memory.NewFolderRepository()
	ctx := context.Background()

	const callers = 16
	ids := make([]domain.FolderID, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f, err := repo.EnsureDefault(ctx, owner)
			if err != nil {
				t.Errorf("EnsureDefault() error = %v", err)
				return
			}
			ids[i] = f.ID
		}()
	}
	wg.Wait()

	folders, err := repo.List(ctx, owner)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(folders) != 1 {
		t.Fatalf("List() length = %d, want 1: concurrent callers made more than one default", len(folders))
	}
	for i, id := range ids {
		if id != folders[0].ID {
			t.Errorf("caller %d saw default %q, want %q", i, id, folders[0].ID)
		}
	}
}

func TestCreateIsNeverTheDefault(t *testing.T) {
	repo := memory.NewFolderRepository()
	ctx := context.Background()

	created, err := repo.Create(ctx, owner, "Campaign")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Default {
		t.Error("Create() made a default folder; only EnsureDefault may")
	}
	if created.Name != "Campaign" || created.Owner != owner {
		t.Errorf("Create() = %+v, want name Campaign owned by %q", created, owner)
	}
}

func TestFolderListPutsTheDefaultFirstAndFiltersByOwner(t *testing.T) {
	repo := memory.NewFolderRepository()
	ctx := context.Background()

	// Created before the default exists, so ordering cannot be an accident
	// of insertion order.
	if _, err := repo.Create(ctx, owner, "Retired"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	def, err := repo.EnsureDefault(ctx, owner)
	if err != nil {
		t.Fatalf("EnsureDefault() error = %v", err)
	}
	if _, err := repo.Create(ctx, "usr_2", "Somebody else's"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	folders, err := repo.List(ctx, owner)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("List() length = %d, want 2", len(folders))
	}
	if folders[0].ID != def.ID {
		t.Errorf("List()[0] = %q, want the default %q", folders[0].ID, def.ID)
	}
	if folders[1].Name != "Retired" {
		t.Errorf("List()[1] name = %q, want Retired", folders[1].Name)
	}
}

func TestFolderGetReportsNotFound(t *testing.T) {
	repo := memory.NewFolderRepository()

	if _, err := repo.Get(context.Background(), "fld_missing"); !types.IsNotFound(err) {
		t.Errorf("Get() error = %v, want a NotFoundError", err)
	}
}

// The default folder is renameable. What an account cannot lose is the folder,
// not the word on it.
func TestRenameLeavesTheDefaultFlagAlone(t *testing.T) {
	repo := memory.NewFolderRepository()
	ctx := context.Background()

	def, err := repo.EnsureDefault(ctx, owner)
	if err != nil {
		t.Fatalf("EnsureDefault() error = %v", err)
	}
	if err := repo.Rename(ctx, def.ID, "Active"); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	got, err := repo.Get(ctx, def.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != "Active" {
		t.Errorf("Get() name = %q, want Active", got.Name)
	}
	if !got.Default {
		t.Error("Rename() cleared the default flag")
	}
}

func TestRenameReportsNotFound(t *testing.T) {
	repo := memory.NewFolderRepository()

	if err := repo.Rename(context.Background(), "fld_missing", "x"); !types.IsNotFound(err) {
		t.Errorf("Rename() error = %v, want a NotFoundError", err)
	}
}

func TestDeleteRefusesTheDefaultFolder(t *testing.T) {
	repo := memory.NewFolderRepository()
	ctx := context.Background()

	def, err := repo.EnsureDefault(ctx, owner)
	if err != nil {
		t.Fatalf("EnsureDefault() error = %v", err)
	}

	err = repo.Delete(ctx, def.ID)
	var validation *types.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Delete() error = %v, want a ValidationError", err)
	}
	if _, err := repo.Get(ctx, def.ID); err != nil {
		t.Errorf("Get() after a refused Delete error = %v, want the folder still there", err)
	}
}

func TestDeleteRemovesANonDefaultFolder(t *testing.T) {
	repo := memory.NewFolderRepository()
	ctx := context.Background()

	created, err := repo.Create(ctx, owner, "Campaign")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Get(ctx, created.ID); !types.IsNotFound(err) {
		t.Errorf("Get() after Delete error = %v, want a NotFoundError", err)
	}
	if err := repo.Delete(ctx, created.ID); !types.IsNotFound(err) {
		t.Errorf("second Delete() error = %v, want a NotFoundError", err)
	}
}
