package file

import (
	"testing"

	"github.com/promix1722/easydnd/internal/domain/catalog"
)

func TestActionTagsAndKindsAreValidated(t *testing.T) {
	t.Parallel()
	c := &conv{where: "test"}
	c.tagged(catalog.Entry{Slug: "x"}, &ActionTag{Kind: "swift"})
	if c.Err() == nil {
		t.Error("unknown action kind on a tag accepted")
	}
	for name, a := range map[string]ActionDefinition{
		"unknown kind":           {ID: "a", Manual: true, Owner: "class:fighter", Kind: "swift"},
		"unowned with condition": {ID: "a", Manual: true, MinimumLevel: 3},
	} {
		if err := validateMechanics(PackMechanics{Actions: []ActionDefinition{a}}); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
