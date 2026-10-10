package character

import (
	"slices"
	"testing"
	"time"
)

// A listing and a sheet are two readings of one log and must say the same
// level. The rogue's log takes three levels by event; declaring seven is how
// an import and a level-up say the rest, and no event records it.
func TestAListingSaysTheLevelTheSheetDoes(t *testing.T) {
	t.Parallel()
	log, cat := RogueLog(t), LoadCatalog(t)
	if err := log.Append(Event{Type: EventChange, At: time.Unix(0, 0).UTC(), Changes: []Change{
		{Path: "identity.desiredLevel", Op: OpSet, Value: IntValue(7)},
	}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	state, err := Project(log, cat)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	sum := Summarize("chr", "owner", "", log, cat)
	if sum.Level != 7 || !slices.Equal(sum.Classes, state.Identity.Classes) {
		t.Errorf("listing = level %d %+v, sheet = %+v", sum.Level, sum.Classes, state.Identity.Classes)
	}
}
