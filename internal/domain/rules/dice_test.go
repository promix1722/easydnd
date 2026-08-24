package rules_test

import (
	"errors"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

func TestParseDice(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		terms   int
		bonus   int
		ability bool
	}{
		{in: "2d6", want: "2d6", terms: 1},
		{in: "1d8+3", want: "1d8+3", terms: 1, bonus: 3},
		{in: "1d4 - 1", want: "1d4-1", terms: 1, bonus: -1},
		{in: "d6", want: "1d6", terms: 1},
		{in: "5", want: "5", bonus: 5},
		// A sum of dice groups: meteor swarm rolls two separate pools.
		{in: "20d6 + 20d6", want: "20d6 + 20d6", terms: 2},
		{in: "4d6 + 5d6", want: "4d6 + 5d6", terms: 2},
		// "MOD" is the caster's ability modifier, unknowable here.
		{in: "1d8 + MOD", want: "1d8 + MOD", terms: 1, ability: true},
		{in: "2d8 + 4d6", want: "2d8 + 4d6", terms: 2},
		{in: "7d8 + 30", want: "7d8+30", terms: 1, bonus: 30},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := rules.ParseDice(tt.in)
			if err != nil {
				t.Fatalf("ParseDice(%q) error = %v", tt.in, err)
			}
			if len(got.Terms) != tt.terms {
				t.Errorf("terms = %d, want %d", len(got.Terms), tt.terms)
			}
			if got.Bonus != tt.bonus {
				t.Errorf("bonus = %d, want %d", got.Bonus, tt.bonus)
			}
			if got.PlusAbility != tt.ability {
				t.Errorf("PlusAbility = %v, want %v", got.PlusAbility, tt.ability)
			}
			if got.String() != tt.want {
				t.Errorf("String() = %q, want %q", got.String(), tt.want)
			}
		})
	}
}

// Round-tripping is what lets the loader and the generator share a notation
// without a separate encoder.
func TestParseDiceRoundTrips(t *testing.T) {
	for _, in := range []string{"2d6", "1d8+3", "4d6 + 5d6", "1d8 + MOD", "1d4-1", "5"} {
		parsed, err := rules.ParseDice(in)
		if err != nil {
			t.Fatalf("ParseDice(%q) error = %v", in, err)
		}
		again, err := rules.ParseDice(parsed.String())
		if err != nil {
			t.Fatalf("ParseDice(%q) error = %v", parsed.String(), err)
		}
		if again.String() != parsed.String() {
			t.Errorf("round trip of %q = %q, want %q", in, again.String(), parsed.String())
		}
	}
}

func TestParseDiceRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "d", "xdy", "2d", "1d6+", "-2d6"} {
		if _, err := rules.ParseDice(in); !errors.Is(err, rules.ErrInvalidDice) {
			t.Errorf("ParseDice(%q) error = %v, want ErrInvalidDice", in, err)
		}
	}
}

func TestDiceRange(t *testing.T) {
	d, err := rules.ParseDice("2d6+3")
	if err != nil {
		t.Fatalf("ParseDice() error = %v", err)
	}
	if got := d.Min(); got != 5 {
		t.Errorf("Min() = %d, want 5", got)
	}
	if got := d.Max(); got != 15 {
		t.Errorf("Max() = %d, want 15", got)
	}
	if got := d.Average(); got != 10 {
		t.Errorf("Average() = %v, want 10", got)
	}
}

// Rounding must go down on both sides of 10. Go's integer division truncates
// toward zero, which would make a score of 7 read as -1 instead of -2.
func TestModifierRoundsDown(t *testing.T) {
	tests := []struct{ score, want int }{
		{1, -5}, {3, -4}, {7, -2}, {8, -1}, {9, -1},
		{10, 0}, {11, 0}, {12, 1}, {15, 2}, {20, 5}, {30, 10},
	}
	for _, tt := range tests {
		if got := rules.Modifier(tt.score); got != tt.want {
			t.Errorf("Modifier(%d) = %d, want %d", tt.score, got, tt.want)
		}
	}
}

// Expertise doubles the proficiency bonus and Jack of All Trades halves it,
// which is why proficiency is an enum rather than a bool.
func TestProficiencyApply(t *testing.T) {
	tests := []struct {
		level rules.Proficiency
		bonus int
		want  int
	}{
		{rules.NotProficient, 3, 0},
		{rules.HalfProficient, 3, 1},
		{rules.Proficient, 3, 3},
		{rules.Expertise, 3, 6},
	}
	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			if got := tt.level.Apply(tt.bonus); got != tt.want {
				t.Errorf("Apply(%d) = %d, want %d", tt.bonus, got, tt.want)
			}
		})
	}
}

func TestRefRoundTrips(t *testing.T) {
	ref := rules.NewRef(rules.RefSpell, "acid-arrow")
	if ref.String() != "spell:acid-arrow" {
		t.Errorf("String() = %q, want %q", ref, "spell:acid-arrow")
	}
	got, ok := rules.ParseRef("spell:acid-arrow")
	if !ok {
		t.Fatal("ParseRef() failed")
	}
	if got != ref {
		t.Errorf("ParseRef() = %v, want %v", got, ref)
	}
	// A slug alone is ambiguous between collections, so an untyped reference
	// must be rejected rather than guessed at.
	if _, ok := rules.ParseRef("acid-arrow"); ok {
		t.Error("ParseRef() accepted an untyped reference")
	}
}

func TestCoinsConvertToCopper(t *testing.T) {
	tests := []struct {
		coins rules.Coins
		want  int
	}{
		{rules.Coins{Amount: 1, Unit: rules.Copper}, 1},
		{rules.Coins{Amount: 1, Unit: rules.Silver}, 10},
		{rules.Coins{Amount: 1, Unit: rules.Electrum}, 50},
		{rules.Coins{Amount: 15, Unit: rules.Gold}, 1500},
		{rules.Coins{Amount: 1, Unit: rules.Platinum}, 1000},
	}
	for _, tt := range tests {
		if got := tt.coins.InCopper(); got != tt.want {
			t.Errorf("%v InCopper() = %d, want %d", tt.coins, got, tt.want)
		}
	}
}
