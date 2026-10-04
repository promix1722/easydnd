package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/promix1722/easydnd/internal/usecase/spellicon"
)

// spellPromptsCmd keeps offline batches on the service's art direction.
func spellPromptsCmd(args []string) error {
	fs := flag.NewFlagSet("llm spell-prompts", flag.ExitOnError)
	in := fs.String("in", "data/srd_5.1", "data pack directory")
	out := fs.String("out", "", "output JSON file mapping spell slugs to prompts")
	fs.Parse(args)
	if *out == "" {
		return fmt.Errorf("spell-prompts: -out is required")
	}
	data, err := os.ReadFile(filepath.Join(*in, "spells.json"))
	if err != nil {
		return err
	}
	var spells []struct{ Slug, School string }
	if err := json.Unmarshal(data, &spells); err != nil {
		return err
	}
	data, err = os.ReadFile(filepath.Join(*in, "i18n", "en", "spells.json"))
	if err != nil {
		return err
	}
	var prose map[string]struct{ Name string }
	if err := json.Unmarshal(data, &prose); err != nil {
		return err
	}
	prompts := make(map[string]string, len(spells))
	for _, spell := range spells {
		name := prose[spell.Slug].Name
		if name == "" {
			name = spell.Slug
		}
		prompts[spell.Slug] = spellicon.Prompt(name, spell.School)
	}
	data, err = json.MarshalIndent(prompts, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		return err
	}
	log.Printf("wrote %d prompts to %s", len(prompts), *out)
	return nil
}
