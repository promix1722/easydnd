package character

import "testing"

func TestFuzzyNamesPreserveSpellIdentity(t *testing.T) {
	for _, test := range []struct {
		a, b     string
		min, max float64
	}{{"  MAGIC--MISSILE ", "Magic Missile", 1, 1}, {"Firebal", "Fireball", .8, 1}, {"Fire Bolt", "Fireball", 0, .7}, {"Mass Healing Word", "Healing Word", 0, 0}, {"Greater Restoration", "Lesser Restoration", 0, 0}, {"Волшебная стрела", "волшебная-стрела", 1, 1}, {"", "Fireball", 0, 0}} {
		score := nameScore(test.a, test.b)
		if score < test.min || score > test.max {
			t.Errorf("%q / %q score %v", test.a, test.b, score)
		}
	}
}
