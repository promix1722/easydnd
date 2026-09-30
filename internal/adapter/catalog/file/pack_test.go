package file_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	file "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	character "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

func basePath() string { return filepath.Join("..", "..", "..", "..", "data", "srd_5.1") }
func addonPath() string {
	return filepath.Join("..", "..", "..", "..", "data", "packs", "examples", "tactician.json")
}
func registry(t *testing.T) *file.Registry {
	t.Helper()
	r, err := file.NewRegistry([]string{basePath(), addonPath()}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func build(t *testing.T, classes ...character.Event) character.Log {
	t.Helper()
	l := character.Log{}
	if err := l.Append(append([]character.Event{{Type: character.EventInit}}, classes...)...); err != nil {
		t.Fatal(err)
	}
	return l
}
func ref(kind rules.RefKind, id string) rules.Ref { return rules.NewRef(kind, rules.Slug(id)) }

func TestPackRoundTripAndCanonicalDigest(t *testing.T) {
	p, err := file.LoadPack(addonPath())
	if err != nil {
		t.Fatal(err)
	}
	before, err := file.PackDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	data, err := file.EncodePack(p)
	if err != nil {
		t.Fatal(err)
	}
	copy, err := file.DecodePack(data)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "pack")
	if err = file.SavePackDirectory(dir, copy); err != nil {
		t.Fatal(err)
	}
	loaded, err := file.LoadPack(dir)
	if err != nil {
		t.Fatal(err)
	}
	after, err := file.PackDigest(loaded)
	if err != nil || before != after {
		t.Fatalf("digest changed: %s %s %v", before, after, err)
	}
	if err = file.SavePackDirectory(dir, copy); err == nil {
		t.Fatal("overwrote existing release")
	}
}
func TestAddonResourcesAndLocale(t *testing.T) {
	r := registry(t)
	cat, err := r.Load(context.Background(), rules.LocaleEN)
	if err != nil {
		t.Fatal(err)
	}
	log := build(t, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, "fighter"), Level: 10}, character.Event{Type: character.EventSubclass, Ref: ref(rules.RefSubclass, "example/tactician")}, character.Event{Type: character.EventRace, Ref: ref(rules.RefRace, "example/wanderer")})
	log.Events[0].RulesLock = cat.Lock
	sheet, err := character.Project(log, cat)
	if err != nil {
		t.Fatal(err)
	}
	pool := sheet.Resources.Pools["example/combat-dice"]
	if pool.Max != 5 || pool.Dice != "1d10" || pool.Name != "Combat Dice" {
		t.Fatalf("pool: %+v", pool)
	}
	if len(sheet.Abilities.Scores) != 6 || sheet.Abilities.Score(rules.Wisdom) != 11 {
		t.Fatal("expected six standard scores and the race Wisdom bonus")
	}
	ru, err := r.Load(context.Background(), rules.Locale("ru-RU"))
	if err != nil {
		t.Fatal(err)
	}
	localized, err := character.Project(log, ru)
	if err != nil {
		t.Fatal(err)
	}
	if got := localized.Resources.Pools["example/combat-dice"]; got.Name != "Боевые кости" || got.Max != pool.Max {
		t.Fatalf("localized pool: %+v", got)
	}
	fighter, _ := cat.Classes.Get("fighter")
	found := false
	for _, s := range fighter.Subclasses {
		found = found || s == "example/tactician"
	}
	if !found {
		t.Fatal("parent class did not discover addon subclass")
	}
}
func TestResourceReplayAndSeparateCastingPools(t *testing.T) {
	r := registry(t)
	cat, err := r.Load(context.Background(), rules.LocaleEN)
	if err != nil {
		t.Fatal(err)
	}
	log := build(t, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, "warlock"), Level: 5}, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, "wizard"), Level: 3})
	if err = log.Append(character.Event{Type: character.EventResourceSpent, Resource: "pact-magic", Amount: 1}, character.Event{Type: character.EventResourceSpent, Resource: "spell-slots/1", Amount: 1}, character.Event{Type: character.EventRest, Trigger: "short-rest"}); err != nil {
		t.Fatal(err)
	}
	sheet, err := character.Project(log, cat)
	if err != nil {
		t.Fatal(err)
	}
	if p := sheet.Resources.Pools["pact-magic"]; p.Max != 2 || p.SlotLevel != 3 || p.Used != 0 {
		t.Fatalf("pact pool: %+v", p)
	}
	if p := sheet.Resources.Pools["spell-slots/1"]; p.Max != 4 || p.Used != 1 {
		t.Fatalf("ordinary pool: %+v", p)
	}
	bad := build(t, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, "warlock"), Level: 1}, character.Event{Type: character.EventResourceSpent, Resource: "pact-magic", Amount: 2}, character.Event{Type: character.EventLevel, Ref: ref(rules.RefClass, "warlock"), Level: 2})
	if _, err = character.Project(bad, cat); err == nil {
		t.Fatal("later level legalized earlier overspend")
	}
}
func TestRejectDuplicateKeysMissingDependenciesAndChangedRelease(t *testing.T) {
	if _, err := file.DecodePack([]byte(`{"manifest":{},"manifest":{}}`)); err == nil {
		t.Fatal("duplicate keys accepted")
	}
	if _, err := file.NewRegistry([]string{addonPath()}, nil, ""); err == nil {
		t.Fatal("missing dependency accepted")
	}
	archive := t.TempDir()
	r, err := file.NewRegistry([]string{basePath(), addonPath()}, nil, archive)
	if err != nil {
		t.Fatal(err)
	}
	p, err := file.LoadPack(addonPath())
	if err != nil {
		t.Fatal(err)
	}
	p.Locales["en"]["resources"]["combat-dice"] = file.Prose{Name: "Changed"}
	b, err := file.EncodePack(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "changed.json")
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = file.NewRegistry([]string{basePath(), path}, nil, archive); err == nil {
		t.Fatal("changed immutable release accepted")
	}
	lock := r.DefaultLock()
	lock.Packs[0].Digest = strings.Repeat("0", 64)
	if _, err = r.LoadLocked(context.Background(), rules.LocaleEN, lock); err == nil {
		t.Fatal("wrong digest accepted")
	}
	p.Manifest.Version = "2.0.1"
	b, _ = json.Marshal(p)
	if _, err = file.DecodePack(b); err != nil {
		t.Fatal(err)
	}
}

func TestActionCostRecoveryBudgetAndReplay(t *testing.T) {
	r := registry(t)
	cat, err := r.Load(context.Background(), rules.LocaleEN)
	if err != nil {
		t.Fatal(err)
	}
	log := build(t, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, "fighter"), Level: 3}, character.Event{Type: character.EventSubclass, Ref: ref(rules.RefSubclass, "example/tactician")})
	for range 4 {
		if err := log.Append(character.Event{Type: character.EventAction, Ref: ref(rules.RefAction, "example/maneuver")}); err != nil {
			t.Fatal(err)
		}
	}
	sheet, err := character.Project(log, cat)
	if err != nil {
		t.Fatal(err)
	}
	if p := sheet.Resources.Pools["example/combat-dice"]; p.Used != 4 || p.Available() != 0 {
		t.Fatalf("action cost: %+v", p)
	}
	if len(sheet.PackActions) != 1 || sheet.PackActions[0].Available {
		t.Fatal("exhausted action still offered as affordable")
	}
	if sheet.Status.Initiative != 1 || len(sheet.Spells.Known) != 1 || sheet.Spells.Known[0] != "example/guiding-mark" {
		t.Fatal("addon grant/modifier not applied")
	}
	overspend := log.Clone()
	_ = overspend.Append(character.Event{Type: character.EventAction, Ref: ref(rules.RefAction, "example/maneuver")})
	if _, err := character.Project(overspend, cat); err == nil {
		t.Fatal("action overspend accepted")
	}
	_ = log.Append(character.Event{Type: character.EventRest, Trigger: "short-rest"}, character.Event{Type: character.EventAction, Ref: ref(rules.RefAction, "example/maneuver")})
	sheet, err = character.Project(log, cat)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Resources.Pools["example/combat-dice"].Used != 1 {
		t.Fatal("rest/action replay wrong")
	}
	// Recovery budget is across classes, not independently per pool.
	dice := build(t, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, "fighter"), Level: 3}, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, "wizard"), Level: 3}, character.Event{Type: character.EventResourceSpent, Resource: "hit-dice/fighter", Amount: 3}, character.Event{Type: character.EventResourceSpent, Resource: "hit-dice/wizard", Amount: 3})
	_ = dice.Append(character.Event{Type: character.EventRest, Trigger: "long-rest", Allocations: map[rules.Slug]int{"hit-dice/fighter": 2, "hit-dice/wizard": 1}})
	sheet, err = character.Project(dice, cat)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Resources.Pools["hit-dice/fighter"].Used != 1 || sheet.Resources.Pools["hit-dice/wizard"].Used != 2 {
		t.Fatal("incorrect allocated recovery")
	}
	dice.Events[len(dice.Events)-1].Allocations["hit-dice/wizard"] = 2
	if _, err := character.Project(dice, cat); err == nil {
		t.Fatal("shared recovery budget exceeded")
	}
}

func writeDocument(t *testing.T, p *file.PackDocument) string {
	t.Helper()
	data, err := file.EncodePack(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "pack.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestRuleValidationAndGuardedOverrides(t *testing.T) {
	p, err := file.LoadPack(addonPath())
	if err != nil {
		t.Fatal(err)
	}
	replacement := file.ResourceDefinition{ID: "override", Owner: "srd-2014:class:warlock", MinimumLevel: 1, Kind: "pool", Input: "srd-2014:class:warlock", Rows: []file.ResourceRow{{From: 1, Capacity: 7, SlotLevel: 1}}}
	p.Mechanics.Overrides = []file.Override{{Target: "srd-2014:resource:pact-magic", Version: "^1.0.0", Resource: &replacement}}
	path := writeDocument(t, p)
	r, err := file.NewRegistry([]string{basePath(), path}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := r.Load(context.Background(), rules.LocaleEN)
	sheet, err := character.Project(build(t, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, "warlock"), Level: 1}), cat)
	if err != nil || sheet.Resources.Pools["pact-magic"].Max != 7 {
		t.Fatalf("replacement: %v %+v", err, sheet.Resources.Pools["pact-magic"])
	}
	p.Mechanics.Overrides[0].Version = ">=2.0.0"
	if _, err := file.NewRegistry([]string{basePath(), writeDocument(t, p)}, nil, ""); err == nil {
		t.Fatal("replacement precondition ignored")
	}
	p.Mechanics.Overrides[0].Version = "^1.0.0"
	p.Mechanics.Overrides = append(p.Mechanics.Overrides, p.Mechanics.Overrides[0])
	if _, err := file.NewRegistry([]string{basePath(), writeDocument(t, p)}, nil, ""); err == nil {
		t.Fatal("conflicting replacements accepted")
	}
	p.Mechanics.Overrides = nil
	p.Mechanics.Rules = []file.RuleDefinition{{ID: "cycle", Owner: "subclass:tactician", Effects: []file.Effect{{Op: "add", Target: "abilities.srd-2014:ability:wis", Value: file.Expression{Op: "read", Ref: "modifier:srd-2014:ability:wis"}}}}}
	if _, err := file.NewRegistry([]string{basePath(), writeDocument(t, p)}, nil, ""); err == nil {
		t.Fatal("cyclic stat dependency accepted")
	}
	p.Mechanics.Rules[0].Effects[0].Target = "unsupported.stat"
	if err := p.Validate(); err == nil {
		t.Fatal("unsupported effect accepted")
	}
	p.Mechanics.Rules = nil
	p.Manifest.Dependencies = nil
	if err := p.Validate(); err == nil {
		t.Fatal("undeclared cross-pack reference accepted")
	}
}
func TestPinnedReleaseSurvivesUpdateAndRestart(t *testing.T) {
	archive := t.TempDir()
	old := registry(t)
	oldLock := old.DefaultLock()
	p, err := file.LoadPack(addonPath())
	if err != nil {
		t.Fatal(err)
	}
	p.Manifest.Version = "2.1.0"
	p.Mechanics.Resources[0].Rows[0].Capacity = 9
	newPath := writeDocument(t, p)
	// Archive both releases; only the explicit roots select the new default.
	r, err := file.NewRegistry([]string{basePath(), addonPath(), newPath}, []file.Dependency{{ID: "example", Version: "2.1.0"}}, archive)
	if err != nil {
		t.Fatal(err)
	}
	log := build(t, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, "fighter"), Level: 3}, character.Event{Type: character.EventSubclass, Ref: ref(rules.RefSubclass, "example/tactician")})
	log.Events[0].RulesLock = oldLock
	cat, err := r.LoadLocked(context.Background(), rules.LocaleEN, oldLock)
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := character.Project(log, cat)
	if err != nil || sheet.Resources.Pools["example/combat-dice"].Max != 4 {
		t.Fatal("old character changed")
	}
	restarted, err := file.NewRegistry([]string{basePath(), newPath}, nil, archive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.LoadLocked(context.Background(), rules.LocaleRU, oldLock); err != nil {
		t.Fatal(err)
	}
	newest, _ := restarted.Load(context.Background(), rules.LocaleEN)
	if _, err := character.Project(log, newest); err == nil {
		t.Fatal("wrong context silently substituted")
	}
}

func TestExplicitCasterProfilesAndTypedParameters(t *testing.T) {
	p, err := file.LoadPack(addonPath())
	if err != nil {
		t.Fatal(err)
	}
	p.Entities["classes"], _ = json.Marshal([]file.Class{{Slug: "arcanist", HitDie: 6, SpellcastingLevel: 1, SpellcastingAbility: "srd-2014:ability:int"}})
	for _, locale := range []string{"en", "ru"} {
		p.Locales[locale]["classes"] = file.Bundle{"arcanist": file.Prose{Name: "Arcanist"}}
	}
	p.Mechanics.Casting = map[string]file.CastingProfile{"arcanist": {Kind: "shared", Numerator: 1, Denominator: 3, Rounding: "floor", StartsAt: 1}}
	r, err := file.NewRegistry([]string{basePath(), writeDocument(t, p)}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	cat, err := r.Load(context.Background(), rules.LocaleEN)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		class         string
		level, slots2 int
	}{{"paladin", 1, 2}, {"paladin", 3, 3}, {"example/arcanist", 3, 3}} {
		log := build(t, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, "wizard"), Level: 3}, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, tc.class), Level: tc.level})
		sheet, err := character.Project(log, cat)
		if err != nil {
			t.Fatal(err)
		}
		if sheet.Resources.Pools["spell-slots/2"].Max != tc.slots2 {
			t.Fatalf("profile %+v produced %+v", tc, sheet.Resources.Pools)
		}
	}
	log := build(t, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, "cleric"), Level: 6}, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, "paladin"), Level: 3}, character.Event{Type: character.EventSubclass, Ref: ref(rules.RefSubclass, "devotion")})
	sheet, err := character.Project(log, cat)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Resources.Pools["channel-divinity"].Max != 2 {
		t.Fatal("paladin and cleric double-counted shared divinity")
	}
	sheet, err = character.Project(build(t, character.Event{Type: character.EventClass, Ref: ref(rules.RefClass, "druid"), Level: 2}), cat)
	if err != nil {
		t.Fatal(err)
	}
	cr := sheet.Resources.Parameters["wild-shape-max-cr"].Rational
	if cr == nil || cr.Numerator != 1 || cr.Denominator != 4 {
		t.Fatal("fractional CR lost its type")
	}
	if swimming := sheet.Resources.Parameters["wild-shape-swim"].Boolean; swimming == nil || *swimming {
		t.Fatal("boolean parameter was lost")
	}
}

func TestPacksRejectCustomAbilityScores(t *testing.T) {
	p, err := file.LoadPack(addonPath())
	if err != nil {
		t.Fatal(err)
	}
	p.Entities["abilities"] = json.RawMessage(`[{"slug":"luck"}]`)
	p.Locales["en"]["abilities"] = file.Bundle{"luck": file.Prose{Name: "Luck"}}
	if err := p.Validate(); err == nil {
		t.Fatal("custom characteristic accepted")
	}
	if _, ok := rules.ParseAbility("example/luck"); ok {
		t.Fatal("custom characteristic accepted in an event path")
	}
}
