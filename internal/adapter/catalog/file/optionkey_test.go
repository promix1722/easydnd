package file_test

import (
	"testing"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// walk visits a choice and every choice nested inside it.
func walk(c rules.Choice, visit func(rules.Choice)) {
	if c.Prompt.IsZero() {
		return
	}
	visit(c)
	for _, option := range c.From.Options {
		switch opt := option.(type) {
		case rules.NestedOption:
			walk(opt.Choice, visit)
		case rules.BundleOption:
			// A bundle can contain a nested choice -- the rogue's "one skill
			// plus thieves' tools" Expertise branch is exactly that. Missing
			// this is what let a duplicate prompt id go unnoticed.
			for _, item := range opt.Items {
				if nested, ok := item.(rules.NestedOption); ok {
					walk(nested.Choice, visit)
				}
			}
		}
	}
}

// walkPtr is walk for the optional choices, which the catalogue models as
// pointers because "this race poses no ability-bonus prompt" and "it poses an
// empty one" are different things.
func walkPtr(c *rules.Choice, visit func(rules.Choice)) {
	if c == nil {
		return
	}
	walk(*c, visit)
}

// walkAll is walk over a slice of choices.
func walkAll(cs []rules.Choice, visit func(rules.Choice)) {
	for _, c := range cs {
		walk(c, visit)
	}
}

// everyChoice yields every prompt the compendium poses, from every entry that
// can pose one.
func everyChoice(c *catalog.Catalog, visit func(rules.Choice)) {
	for _, r := range c.Races.All() {
		walkPtr(r.AbilityBonusOptions, visit)
		walkPtr(r.LanguageOptions, visit)
		walkAll(r.ProficiencyOptions, visit)
	}
	for _, s := range c.Subraces.All() {
		walkPtr(s.LanguageOptions, visit)
	}
	for _, tr := range c.Traits.All() {
		walkPtr(tr.ProficiencyOptions, visit)
		if tr.Specific != nil {
			walkPtr(tr.Specific.SpellOptions, visit)
			walkPtr(tr.Specific.SubtraitOptions, visit)
			walkPtr(tr.Specific.BreathWeapon, visit)
		}
	}
	for _, cl := range c.Classes.All() {
		walkAll(cl.ProficiencyOptions, visit)
		walkAll(cl.StartingEquipmentOptions, visit)
		walkAll(cl.MultiClassing.ProficiencyOptions, visit)
	}
	for _, f := range c.Features.All() {
		if f.Specific == nil {
			continue
		}
		walkPtr(f.Specific.ExpertiseOptions, visit)
		walkPtr(f.Specific.SubfeatureOptions, visit)
		walkPtr(f.Specific.EnemyTypeOptions, visit)
		walkPtr(f.Specific.TerrainTypeOptions, visit)
	}
	for _, b := range c.Backgrounds.All() {
		walkPtr(b.LanguageOptions, visit)
		walk(b.PersonalityTraits, visit)
		walk(b.Ideals, visit)
		walk(b.Bonds, visit)
		walk(b.Flaws, visit)
		walkAll(b.StartingEquipmentOptions, visit)
	}
}

// OptionKey is only useful if it is total and injective over the real data:
// total because an option with no key is an option a player cannot pick, and
// injective within a prompt because two options sharing a key means an answer
// is ambiguous and the projector would resolve it to whichever came first.
//
// This is the test that would catch a future SRD collection whose options are
// all bundles, or a generator change that drops a TextOption's key.
func TestOptionKeysAreTotalAndUniquePerPrompt(t *testing.T) {
	c := load(t, rules.LocaleEN)

	prompts := 0
	everyChoice(c, func(choice rules.Choice) {
		prompts++
		if choice.From.Kind != rules.OptionsExplicit {
			return
		}
		seen := make(map[rules.Slug]int, len(choice.From.Options))
		for i, option := range choice.From.Options {
			key := rules.OptionKey(option, i)
			if key.IsZero() {
				t.Errorf("prompt %q option %d (%T) has no key", choice.Prompt, i, option)
				continue
			}
			if first, dup := seen[key]; dup {
				t.Errorf("prompt %q: options %d and %d share the key %q",
					choice.Prompt, first, i, key)
				continue
			}
			seen[key] = i
		}
	})

	// A guard against this test silently passing because the walk found
	// nothing -- the failure mode that makes coverage tests worthless.
	if prompts < 100 {
		t.Fatalf("walked %d prompts, expected the compendium to pose far more", prompts)
	}
	t.Logf("checked %d prompts", prompts)
}

// The rogue's starting kit and Expertise are the two prompts that motivated
// OptionKey; pin them so a regression is legible rather than statistical.
func TestRogueBundleAndNestedPromptsAreAnswerable(t *testing.T) {
	c := load(t, rules.LocaleEN)

	rogue, ok := c.Classes.Get("rogue")
	if !ok {
		t.Fatal("Classes.Get(rogue) not found")
	}
	var kit rules.Choice
	for _, ch := range rogue.StartingEquipmentOptions {
		if ch.Prompt == "rogue/starting-equipment/1" {
			kit = ch
		}
	}
	if kit.Prompt.IsZero() {
		t.Fatal("rogue/starting-equipment/1 not found")
	}
	keys := rules.OptionKeys(kit.From)
	if len(keys) == 0 {
		t.Fatal("rogue/starting-equipment/1 has no option keys")
	}
	// The shortbow-and-arrows bundle has no slug of its own.
	if keys[0] != "#0" {
		t.Errorf("bundle option key = %q, want #0", keys[0])
	}

	expertise, ok := c.Features.Get("rogue-expertise-1")
	if !ok || expertise.Specific == nil {
		t.Fatal("rogue-expertise-1 has no specifics")
	}
	// Expertise is "choose 1 of: two skills, or one skill plus thieves'
	// tools" -- a nested choice against a bundle that contains one. Both
	// branches must be nameable, and the bundle has no slug of its own.
	outer := expertise.Specific.ExpertiseOptions
	if outer == nil {
		t.Fatal("rogue-expertise-1 has no expertise options")
	}
	inner := rules.OptionKeys(outer.From)
	want := []rules.Slug{"rogue-expertise-1/expertise/0/0", "#1"}
	if len(inner) != len(want) {
		t.Fatalf("expertise option keys = %v, want %v", inner, want)
	}
	for i, key := range want {
		if inner[i] != key {
			t.Errorf("expertise option key %d = %q, want %q", i, inner[i], key)
		}
	}
}

// Choice.Prompt is the only link between a stored answer and the question it
// answers, so two prompts sharing an id means an answer is ambiguous.
//
// This was not true until the generator started namespacing a bundle's items
// under the bundle's own index: the rogue's Expertise has a nested choice as
// option 0 and a bundle containing another nested choice as option 1, and
// both derived "rogue-expertise-1/expertise/0/0" while asking for a different
// number of picks.
func TestPromptIDsAreGloballyUnique(t *testing.T) {
	c := load(t, rules.LocaleEN)

	seen := make(map[rules.Slug]bool)
	total := 0
	everyChoice(c, func(choice rules.Choice) {
		total++
		if seen[choice.Prompt] {
			t.Errorf("prompt id %q is used by more than one choice", choice.Prompt)
			return
		}
		seen[choice.Prompt] = true
	})
	if total < 127 {
		t.Fatalf("walked %d prompts, expected at least the 127 the compendium poses", total)
	}
	t.Logf("checked %d prompt ids", total)
}
