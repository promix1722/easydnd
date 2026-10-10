package pack_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	file "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/user"
	uc "github.com/promix1722/easydnd/internal/usecase/pack"
)

func fixture(t *testing.T) (context.Context, *uc.Service, *file.Authoring, *memory.PackRepository, *memory.GroupRepository) {
	t.Helper()
	ctx := context.Background()
	base, err := file.NewRegistry([]string{"../../../data/pack/srd-5.1"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	repo := memory.NewPackRepository()
	users := memory.NewUserRepository()
	groups := memory.NewGroupRepository(users)
	engine := file.NewAuthoring(base, repo)
	return ctx, uc.NewService(repo, engine, groups, users), engine, repo, groups
}
func create(t *testing.T, ctx context.Context, s *uc.Service, who string) pack.Record {
	t.Helper()
	r, err := s.Create(ctx, user.User{ID: user.ID(who), Anonymous: true, DisplayName: who}, "Test pack", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func publish(t *testing.T, ctx context.Context, s *uc.Service, r pack.Record) pack.Record {
	t.Helper()
	r, err := s.Publish(ctx, r.Owner, r.ID, r.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestDraftReleasesAndImportIsolation(t *testing.T) {
	ctx, s, engine, repo, _ := fixture(t)
	r := create(t, ctx, s, "anon:alice")
	r = publish(t, ctx, s, r)
	first := r.Releases[0]
	lock, err := s.Resolve(ctx, r.Owner, []pack.Release{first.Release})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Resolve(ctx, "anon:bob", []pack.Release{first.Release}); err == nil {
		t.Fatal("private release resolved")
	}
	if _, err = s.Export(ctx, "anon:bob", r.ID, "1.0.0"); err == nil {
		t.Fatal("private release exported")
	}
	if err = s.AuthorizeLock(ctx, "anon:bob", lock, pack.Lock{}); err == nil {
		t.Fatal("forged lock accepted")
	}
	if _, err = s.Publish(ctx, r.Owner, r.ID, r.Revision); err == nil {
		t.Fatal("immutable release overwritten")
	}
	var doc map[string]any
	_ = json.Unmarshal(r.Draft, &doc)
	doc["manifest"].(map[string]any)["version"] = "1.1.0"
	b, _ := json.Marshal(doc)
	newer, err := s.Save(ctx, r.Owner, r.ID, r.Title, r.Revision, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Save(ctx, r.Owner, r.ID, r.Title, r.Revision, b, nil); err == nil {
		t.Fatal("stale save succeeded")
	}
	newer = publish(t, ctx, s, newer)
	if newer.Releases[0].Release != first.Release {
		t.Fatal("old release changed")
	}
	// A fresh adapter can reconstruct pinned contexts from the repository.
	base, err := file.NewRegistry([]string{"../../../data/pack/srd-5.1"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	fresh := file.NewAuthoring(base, repo)
	if _, err = fresh.LoadLocked(ctx, "en", lock); err != nil {
		t.Fatal(err)
	}
	copy, err := s.Create(ctx, user.User{ID: "anon:bob", Anonymous: true, DisplayName: "Bob"}, "Imported", first.Data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if copy.ID == r.ID {
		t.Fatal("import retained identity")
	}
	copy = publish(t, ctx, s, copy)
	if copy.Releases[0].Release.Version != "1.0.0" {
		t.Fatal("incorrect initial version")
	}
	if _, err = engine.LoadLocked(ctx, "en", lock); err != nil {
		t.Fatal(err)
	}
}
func TestSharingAndRetainedCharacterAccess(t *testing.T) {
	ctx, s, _, _, groups := fixture(t)
	r := publish(t, ctx, s, create(t, ctx, s, "anon:alice"))
	_ = create(t, ctx, s, "anon:bob")
	_ = create(t, ctx, s, "anon:carol")
	if err := groups.Create(ctx, group.Group{ID: "table", Name: "Table", CreatedAt: time.Now()}, "anon:carol"); err != nil {
		t.Fatal(err)
	}
	for _, u := range []user.ID{"anon:alice", "anon:bob"} {
		if err := groups.AddMember(ctx, "table", u, group.RolePlayer, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Share(ctx, r.Owner, "table", r.ID, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	lock, err := s.Resolve(ctx, "anon:bob", []pack.Release{r.Releases[0].Release})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Unshare(ctx, "anon:bob", "table", r.ID); err == nil {
		t.Fatal("player removed somebody else's share")
	}
	if err = s.Unshare(ctx, "anon:carol", "table", r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Resolve(ctx, "anon:bob", []pack.Release{r.Releases[0].Release}); err == nil {
		t.Fatal("unshared release selectable")
	}
	if err = s.AuthorizeLock(ctx, "anon:bob", lock, lock); err != nil {
		t.Fatalf("existing character lost access: %v", err)
	}
	if err = s.AuthorizeLock(ctx, "anon:bob", lock, pack.Lock{}); err == nil {
		t.Fatal("new character accepted removed release")
	}
}
func TestInvalidDraftCanSaveButNotPublish(t *testing.T) {
	ctx, s, _, _, _ := fixture(t)
	r := create(t, ctx, s, "anon:alice")
	var doc map[string]any
	_ = json.Unmarshal(r.Draft, &doc)
	doc["manifest"].(map[string]any)["dependencies"] = []any{map[string]any{"id": "missing", "version": "1.0.0"}}
	b, _ := json.Marshal(doc)
	r, err := s.Save(ctx, r.Owner, r.ID, r.Title, r.Revision, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Publish(ctx, r.Owner, r.ID, r.Revision); err == nil {
		t.Fatal("missing dependency published")
	}
	if _, err = s.Create(ctx, user.User{ID: r.Owner}, "Bad", []byte(`{"manifest":{},"manifest":{}}`), nil); err == nil {
		t.Fatal("duplicate key accepted")
	}
}

// overlayDir writes a descriptions-only overlay pack where a private folder
// can be pointed at it.
func overlayDir(t *testing.T) string {
	t.Helper()
	doc, err := file.EncodePack(&file.PackDocument{
		Manifest: file.PackManifest{SchemaVersion: 1, ID: "overlay", Version: "1.0.0", Edition: "2014", Semantics: "1", DefaultLocale: "en",
			Dependencies: []file.Dependency{{ID: "srd-2014", Version: ">=1.0.0"}}},
		Entities: map[string]json.RawMessage{},
		Locales:  map[string]map[string]file.Bundle{"en": {"classes": {"srd-2014:class:wizard": {Desc: []string{"A scholar."}}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "pack"), doc, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// restrictedFixture installs a descriptions-only overlay as a private pack and
// names root as the superadmin, by verified email.
func restrictedFixture(t *testing.T) (context.Context, *uc.Service, *memory.GroupRepository, pack.Release) {
	t.Helper()
	ctx := context.Background()
	dir := overlayDir(t)
	base, err := file.NewRegistry([]string{"../../../data/pack/srd-5.1"}, nil, "", file.PackFolder{Path: dir, Restricted: true})
	if err != nil {
		t.Fatal(err)
	}
	repo := memory.NewPackRepository()
	users := memory.NewUserRepository()
	groups := memory.NewGroupRepository(users)
	engine := file.NewAuthoring(base, repo)
	if len(engine.Default().Packs) != 1 {
		t.Fatalf("Default() = %v, want the base alone: a private pack is never a default root", engine.Default().Packs)
	}
	for id, identities := range map[user.ID][]user.Identity{
		"root":    {{Provider: user.ProviderGoogle, Subject: "1", Email: "Root@example.com", EmailVerified: true}},
		"claimed": {{Provider: user.ProviderGoogle, Subject: "2", Email: "root@example.com"}},
		"player":  nil,
	} {
		if err = users.Create(ctx, user.User{ID: id, DisplayName: string(id), Identities: identities}); err != nil {
			t.Fatal(err)
		}
	}
	if err = groups.Create(ctx, group.Group{ID: "table", Name: "Table", CreatedAt: time.Now()}, "root"); err != nil {
		t.Fatal(err)
	}
	if err = groups.AddMember(ctx, "table", "player", group.RolePlayer, time.Now()); err != nil {
		t.Fatal(err)
	}
	s := uc.NewService(repo, engine, groups, users)
	s.SetSuperadmins([]string{"root@example.com"})
	for _, r := range engine.Builtins() {
		if r.ID == "overlay" {
			return ctx, s, groups, r.Releases[0].Release
		}
	}
	t.Fatal("private pack not installed")
	return nil, nil, nil, pack.Release{}
}

func TestRestrictedPackIsTheSuperadminsUntilGranted(t *testing.T) {
	ctx, s, _, overlay := restrictedFixture(t)
	roots := []pack.Release{overlay}

	if _, err := s.Resolve(ctx, "root", roots); err != nil {
		t.Fatalf("superadmin cannot select the private pack: %v", err)
	}
	// An unverified email is anybody's to type; a guest has no account at all.
	for _, who := range []user.ID{"player", "claimed", "anon:guest"} {
		if _, err := s.Resolve(ctx, who, roots); err == nil {
			t.Errorf("%s selected the private pack", who)
		}
		if _, err := s.Export(ctx, who, "overlay", "1.0.0"); err == nil {
			t.Errorf("%s exported the private pack", who)
		}
		rows, err := s.List(ctx, who)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.ID == "overlay" {
				t.Errorf("%s is shown the private pack", who)
			}
		}
	}

	// Only a superadmin grants it, and a grant reaches the table's members.
	if err := s.Share(ctx, "player", "table", "overlay", "1.0.0"); err == nil {
		t.Fatal("a player shared the private pack")
	}
	if err := s.Share(ctx, "root", "table", "overlay", "1.0.0"); err != nil {
		t.Fatalf("superadmin cannot grant the private pack: %v", err)
	}
	if _, err := s.Resolve(ctx, "player", roots); err != nil {
		t.Fatalf("a member of the granted group cannot select it: %v", err)
	}
	if _, err := s.Resolve(ctx, "claimed", roots); err == nil {
		t.Fatal("an account outside the group selected it")
	}
	// A grant is to play with the pack, not to take a copy of it.
	if _, err := s.Export(ctx, "player", "overlay", "1.0.0"); err == nil {
		t.Fatal("a granted member exported the private pack")
	}
	if _, err := s.Export(ctx, "root", "overlay", "1.0.0"); err != nil {
		t.Fatalf("superadmin cannot export it: %v", err)
	}
	if err := s.Unshare(ctx, "root", "table", "overlay"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(ctx, "player", roots); err == nil {
		t.Fatal("the grant outlived its removal")
	}
}

// A member who was granted the pack may build on it, but may not carry it to
// another table inside a pack of their own.
func TestGrantedRestrictedPackCannotBeResharedThroughAHomebrewPack(t *testing.T) {
	ctx, s, groups, overlay := restrictedFixture(t)
	if err := s.Share(ctx, "root", "table", "overlay", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := groups.Create(ctx, group.Group{ID: "elsewhere", Name: "Elsewhere", CreatedAt: time.Now()}, "player"); err != nil {
		t.Fatal(err)
	}
	draft, err := json.Marshal(file.PackDocument{
		Manifest: file.PackManifest{SchemaVersion: 1, ID: "mine", Version: "1.0.0", Edition: "2014", Semantics: "1", DefaultLocale: "en",
			Dependencies: []file.Dependency{{ID: "srd-2014", Version: ">=1.0.0"}, {ID: overlay.ID, Version: overlay.Version}}},
		Entities: map[string]json.RawMessage{},
		Locales:  map[string]map[string]file.Bundle{"en": {}},
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Create(ctx, user.User{ID: "player", DisplayName: "player"}, "Mine", draft, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r, err = s.Publish(ctx, "player", r.ID, r.Revision); err != nil {
		t.Fatal(err)
	}
	err = s.Share(ctx, "player", "elsewhere", r.ID, "1.0.0")
	if err == nil || !strings.Contains(err.Error(), "dependency cannot be shared") {
		t.Fatalf("a granted private pack left its table inside a homebrew pack: %v", err)
	}
}
