package rules

import (
	"errors"
	"fmt"
)

// ErrInvalidCoin is returned for a coin unit outside the SRD's five
// denominations. Callers match it with errors.Is.
var ErrInvalidCoin = errors.New("invalid coin unit")

// CoinUnit is one of the five SRD coin denominations.
type CoinUnit uint8

// The coin denominations, least valuable first.
const (
	CoinNone CoinUnit = iota
	Copper
	Silver
	Electrum
	Gold
	Platinum
)

var coinNames = map[CoinUnit]string{
	CoinNone: "none",
	Copper:   "cp",
	Silver:   "sp",
	Electrum: "ep",
	Gold:     "gp",
	Platinum: "pp",
}

// copperValue is each denomination's worth in copper pieces, per the SRD's
// exchange table.
var copperValue = map[CoinUnit]int{
	Copper:   1,
	Silver:   10,
	Electrum: 50,
	Gold:     100,
	Platinum: 1000,
}

// String returns the two-letter abbreviation ("gp", "sp", ...), or "unknown"
// outside the enumeration.
func (u CoinUnit) String() string {
	if name, ok := coinNames[u]; ok {
		return name
	}
	return "unknown"
}

// ParseCoinUnit maps an abbreviation to its CoinUnit.
func ParseCoinUnit(s string) (CoinUnit, error) {
	for unit, name := range coinNames {
		if name == s && unit != CoinNone {
			return unit, nil
		}
	}
	return CoinNone, fmt.Errorf("%w: %q", ErrInvalidCoin, s)
}

// Coins is an amount of money in a single denomination.
//
// Prices are kept in the denomination the SRD prints them in rather than
// normalised to copper, so a longsword still costs "15 gp" on screen. Compare
// and total through InCopper.
type Coins struct {
	Amount int
	Unit   CoinUnit
}

// InCopper converts the amount to copper pieces, the common denominator for
// arithmetic and comparison. An unrecognised unit contributes nothing.
func (c Coins) InCopper() int { return c.Amount * copperValue[c.Unit] }

// IsZero reports whether the amount is nil.
func (c Coins) IsZero() bool { return c.Amount == 0 }

// String renders the amount as the SRD prints it, e.g. "15 gp".
func (c Coins) String() string { return fmt.Sprintf("%d %s", c.Amount, c.Unit) }

// Purse is a character's money, held per denomination because players track
// coins that way and converting on deposit would lose information.
type Purse map[CoinUnit]int

// InCopper totals the purse in copper pieces.
func (p Purse) InCopper() int {
	total := 0
	for unit, n := range p {
		total += n * copperValue[unit]
	}
	return total
}

// Add deposits an amount into the purse.
func (p Purse) Add(c Coins) {
	if c.Unit != CoinNone {
		p[c.Unit] += c.Amount
	}
}
