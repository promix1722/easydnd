package repotest

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
)

// NewFolderRepository builds an empty folder store for one subtest.
type NewFolderRepository func(t *testing.T) domain.FolderRepository

// RunFolderRepository runs the whole port contract against one
// implementation.
func RunFolderRepository(t *testing.T, newRepo NewFolderRepository) {
	t.Helper()

	tests := []struct {
		name string
		run  func(t *testing.T, repo domain.FolderRepository)
	}{
		{
			name: "ensure default is idempotent",
			run: func(t *testing.T, repo domain.FolderRepository) {
				ctx := context.Background()
				first, err := repo.EnsureDefault(ctx, charOwner)
				if err != nil {
					t.Fatalf("EnsureDefault() error = %v", err)
				}
				if !first.Default {
					t.Error("EnsureDefault() returned a folder that is not the default")
				}
				if first.Name != domain.DefaultFolderName {
					t.Errorf("EnsureDefault() name = %q, want %q", first.Name, domain.DefaultFolderName)
				}
				second, err := repo.EnsureDefault(ctx, charOwner)
				if err != nil {
					t.Fatalf("EnsureDefault() error = %v", err)
				}
				if second.ID != first.ID {
					t.Errorf("EnsureDefault() issued a second default %q, want %q", second.ID, first.ID)
				}
				folders, err := repo.List(ctx, charOwner)
				if err != nil {
					t.Fatalf("List() error = %v", err)
				}
				if len(folders) != 1 {
					t.Errorf("List() length = %d, want 1", len(folders))
				}
			},
		},
		{
			// The reason EnsureDefault is a repository method rather than a
			// get-or-create in the usecase: two requests arriving together
			// for a new account must not each make a default folder.
			name: "ensure default is safe under concurrent callers",
			run: func(t *testing.T, repo domain.FolderRepository) {
				ctx := context.Background()
				const callers = 16
				ids := make([]domain.FolderID, callers)
				var wg sync.WaitGroup
				for i := range callers {
					wg.Add(1)
					go func() {
						defer wg.Done()
						f, err := repo.EnsureDefault(ctx, charOwner)
						if err != nil {
							t.Errorf("EnsureDefault() error = %v", err)
							return
						}
						ids[i] = f.ID
					}()
				}
				wg.Wait()
				folders, err := repo.List(ctx, charOwner)
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
			},
		},
		{
			name: "create is never the default",
			run: func(t *testing.T, repo domain.FolderRepository) {
				created, err := repo.Create(context.Background(), charOwner, "Campaign")
				if err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				if created.Default {
					t.Error("Create() made a default folder; only EnsureDefault may")
				}
				if created.Name != "Campaign" || created.Owner != charOwner || created.ID == "" {
					t.Errorf("Create() = %+v, want name Campaign owned by %q with an id", created, charOwner)
				}
			},
		},
		{
			name: "list puts the default first and filters by owner",
			run: func(t *testing.T, repo domain.FolderRepository) {
				ctx := context.Background()
				// Created before the default exists, so ordering cannot be
				// an accident of insertion order.
				if _, err := repo.Create(ctx, charOwner, "Retired"); err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				def, err := repo.EnsureDefault(ctx, charOwner)
				if err != nil {
					t.Fatalf("EnsureDefault() error = %v", err)
				}
				if _, err := repo.Create(ctx, "usr_2", "Somebody else's"); err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				folders, err := repo.List(ctx, charOwner)
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
				none, err := repo.List(ctx, "usr_nobody")
				if err != nil {
					t.Fatalf("List() error = %v", err)
				}
				if none == nil || len(none) != 0 {
					t.Errorf("List() for nobody = %v, want an empty, non-nil slice", none)
				}
			},
		},
		{
			name: "get reports not found",
			run: func(t *testing.T, repo domain.FolderRepository) {
				if _, err := repo.Get(context.Background(), "fld_missing"); !types.IsNotFound(err) {
					t.Errorf("Get() error = %v, want a NotFoundError", err)
				}
			},
		},
		{
			// The default folder is renameable. What an account cannot lose
			// is the folder, not the word on it.
			name: "rename leaves the default flag alone",
			run: func(t *testing.T, repo domain.FolderRepository) {
				ctx := context.Background()
				def, err := repo.EnsureDefault(ctx, charOwner)
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
				if err := repo.Rename(ctx, "fld_missing", "x"); !types.IsNotFound(err) {
					t.Errorf("Rename() error = %v, want a NotFoundError", err)
				}
			},
		},
		{
			name: "delete refuses the default folder",
			run: func(t *testing.T, repo domain.FolderRepository) {
				ctx := context.Background()
				def, err := repo.EnsureDefault(ctx, charOwner)
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
			},
		},
		{
			name: "delete removes a non-default folder",
			run: func(t *testing.T, repo domain.FolderRepository) {
				ctx := context.Background()
				created, err := repo.Create(ctx, charOwner, "Campaign")
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
			},
		},
		{
			name: "reorder rewrites the whole run",
			run: func(t *testing.T, repo domain.FolderRepository) {
				ctx := context.Background()
				def, first, second := threeFolders(t, repo)

				// A new folder lands last, before anybody has said otherwise.
				if got := folderNames(t, repo); got != "Default,Tuesday game,Retired" {
					t.Errorf("List() before = %q, want Default,Tuesday game,Retired", got)
				}
				if err := repo.Reorder(ctx, charOwner, []domain.FolderID{second, first}); err != nil {
					t.Fatalf("Reorder() error = %v", err)
				}
				// The default did not move, and it was never named.
				if got := folderNames(t, repo); got != "Default,Retired,Tuesday game" {
					t.Errorf("List() after = %q, want Default,Retired,Tuesday game", got)
				}
				folders, err := repo.List(ctx, charOwner)
				if err != nil {
					t.Fatalf("List() error = %v", err)
				}
				if folders[0].ID != def {
					t.Errorf("List()[0] = %q, want the default %q", folders[0].ID, def)
				}
				// Sending it again is the same order, not a rotation of it:
				// this is what lets a client re-send a drag it is unsure
				// landed.
				if err := repo.Reorder(ctx, charOwner, []domain.FolderID{second, first}); err != nil {
					t.Fatalf("Reorder() twice error = %v", err)
				}
				if got := folderNames(t, repo); got != "Default,Retired,Tuesday game" {
					t.Errorf("List() after twice = %q, want Default,Retired,Tuesday game", got)
				}
			},
		},
		{
			// The set has to match exactly. Each of these leaves a folder
			// without a decided position, which is the state List cannot
			// render honestly.
			name: "reorder refuses an incomplete or foreign set",
			run: func(t *testing.T, repo domain.FolderRepository) {
				ctx := context.Background()
				_, first, second := threeFolders(t, repo)
				theirs, err := repo.Create(ctx, "usr_2", "Somebody else's")
				if err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				cases := map[string][]domain.FolderID{
					"one missing":     {first},
					"one too many":    {first, second, theirs.ID},
					"one repeated":    {first, first},
					"somebody else's": {first, theirs.ID},
					"empty":           {},
				}
				for name, ids := range cases {
					t.Run(name, func(t *testing.T) {
						err := repo.Reorder(ctx, charOwner, ids)
						var validation *types.ValidationError
						if !errors.As(err, &validation) {
							t.Fatalf("Reorder() error = %v, want a ValidationError", err)
						}
					})
				}
				// And none of the refusals moved anything.
				if got := folderNames(t, repo); got != "Default,Tuesday game,Retired" {
					t.Errorf("List() = %q, want the untouched Default,Tuesday game,Retired", got)
				}
			},
		},
		{
			// A folder made after a reorder goes to the end, rather than
			// colliding with whatever position it was handed at creation.
			name: "reorder then create puts the new folder last",
			run: func(t *testing.T, repo domain.FolderRepository) {
				ctx := context.Background()
				_, first, second := threeFolders(t, repo)
				if err := repo.Reorder(ctx, charOwner, []domain.FolderID{second, first}); err != nil {
					t.Fatalf("Reorder() error = %v", err)
				}
				if _, err := repo.Create(ctx, charOwner, "New"); err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				if got := folderNames(t, repo); got != "Default,Retired,Tuesday game,New" {
					t.Errorf("List() = %q, want Default,Retired,Tuesday game,New", got)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, newRepo(t))
		})
	}
}

// threeFolders seeds the default folder, then "Tuesday game" and "Retired",
// and returns their ids in that order.
func threeFolders(t *testing.T, repo domain.FolderRepository) (def, first, second domain.FolderID) {
	t.Helper()
	ctx := context.Background()
	d, err := repo.EnsureDefault(ctx, charOwner)
	if err != nil {
		t.Fatalf("EnsureDefault() error = %v", err)
	}
	f, err := repo.Create(ctx, charOwner, "Tuesday game")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	s, err := repo.Create(ctx, charOwner, "Retired")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return d.ID, f.ID, s.ID
}

// folderNames joins the owner's listing by name, so an order assertion reads
// as one string rather than as four index comparisons.
func folderNames(t *testing.T, repo domain.FolderRepository) string {
	t.Helper()
	folders, err := repo.List(context.Background(), charOwner)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	out := make([]string, 0, len(folders))
	for _, f := range folders {
		out = append(out, f.Name)
	}
	return strings.Join(out, ",")
}
