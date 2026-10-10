package file

import "testing"

// A class's "any arcane focus" must name the category the way the pack's own
// equipment-categories are named, or the choice resolves to no options.
func TestOptionSetCategoryIsQualifiedWithThePack(t *testing.T) {
	t.Parallel()
	set := func() map[string]any {
		return map[string]any{"kind": "equipment-category", "category": "arcane-foci"}
	}
	if got := normalizeValue("dnd-2014", "from", set()).(map[string]any)["category"]; got != "dnd-2014/arcane-foci" {
		t.Errorf("namespaced pack: category = %v", got)
	}
	if got := normalizeValue("srd-2014", "from", set()).(map[string]any)["category"]; got != "arcane-foci" {
		t.Errorf("base pack: category = %v", got)
	}
	weapon := map[string]any{"category": "simple", "range": "melee"}
	if got := normalizeValue("dnd-2014", "weapon", weapon).(map[string]any)["category"]; got != "simple" {
		t.Errorf("a weapon's category was qualified: %v", got)
	}
}
