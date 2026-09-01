package rules_test

import (
	"testing"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

func TestOptionKeyNamesEveryOptionKind(t *testing.T) {
	nested := rules.Choice{Prompt: "rogue-expertise-1/expertise/0/0", Choose: 2}

	tests := []struct {
		name   string
		option rules.Option
		want   rules.Slug
	}{
		{"ref", rules.RefOption{Ref: rules.NewRef(rules.RefItem, "shortsword"), Count: 1}, "shortsword"},
		{"text", rules.TextOption{Key: "i-idolize-a-particular-hero-of"}, "i-idolize-a-particular-hero-of"},
		{"ability bonus", rules.AbilityBonusOption{Ability: rules.Dexterity, Bonus: 1}, "dex"},
		{"size", rules.SizeOption{Size: rules.Medium}, rules.Slug(rules.Medium.String())},
		{"action", rules.ActionOption{Key: "breath-weapon", Count: 1}, "breath-weapon"},
		{"damage notes", rules.DamageOption{Notes: "black-dragon"}, "black-dragon"},
		// Where the prose naming the dragon is absent, the damage names it.
		{"damage type", rules.DamageOption{Damage: rules.Damage{Type: "acid"}}, "acid"},

		// The options that carry no slug of their own are named by what they
		// are made of, never by where they sit.
		{"bundle", rules.BundleOption{Items: []rules.Option{
			rules.RefOption{Ref: rules.NewRef(rules.RefItem, "shortbow"), Count: 1},
			rules.RefOption{Ref: rules.NewRef(rules.RefItem, "arrow"), Count: 20},
		}}, "shortbow+arrow"},
		{"money", rules.MoneyOption{Coins: rules.Coins{Amount: 15, Unit: rules.Gold}}, "15-gp"},
		{"score minimum", rules.ScoreMinimumOption{Ability: rules.Strength, Minimum: 13}, "str-13"},

		// A branch is named by the pool it draws from, and falls back to its
		// own prompt only where it lists its options inline.
		{"nested from a category", rules.NestedOption{Choice: rules.Choice{
			Prompt: "fighter/starting-equipment/1/0/0",
			From:   rules.OptionSet{Kind: rules.OptionsFromEquipmentCategory, Category: "martial-weapons"},
		}}, "martial-weapons"},
		{"nested from a collection", rules.NestedOption{Choice: rules.Choice{
			Prompt: "rogue/ability-score-improvement/4/1",
			From:   rules.OptionSet{Kind: rules.OptionsFromCollection, Collection: rules.RefFeat},
		}}, "feat"},
		{"nested from an inline list", rules.NestedOption{Choice: nested}, "rogue-expertise-1/expertise/0/0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rules.OptionKey(tt.option); got != tt.want {
				t.Errorf("OptionKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

// A composed key reads back as the things it is composed of, which is the
// whole point of composing it: an answer naming a bundle says what is in the
// bundle, in a log row and in a hand-written fixture alike.
func TestBundleKeysReadAsTheirContents(t *testing.T) {
	nested := rules.NestedOption{Choice: rules.Choice{
		Prompt: "fighter/starting-equipment/1/0/0",
		From:   rules.OptionSet{Kind: rules.OptionsFromEquipmentCategory, Category: "martial-weapons"},
	}}
	key := rules.OptionKey(rules.BundleOption{Items: []rules.Option{
		nested,
		rules.RefOption{Ref: rules.NewRef(rules.RefItem, "shield"), Count: 1},
	}})
	if key != "martial-weapons+shield" {
		t.Errorf("bundle key = %q, want martial-weapons+shield", key)
	}
}

// FindOption is the inverse of OptionKey, and Project depends on that being
// exactly true: a key it cannot resolve is a choice the character loses.
func TestFindOptionInvertsOptionKey(t *testing.T) {
	set := rules.OptionSet{
		Kind: rules.OptionsExplicit,
		Options: []rules.Option{
			rules.BundleOption{Items: []rules.Option{
				rules.RefOption{Ref: rules.NewRef(rules.RefItem, "shortbow"), Count: 1},
				rules.RefOption{Ref: rules.NewRef(rules.RefItem, "arrow"), Count: 20},
			}},
			rules.RefOption{Ref: rules.NewRef(rules.RefItem, "shortsword"), Count: 1},
		},
	}

	keys := rules.OptionKeys(set)
	if len(keys) != 2 {
		t.Fatalf("OptionKeys() = %v, want 2 keys", keys)
	}
	if keys[0] != "shortbow+arrow" {
		t.Errorf("bundle key = %q, want shortbow+arrow", keys[0])
	}
	if keys[1] != "shortsword" {
		t.Errorf("ref key = %q, want shortsword", keys[1])
	}

	bundle, ok := rules.FindOption(set, keys[0])
	if !ok {
		t.Fatalf("FindOption(%q) not found", keys[0])
	}
	if _, isBundle := bundle.(rules.BundleOption); !isBundle {
		t.Errorf("FindOption(%q) = %T, want BundleOption", keys[0], bundle)
	}

	sword, ok := rules.FindOption(set, keys[1])
	if !ok {
		t.Fatalf("FindOption(%q) not found", keys[1])
	}
	if ref, isRef := sword.(rules.RefOption); !isRef || ref.Ref.Slug != "shortsword" {
		t.Errorf("FindOption(%q) = %#v, want RefOption{shortsword}", keys[1], sword)
	}

	if _, ok := rules.FindOption(set, "no-such-option"); ok {
		t.Error("FindOption(no-such-option) reported found")
	}
}

// A set that draws from a collection names its members by their own slugs,
// so it has no explicit keys.
func TestOptionKeysIsNilForNonExplicitSets(t *testing.T) {
	set := rules.OptionSet{Kind: rules.OptionsFromCollection, Collection: rules.RefLanguage}
	if got := rules.OptionKeys(set); got != nil {
		t.Errorf("OptionKeys() = %v, want nil", got)
	}
}

// Every ChoiceKind must have a wire name and round-trip through it. The kinds
// are serialized by name, never by number, so a missing entry in the table
// would silently render as "unknown" on a prompt the UI then cannot draw.
func TestEveryChoiceKindRoundTrips(t *testing.T) {
	// ChooseNothing is the zero value and is deliberately excluded from
	// ParseChoiceKind's inverse, so start above it and walk until String()
	// stops recognising a value.
	for kind := rules.ChooseNothing; ; kind++ {
		name := kind.String()
		if name == "unknown" {
			if kind <= rules.ChooseAbilityScores {
				t.Fatalf("ChoiceKind(%d) has no wire name, but %d is in range",
					kind, rules.ChooseAbilityScores)
			}
			break
		}
		got, ok := rules.ParseChoiceKind(name)
		if !ok {
			t.Errorf("ParseChoiceKind(%q) not recognised", name)
			continue
		}
		if got != kind {
			t.Errorf("ParseChoiceKind(%q) = %d, want %d", name, got, kind)
		}
	}

	// The synthetic kinds specifically: these are the ones a future edit is
	// most likely to add without touching the name table.
	for _, tt := range []struct {
		kind rules.ChoiceKind
		name string
	}{
		{rules.ChooseRace, "race"},
		{rules.ChooseSubrace, "subrace"},
		{rules.ChooseBackground, "background"},
		{rules.ChooseClass, "class"},
		{rules.ChooseSubclass, "subclass"},
		{rules.ChooseLevel, "level"},
		{rules.ChooseAlignment, "alignment"},
		{rules.ChooseAbilityScores, "ability-scores"},
	} {
		if got := tt.kind.String(); got != tt.name {
			t.Errorf("ChoiceKind.String() = %q, want %q", got, tt.name)
		}
	}
}
