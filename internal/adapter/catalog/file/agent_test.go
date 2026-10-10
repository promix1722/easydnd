package file_test

import (
	"context"
	"strings"
	"testing"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

func TestPrivateDefinitionsArePinnedAndNotGlobal(t *testing.T) {
	t.Parallel()
	r := freshRegistry(t)
	base := r.DefaultLock()
	body := []byte(`{"entities":{"feats":[{"slug":"star-touched"}]},"locales":{"en":{"feats":{"star-touched":{"name":"Star Touched","desc":["Source text"]}}}}}`)
	cat, err := r.CompilePrivate(context.Background(), base, "testsession", body, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	if !cat.Feats.Has("import-testsession/star-touched") {
		t.Fatal("custom feat absent")
	}
	global, err := r.Load(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	if global.Feats.Has("import-testsession/star-touched") {
		t.Fatal("private option leaked into default catalogue")
	}
	oldLock := cat.Lock.Clone()
	changed := []byte(strings.ReplaceAll(string(body), "Star Touched", "Moon Touched"))
	next, err := r.CompilePrivate(context.Background(), cat.Lock, "testsession", changed, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	if next.Lock.Equal(oldLock) {
		t.Fatal("edited custom definition reused immutable version")
	}
	old, err := r.LoadLocked(context.Background(), rules.DefaultLocale, oldLock)
	if err != nil {
		t.Fatal(err)
	}
	feat, _ := old.Feats.Get("import-testsession/star-touched")
	if feat.Name != "Star Touched" {
		t.Fatal("changed an older character's definition")
	}
	other, err := r.CompilePrivate(context.Background(), base, "other", body, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	if other.Feats.Has("import-testsession/star-touched") {
		t.Fatal("definition escaped session")
	}
}
func TestPrivateDefinitionsRejectCoreAndForeignIdentity(t *testing.T) {
	t.Parallel()
	r := freshRegistry(t)
	for _, body := range []string{`{"entities":{"feats":[{"slug":"srd-2014:feat:grappler"}]},"locales":{"en":{}}}`, `{"entities":{},"locales":{"en":{}},"mechanics":{"core":{}}}`} {
		if _, err := r.CompilePrivate(context.Background(), r.DefaultLock(), "test", []byte(body), rules.DefaultLocale); err == nil {
			t.Fatalf("accepted unsafe definition: %s", body)
		}
	}
}

func TestAuthoringRetainsPrivateImportDefinitions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := catalogfile.NewAuthoring(registry(t), memory.NewPackRepository())
	body := []byte(`{"entities":{"feats":[{"slug":"star-touched"}]},"locales":{"en":{"feats":{"star-touched":{"name":"Star Touched","desc":["Source text"]}}}}}`)
	cat, err := a.CompilePrivate(ctx, a.Default(), "private", body, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	oldLock := cat.Lock.Clone()
	changed := []byte(strings.ReplaceAll(string(body), "Star Touched", "Moon Touched"))
	if _, err := a.CompilePrivate(ctx, oldLock, "private", changed, rules.DefaultLocale); err != nil {
		t.Fatal(err)
	}
	old, err := a.LoadLocked(ctx, rules.DefaultLocale, oldLock)
	if err != nil {
		t.Fatal(err)
	}
	feat, ok := old.Feats.Get("import-private/star-touched")
	if !ok || feat.Name != "Star Touched" {
		t.Fatal("private pinned definition lost")
	}
	if _, err := a.LoadLocked(ctx, rules.Locale("ru"), oldLock); err != nil {
		t.Fatal(err)
	}
	for _, record := range a.Builtins() {
		if strings.HasPrefix(record.ID, "import-") {
			t.Fatal("private import leaked into library")
		}
	}
	global, err := a.Load(ctx, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	if global.Feats.Has("import-private/star-touched") {
		t.Fatal("private import leaked into default catalogue")
	}
	retained := a.PrivateReleases(context.Background(), oldLock)
	if len(retained.Packs) != 1 || retained.Packs[0].ID != "import-private" {
		t.Fatal("copy must retain only private definitions")
	}
}
