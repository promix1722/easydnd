package character

import "testing"

func TestFuzzyNamesPreserveSpellIdentity(t *testing.T) {
	for _, test := range []struct {
		a, b     string
		min, max float64
	}{{"  MAGIC--MISSILE ", "Magic Missile", 1, 1}, {"Firebal", "Fireball", .8, 1}, {"Fire Bolt", "Fireball", 0, .7}, {"Mass Healing Word", "Healing Word", 0, 0}, {"Greater Restoration", "Lesser Restoration", 0, 0}, {"Волшебная стрела", "волшебная-стрела", 1, 1}, {"", "Fireball", 0, 0}} {
		score := nameScore(test.a, test.b, false)
		if score < test.min || score > test.max {
			t.Errorf("%q / %q score %v", test.a, test.b, score)
		}
	}
}

// A sheet prints names its own way. These are the same identity as the
// catalogue's, and are told apart from the ones that only look like it.
func TestPrintedNamesMatchCatalogueNames(t *testing.T) {
	for _, test := range []struct {
		printed, catalogue string
		loose              bool
		min, max           float64
	}{
		{"Hempen Rope (50 feet)", "Rope, hempen (50 feet)", false, .99, .99},
		{"Hooded Lantern", "Lantern, hooded", false, .99, .99},
		{"Arrows", "Arrow", false, .99, .99},
		{"Rations", "Rations (1 day)", false, .9, .9},
		{"Insight", "Skill: Insight", false, .9, .9},
		// More words than the catalogue has is only a match where the
		// candidates are a closed set.
		{"Path of the Berserker", "Berserker", true, .9, .9},
		{"Path of the Berserker", "Berserker", false, 0, .84},
		{"Fire Shield", "Shield", false, 0, .84},
		{"Silver Dagger", "Dagger", false, 0, .84},
	} {
		score := nameScore(test.printed, test.catalogue, test.loose)
		if score < test.min || score > test.max {
			t.Errorf("%q / %q (loose=%v) score %v, want %v..%v", test.printed, test.catalogue, test.loose, score, test.min, test.max)
		}
	}
}
