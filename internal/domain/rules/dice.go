package rules

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidDice is returned by ParseDice for text it cannot read as a dice
// expression. Callers match it with errors.Is.
var ErrInvalidDice = errors.New("invalid dice expression")

// DiceTerm is one group of identical dice: the "4d6" in "4d6 + 2d8".
type DiceTerm struct {
	Count int // number of dice rolled
	Faces int // sides per die
}

// String renders the term in NdF form.
func (t DiceTerm) String() string { return fmt.Sprintf("%dd%d", t.Count, t.Faces) }

// Dice is a rolled expression in the SRD's notation.
//
// It is a sum rather than a single NdF+B, because the SRD writes several
// spells that way -- meteor swarm is "20d6 + 20d6", one term of fire and one
// of bludgeoning, and delayed blast fireball scales as "4d6 + 5d6". Flattening
// those to a single group would change the damage.
//
// Terms may be empty, which is how a flat value such as a fixed heal is
// expressed, and how "no damage at this level" stays distinguishable from
// "zero damage".
type Dice struct {
	Terms []DiceTerm
	Bonus int // flat modifier added after the roll; may be negative

	// PlusAbility marks an expression that adds the caster's spellcasting
	// ability modifier -- the SRD writes healing as "1d8 + MOD". It cannot be
	// folded into Bonus because the value is not known until a specific
	// character casts the spell.
	PlusAbility bool
}

// ParseDice reads the SRD's dice notation: "2d6", "1d8+3", "1d4 - 1",
// "4d6 + 5d6", "1d8 + MOD", and a bare constant such as "5".
func ParseDice(s string) (Dice, error) {
	text := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	if text == "" {
		return Dice{}, fmt.Errorf("%w: empty", ErrInvalidDice)
	}

	var d Dice
	for _, token := range splitSigned(text) {
		if err := d.addToken(token, s); err != nil {
			return Dice{}, err
		}
	}
	return d, nil
}

// splitSigned breaks "4d6+2d8-1" into "+4d6", "+2d8", "-1", keeping each
// token's sign attached so the caller never has to track it.
func splitSigned(text string) []string {
	var out []string
	start := 0
	for i := 1; i < len(text); i++ {
		if text[i] == '+' || text[i] == '-' {
			out = append(out, text[start:i])
			start = i
		}
	}
	return append(out, text[start:])
}

// addToken folds one signed token into the expression.
func (d *Dice) addToken(token, original string) error {
	negative := strings.HasPrefix(token, "-")
	token = strings.TrimLeft(token, "+-")
	if token == "" {
		return fmt.Errorf("%w: dangling sign in %q", ErrInvalidDice, original)
	}

	// "MOD" stands for the caster's ability modifier.
	if token == "mod" {
		d.PlusAbility = true
		return nil
	}

	countText, facesText, isDice := strings.Cut(token, "d")
	if !isDice {
		n, err := strconv.Atoi(token)
		if err != nil {
			return fmt.Errorf("%w: %q", ErrInvalidDice, original)
		}
		if negative {
			n = -n
		}
		d.Bonus += n
		return nil
	}

	count := 1 // "d6" means one d6
	if countText != "" {
		n, err := strconv.Atoi(countText)
		if err != nil {
			return fmt.Errorf("%w: bad count in %q", ErrInvalidDice, original)
		}
		count = n
	}
	faces, err := strconv.Atoi(facesText)
	if err != nil {
		return fmt.Errorf("%w: bad faces in %q", ErrInvalidDice, original)
	}
	if count < 0 || faces < 0 {
		return fmt.Errorf("%w: negative dice in %q", ErrInvalidDice, original)
	}
	if negative {
		return fmt.Errorf("%w: subtracted dice in %q", ErrInvalidDice, original)
	}
	d.Terms = append(d.Terms, DiceTerm{Count: count, Faces: faces})
	return nil
}

// IsZero reports whether the expression rolls nothing and adds nothing.
func (d Dice) IsZero() bool { return len(d.Terms) == 0 && d.Bonus == 0 && !d.PlusAbility }

// String renders the expression back into SRD notation. It round-trips
// through ParseDice.
func (d Dice) String() string {
	var b strings.Builder
	for i, term := range d.Terms {
		if i > 0 {
			b.WriteString(" + ")
		}
		b.WriteString(term.String())
	}
	switch {
	case d.Bonus > 0 && b.Len() > 0:
		fmt.Fprintf(&b, "+%d", d.Bonus)
	case d.Bonus != 0 && b.Len() > 0:
		fmt.Fprintf(&b, "%d", d.Bonus)
	case b.Len() == 0 && !d.PlusAbility:
		fmt.Fprintf(&b, "%d", d.Bonus)
	case b.Len() == 0 && d.Bonus != 0:
		fmt.Fprintf(&b, "%d", d.Bonus)
	}
	if d.PlusAbility {
		if b.Len() > 0 {
			b.WriteString(" + MOD")
		} else {
			b.WriteString("MOD")
		}
	}
	return b.String()
}

// Average returns the mean result, with each die averaging (faces+1)/2.
//
// PlusAbility contributes nothing: the modifier belongs to a character, and
// this type does not know one. Use it for estimates and sorting, never in
// place of an actual roll.
func (d Dice) Average() float64 {
	total := float64(d.Bonus)
	for _, t := range d.Terms {
		total += float64(t.Count) * (float64(t.Faces) + 1) / 2
	}
	return total
}

// Min returns the lowest possible result: every die rolls a 1.
func (d Dice) Min() int {
	total := d.Bonus
	for _, t := range d.Terms {
		total += t.Count
	}
	return total
}

// Max returns the highest possible result: every die rolls its top face.
func (d Dice) Max() int {
	total := d.Bonus
	for _, t := range d.Terms {
		total += t.Count * t.Faces
	}
	return total
}

// Damage pairs a dice expression with the damage type it deals.
type Damage struct {
	Dice Dice
	Type Slug // a damage-type slug: "acid", "bludgeoning", ...
}
