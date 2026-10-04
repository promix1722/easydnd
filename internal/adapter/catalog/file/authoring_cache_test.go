package file

import (
	"bytes"
	"context"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/pack"
)

func TestAuthoringCacheRetainsAccessAndContentChecks(t *testing.T) {
	base, err := NewRegistry([]string{"../../../../data/srd_5.1"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	a := NewAuthoring(base, nil)
	docs := a.Builtins()[0].Releases
	roots := []pack.Release{docs[0].Release}
	lock, err := a.Resolve(context.Background(), docs, roots)
	if err != nil {
		t.Fatal(err)
	}
	first, err := a.LoadLocked(context.Background(), "en", lock)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Resolve(context.Background(), docs, roots); err != nil {
		t.Fatal(err)
	}
	second, err := a.LoadLocked(context.Background(), "en", lock)
	if err != nil || first != second {
		t.Fatal("recompiled an unchanged release", err)
	}
	// A compiled catalogue cannot supply permission or missing dependencies.
	if _, err := a.Resolve(context.Background(), nil, roots); err == nil {
		t.Fatal("cache bypassed available releases")
	}
	// Reusing a claimed digest with changed bytes must not reuse decoded content.
	forged := docs[0]
	forged.Data = bytes.Replace(forged.Data, []byte(`"title": "SRD 5.1"`), []byte(`"title": "Changed"`), 1)
	if bytes.Equal(forged.Data, docs[0].Data) {
		t.Fatal("fixture was not changed")
	}
	if _, err := a.Resolve(context.Background(), []pack.Document{forged}, roots); err == nil {
		t.Fatal("cache trusted a forged digest")
	}
	// Callers can sort or edit returned records without changing installed releases.
	docs[0].Data[0] = '!'
	docs[0].Release.Version = "99.0.0"
	clean := a.Builtins()[0].Releases
	if clean[0].Data[0] != '{' || clean[0].Release.Version == "99.0.0" {
		t.Fatal("caller mutated builtins")
	}
}

func BenchmarkAuthoringResolveInstalled(b *testing.B) {
	base, err := NewRegistry([]string{"../../../../data/srd_5.1"}, nil, "")
	if err != nil {
		b.Fatal(err)
	}
	a := NewAuthoring(base, nil)
	roots := a.Default().Packs
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := a.Resolve(context.Background(), a.Builtins()[0].Releases, roots); err != nil {
			b.Fatal(err)
		}
	}
}
