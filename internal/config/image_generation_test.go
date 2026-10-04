package config

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestImageGenerationYAMLDoesNotUseProviderEnvironment(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "must-not-be-used-by-service")
	cfg := loadOK(t, "env: development\nimage_generation:\n  model: artwork-model\n  output_dir: /tmp/artwork\n  cache_dir: /tmp/artwork-cache\n  request_timeout: 3m\n")
	if cfg.ImageGeneration.APIKey != "" {
		t.Fatal("service inherited a provider credential outside its YAML")
	}
}

func TestImageGenerationRejectsInvalidTimeout(t *testing.T) {
	for _, value := range []string{"-1s", "11m", "not-a-duration"} {
		t.Run(value, func(t *testing.T) {
			err := loadErr(t, "env: development\nimage_generation:\n  request_timeout: "+value+"\n", "with invalid image generation timeout")
			if !strings.Contains(err.Error(), "image_generation.request_timeout") {
				t.Fatalf("wrong setting reported: %v", err)
			}
		})
	}
}

func TestImageGenerationWorkersDefaultsAndBounds(t *testing.T) {
	// Omitted means the queue runs at its full width.
	cfg := loadOK(t, "env: development\n")
	if cfg.ImageGeneration.Workers != 10 {
		t.Fatalf("omitted workers must default to 10, got %d", cfg.ImageGeneration.Workers)
	}
	for _, value := range []int{1, 4, 10} {
		cfg := loadOK(t, "env: development\nimage_generation:\n  workers: "+strconv.Itoa(value)+"\n")
		if cfg.ImageGeneration.Workers != value {
			t.Fatalf("workers: %d came back %d", value, cfg.ImageGeneration.Workers)
		}
	}
	// The pool is deliberately narrow -- every worker is a paid request in
	// flight. Zero falls through to the default like any absent int key;
	// negative and above-bound values are misconfiguration.
	for _, value := range []int{-1, -10, 11, 100} {
		t.Run(strconv.Itoa(value), func(t *testing.T) {
			err := loadErr(t, "env: development\nimage_generation:\n  workers: "+strconv.Itoa(value)+"\n", "with out-of-range workers")
			if !strings.Contains(err.Error(), "image_generation.workers") {
				t.Fatalf("wrong setting reported: %v", err)
			}
		})
	}
}

func TestImageGenerationRejectsNegativeRequestRate(t *testing.T) {
	err := loadErr(t, "env: development\nimage_generation:\n  requests_per_minute: -1\n", "with a negative provider rate")
	if !strings.Contains(err.Error(), "image_generation.requests_per_minute") {
		t.Fatalf("wrong setting reported: %v", err)
	}
}

func TestImageGenerationPackDirsLoad(t *testing.T) {
	cfg := loadOK(t, "env: development\nimage_generation:\n  output_dir: /tmp/srd-icons\n  pack_dirs:\n    dnd-2014: /srv/packs/dnd-2014/spell-icons\n    home-9: /srv/packs/home-9\n")
	want := map[string]string{
		"dnd-2014": "/srv/packs/dnd-2014/spell-icons",
		"home-9":   "/srv/packs/home-9",
	}
	if len(cfg.ImageGeneration.PackDirs) != len(want) {
		t.Fatalf("want %v, got %v", want, cfg.ImageGeneration.PackDirs)
	}
	for k, v := range want {
		if cfg.ImageGeneration.PackDirs[k] != v {
			t.Fatalf("pack_dirs[%q]: want %q, got %q", k, v, cfg.ImageGeneration.PackDirs[k])
		}
	}
}

func TestImageGenerationPackDirsOmitted(t *testing.T) {
	cfg := loadOK(t, "env: development\nimage_generation:\n  output_dir: /tmp/srd-icons\n")
	if len(cfg.ImageGeneration.PackDirs) != 0 {
		t.Fatalf("absent pack_dirs must stay empty, got %v", cfg.ImageGeneration.PackDirs)
	}
}

func TestImageGenerationPackDirsRejectBadKeys(t *testing.T) {
	for _, key := range []string{"", "DND-2014", "dnd_2014", "-dnd", "srd-2014"} {
		t.Run(key, func(t *testing.T) {
			loadErr(t, "env: development\nimage_generation:\n  pack_dirs:\n    \""+key+"\": /srv/packs/x\n", "with an invalid pack_dirs key")
		})
	}
}

func TestImageGenerationPackDirsRejectBlankPath(t *testing.T) {
	err := loadErr(t, "env: development\nimage_generation:\n  pack_dirs:\n    dnd-2014: \" \"\n", "with a blank pack directory")
	if !strings.Contains(err.Error(), "pack_dirs") {
		t.Fatalf("wrong setting reported: %v", err)
	}
}

func TestImageGenerationPackDirsRejectOverlap(t *testing.T) {
	base := t.TempDir()
	for name, body := range map[string]string{
		"pack root equals output_dir": "  output_dir: " + base + "\n  pack_dirs:\n    dnd-2014: " + base + "\n",
		"pack root inside output_dir": "  output_dir: " + base + "\n  pack_dirs:\n    dnd-2014: " + filepath.Join(base, "dnd-2014") + "\n",
		"output_dir inside pack root": "  output_dir: " + filepath.Join(base, "icons") + "\n  pack_dirs:\n    dnd-2014: " + base + "\n",
		"two packs share a root":      "  pack_dirs:\n    a-pack: " + base + "\n    b-pack: " + base + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, "env: development\nimage_generation:\n"+body, "with overlapping image roots")
			if !strings.Contains(err.Error(), "overlap") {
				t.Fatalf("overlap must be named in the error: %v", err)
			}
		})
	}
}
