package file

import (
	"context"
	"encoding/json"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"os"
	"reflect"
	"testing"
)

func TestVisualSchemaCoversWireCollectionsAndRecursiveMechanics(t *testing.T) {
	a := &Authoring{}
	var schema struct {
		Root        EditorField
		Definitions map[string]EditorField
	}
	if err := json.Unmarshal(a.Schema(), &schema); err != nil {
		t.Fatal(err)
	}
	for name, constructor := range collectionTypes {
		f, ok := schema.Root.Properties["entities"].Properties[name]
		if !ok || f.Type != "array" {
			t.Fatalf("collection %s missing", name)
		}
		wire := reflect.TypeOf(constructor()).Elem().Elem()
		if f.Items.Ref != wire.Name() {
			t.Fatalf("wrong type for %s", name)
		}
	}
	if weight := schema.Definitions["Item"].Properties["weight"]; weight.Type != "number" || weight.Integer {
		t.Fatal("fractional equipment weights must remain numbers")
	}
	if level := schema.Definitions["ResourceRow"].Properties["from"]; level.Type != "number" || !level.Integer {
		t.Fatal("progression levels must remain integers")
	}
	expression := schema.Definitions["Expression"]
	if expression.Properties["args"].Items.Ref != "Expression" {
		t.Fatal("recursive expressions lost")
	}
	for _, name := range []string{"CoreRules", "ActionDefinition", "Override", "ResourceDefinition", "Prose", "Choice", "Option"} {
		if _, ok := schema.Definitions[name]; !ok {
			t.Fatalf("%s not editable", name)
		}
	}
}
func TestForkRewritesReferencesButPreservesProse(t *testing.T) {
	a := &Authoring{}
	raw := []byte(`{"manifest":{"id":"original","version":"2.0.0","dependencies":[{"id":"other","version":"^1.0.0"}],"attribution":"original:feature:sample"},"entities":{"features":[{"id":"sample","class":"original:class:mage","desc":["original:feature:sample"],"text":"original:feature:sample"}]},"mechanics":{"casting":{"original:class:mage":{"kind":"shared"}},"rules":[{"owner":"original:feature:sample","effects":[{"ref":"other:spell:spell"}]}]},"locales":{"en":{"features":{"sample":{"name":"original:feature:sample"}}}}}`)
	b, err := a.Fork(raw, "copy", map[string]string{"other": "other-copy"})
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	_ = json.Unmarshal(b, &v)
	m := v["manifest"].(map[string]any)
	if m["id"] != "copy" || m["version"] != "1.0.0" || m["attribution"] != "original:feature:sample" {
		t.Fatal(m)
	}
	f := v["entities"].(map[string]any)["features"].([]any)[0].(map[string]any)
	if f["class"] != "copy:class:mage" || f["text"] != "original:feature:sample" {
		t.Fatal(f)
	}
	if v["locales"].(map[string]any)["en"].(map[string]any)["features"].(map[string]any)["sample"].(map[string]any)["name"] != "original:feature:sample" {
		t.Fatal("prose changed")
	}
}
func TestImportedExampleCompilesAndRoundTrips(t *testing.T) {
	base, err := NewRegistry([]string{"../../../../data/pack/srd-5.1"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	a := NewAuthoring(base, nil)
	raw, err := os.ReadFile("testdata/tactician.json")
	if err != nil {
		t.Fatal(err)
	}
	b, err := a.Fork(raw, "my-tactician", nil)
	if err != nil {
		t.Fatal(err)
	}
	docs := []pack.Document{}
	for _, r := range a.Builtins() {
		docs = append(docs, r.Releases...)
	}
	doc, lock, err := a.Validate(context.Background(), docs, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Packs) != 2 {
		t.Fatal(lock)
	}
	decoded, err := DecodePack(doc.Data)
	if err != nil {
		t.Fatal(err)
	}
	round, err := EncodePack(decoded)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodePack(round)
	if err != nil {
		t.Fatal(err)
	}
	release, err := again.Release()
	if err != nil || release != doc.Release {
		t.Fatal("round trip changed digest", err)
	}
}

func TestForkedCoreRetainsExactlyTheSixStandardScores(t *testing.T) {
	base, err := NewRegistry([]string{"../../../../data/pack/srd-5.1"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	a := NewAuthoring(base, nil)
	original := a.Builtins()[0].Releases[0]
	b, err := a.Fork(original.Data, "my-core", nil)
	if err != nil {
		t.Fatal(err)
	}
	doc, lock, err := a.Validate(context.Background(), nil, b)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := a.registry([]pack.Document{doc})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := registry.LoadLocked(context.Background(), "en", lock)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Abilities.Len() != 6 {
		t.Fatal("changed number of scores")
	}
}

func TestForkRewritesQualifiedExpressionInputs(t *testing.T) {
	a := &Authoring{}
	raw := []byte(`{"manifest":{"id":"original","version":"1.0.0"},"mechanics":{"rules":[{"effects":[{"target":"abilities.original:ability:wis","value":{"op":"read","ref":"modifier:original:ability:wis"}}]}]},"entities":{},"locales":{}}`)
	b, err := a.Fork(raw, "copy", nil)
	if err != nil {
		t.Fatal(err)
	}
	var p PackDocument
	if err = json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	effect := p.Mechanics.Rules[0].Effects[0]
	if effect.Target != "abilities.copy:ability:wis" || effect.Value.Ref != "modifier:copy:ability:wis" {
		t.Fatal(effect)
	}
}
