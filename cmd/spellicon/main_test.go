package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/promix1722/easydnd/internal/usecase/spellicon"
)

type fakeGenerator struct{ generate func() ([]byte, error) }

func (f fakeGenerator) Generate(context.Context, string) ([]byte, error) { return f.generate() }

func TestOutputAndOverwriteProtection(t *testing.T) {
	out := filepath.Join(t.TempDir(), "rune.webp")
	calls := 0
	create := func(key string, opts spellicon.Options) generator {
		calls++
		if key != "secret" || opts.Model != "chosen-model" {
			t.Fatal("wrong configuration")
		}
		return fakeGenerator{func() ([]byte, error) { return []byte("complete image"), nil }}
	}
	args := []string{"-prompt", "rune", "-out", out, "-model", "chosen-model"}
	if err := run(context.Background(), args, "secret", io.Discard, create); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	if string(data) != "complete image" {
		t.Fatal("wrong output")
	}
	if err := run(context.Background(), args, "secret", io.Discard, create); err == nil || calls != 1 {
		t.Fatal("overwrote existing image")
	}
	if err := run(context.Background(), append(args, "-overwrite"), "secret", io.Discard, create); err != nil {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(filepath.Dir(out))
	if len(files) != 1 {
		t.Fatal("left temporary files")
	}
}

func TestFailureRetainsOldOutput(t *testing.T) {
	out := filepath.Join(t.TempDir(), "rune.webp")
	_ = os.WriteFile(out, []byte("old"), 0644)
	create := func(string, spellicon.Options) generator {
		return fakeGenerator{func() ([]byte, error) { return nil, errors.New("provider failed") }}
	}
	err := run(context.Background(), []string{"-prompt", "rune", "-out", out, "-overwrite"}, "secret", io.Discard, create)
	if err == nil {
		t.Fatal("expected failure")
	}
	data, _ := os.ReadFile(out)
	if string(data) != "old" {
		t.Fatal("damaged old file")
	}
	files, _ := os.ReadDir(filepath.Dir(out))
	if len(files) != 1 {
		t.Fatal("left temporary files")
	}
}

func TestInvalidInputDoesNotGenerate(t *testing.T) {
	out := filepath.Join(t.TempDir(), "icon.webp")
	create := func(string, spellicon.Options) generator { t.Fatal("generated for invalid input"); return nil }
	for _, args := range [][]string{{}, {"-prompt", "rune"}, {"-out", out}, {"-prompt", "rune", "-out", out, "-timeout", "0"}, {"-prompt", "rune", "-out", out, "-model", " "}} {
		if err := run(context.Background(), args, "secret", io.Discard, create); err == nil {
			t.Fatal("accepted invalid args")
		}
	}
	if err := run(context.Background(), []string{"-prompt", "rune", "-out", out}, "", io.Discard, create); err == nil {
		t.Fatal("accepted missing key")
	}
}

func TestDestinationRaceDoesNotOverwrite(t *testing.T) {
	out := filepath.Join(t.TempDir(), "icon.webp")
	create := func(string, spellicon.Options) generator {
		return fakeGenerator{func() ([]byte, error) {
			_ = os.WriteFile(out, []byte("other process"), 0644)
			return []byte("generated"), nil
		}}
	}
	if err := run(context.Background(), []string{"-prompt", "rune", "-out", out}, "secret", io.Discard, create); err == nil {
		t.Fatal("overwrote racing output")
	}
	data, _ := os.ReadFile(out)
	if string(data) != "other process" {
		t.Fatal("damaged racing output")
	}
}
