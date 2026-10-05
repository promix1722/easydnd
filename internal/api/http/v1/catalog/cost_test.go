package catalog

import (
	"testing"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

// The unit is an enumeration; converting it as a byte sent "\x04" for gold.
func TestCostUnitIsItsAbbreviation(t *testing.T) {
	if got := costOf(rules.Coins{Amount: 25, Unit: rules.Gold}).Unit; got != "gp" {
		t.Fatalf("unit = %q, want gp", got)
	}
}
