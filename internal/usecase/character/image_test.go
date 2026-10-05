package character_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

func portraitPNG(t *testing.T, width, height int) string {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewNRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
}

func TestPortraitLifecycle(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	portrait := portraitPNG(t, 256, 256)
	c, err := s.Create(ctx, testOwner, "", charuc.NewCharacter{Name: "Hero", Image: portrait})
	if err != nil {
		t.Fatal(err)
	}
	check := func(id domain.ID, expected string) {
		t.Helper()
		state, err := s.Sheet(ctx, testOwner, id, rules.DefaultLocale)
		if err != nil || state.Identity.Image != expected {
			t.Fatalf("sheet portrait missing: %v", err)
		}
		list, err := s.List(ctx, testOwner, "", rules.DefaultLocale)
		if err != nil {
			t.Fatal(err)
		}
		for _, summary := range list {
			if summary.ID == id && summary.Image == expected {
				return
			}
		}
		t.Fatal("list portrait missing")
	}
	check(c.ID, portrait)
	if _, err := s.Sheet(ctx, "another-owner", c.ID, rules.DefaultLocale); err == nil {
		t.Fatal("portrait readable by another owner")
	}
	edit := func(path, value string) {
		t.Helper()
		stored, err := s.Get(ctx, testOwner, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		event := domain.Event{Type: domain.EventInit, Changes: []domain.Change{{Path: domain.Path(path), Op: domain.OpSet, Value: domain.StringValue(value)}}}
		_, err = s.Revise(charuc.WithRevision(ctx, stored.Revision), testOwner, c.ID, rules.DefaultLocale, stored.Log.LastSeq(), 1, &event, true)
		if err != nil {
			t.Fatal(err)
		}
	}
	edit("identity.name", "Renamed")
	check(c.ID, portrait)
	replacement := portraitPNG(t, 32, 32)
	edit("identity.image", replacement)
	check(c.ID, replacement)
	stale := domain.Event{Type: domain.EventInit, Changes: []domain.Change{{Path: "identity.image", Op: domain.OpSet, Value: domain.StringValue(portrait)}}}
	if _, err := s.Revise(charuc.WithRevision(ctx, c.Revision), testOwner, c.ID, rules.DefaultLocale, c.Log.LastSeq(), 1, &stale, true); err == nil {
		t.Fatal("stale revision overwrote portrait")
	}
	check(c.ID, replacement)
	copy, err := s.CopyCharacter(ctx, testOwner, c.ID, "", rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	check(copy.ID, replacement)
	edit("identity.image", "")
	check(c.ID, "")
	check(copy.ID, replacement)
}

func TestRejectInvalidPortraitsWithoutChangingCharacter(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	c := mustCreate(t, s)
	valid := portraitPNG(t, 16, 16)
	invalid := []string{
		"https://example.com/portrait.png", "data:image/svg+xml;base64,PHN2Zz4=",
		"data:image/png;base64,broken", "data:image/png;base64,",
		strings.Replace(valid, "image/png", "image/jpeg", 1),
		portraitPNG(t, 257, 1), portraitPNG(t, 1, 257),
		"data:image/png;base64," + strings.Repeat("A", 256<<10),
		valid[:len(valid)-12],
	}
	for i, value := range invalid {
		if _, err := s.Create(ctx, testOwner, "", charuc.NewCharacter{Name: "Bad", Image: value}); err == nil {
			t.Fatalf("creation accepted invalid portrait %d", i)
		}
		event := domain.Event{Type: domain.EventInit, Changes: []domain.Change{{Path: "identity.image", Op: domain.OpSet, Value: domain.StringValue(value)}}}
		if _, err := s.Revise(charuc.WithRevision(ctx, c.Revision), testOwner, c.ID, rules.DefaultLocale, c.Log.LastSeq(), 1, &event, true); err == nil {
			t.Fatalf("revision accepted invalid portrait %d", i)
		}
	}
	stored, err := s.Get(ctx, testOwner, c.ID)
	if err != nil || stored.Revision != c.Revision || stored.Log.LastSeq() != c.Log.LastSeq() {
		t.Fatalf("invalid portrait changed stored character: %v", err)
	}
}

func TestPortraitJPEGAndWebP(t *testing.T) {
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, image.NewRGBA(image.Rect(0, 0, 16, 16)), nil); err != nil {
		t.Fatal(err)
	}
	webpBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "data", "srd_5.1", "spell-icons", "magic-missile.webp"))
	if err != nil {
		t.Fatal(err)
	}
	s := newService(t)
	for format, data := range map[string][]byte{"jpeg": jpegBytes.Bytes(), "webp": webpBytes} {
		value := "data:image/" + format + ";base64," + base64.StdEncoding.EncodeToString(data)
		c, err := s.Create(context.Background(), testOwner, "", charuc.NewCharacter{Name: "Hero", Image: value})
		if err != nil {
			t.Fatalf("%s portrait rejected: %v", format, err)
		}
		state, err := s.Sheet(context.Background(), testOwner, c.ID, rules.DefaultLocale)
		if err != nil || state.Identity.Image != value {
			t.Fatalf("%s portrait lost: %v", format, err)
		}
	}
}
