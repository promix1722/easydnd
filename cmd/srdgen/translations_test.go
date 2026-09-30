package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Russian is intentionally complete. The catalogue still supports partial
// locales, but a new English SRD leaf must not silently put English prose back
// onto a Russian character sheet.
func TestRussianTranslationCoversEveryEnglishLeaf(t *testing.T) {
	englishDir := filepath.Join("..", "..", "data", "srd_5.1", "i18n", "en")
	russianDir := filepath.Join("..", "..", "data", "translations", "ru")
	entries, err := os.ReadDir(englishDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()
		en := readTranslationJSON(t, filepath.Join(englishDir, name))
		ru := readTranslationJSON(t, filepath.Join(russianDir, name))
		enLeaves, ruLeaves := map[string]string{}, map[string]string{}
		translationLeaves(en, "", enLeaves)
		translationLeaves(ru, "", ruLeaves)
		for path, source := range enLeaves {
			if source != "" && ruLeaves[path] == "" {
				t.Errorf("%s%s has no Russian translation", name, path)
			}
		}
	}
}

func TestRussianClassNamesAreUnique(t *testing.T) {
	path := filepath.Join("..", "..", "data", "translations", "ru", "classes.json")
	value := readTranslationJSON(t, path)
	classes, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s is %T, want an object", path, value)
	}
	seen := map[string]string{}
	want := map[string]string{
		"barbarian": "Варвар", "bard": "Бард", "cleric": "Жрец", "druid": "Друид",
		"fighter": "Воин", "monk": "Монах", "paladin": "Паладин", "ranger": "Следопыт",
		"rogue": "Плут", "sorcerer": "Чародей", "warlock": "Колдун", "wizard": "Волшебник",
	}
	for slug, raw := range classes {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("%s: %s is %T, want an object", path, slug, raw)
		}
		name, _ := entry["name"].(string)
		if name != want[slug] {
			t.Errorf("%s name = %q, want %q", slug, name, want[slug])
		}
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			t.Errorf("%s has an empty name", slug)
			continue
		}
		if previous, exists := seen[key]; exists {
			t.Errorf("%s and %s both use Russian class name %q", previous, slug, name)
		}
		seen[key] = slug
	}
}

func TestRussianSkillNamesMatchReferences(t *testing.T) {
	path := filepath.Join("..", "..", "data", "translations", "ru", "skills.json")
	value := readTranslationJSON(t, path)
	skills, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s is %T, want an object", path, value)
	}
	want := map[string]string{
		"acrobatics": "Акробатика", "animal-handling": "Уход за животными", "arcana": "Магия",
		"athletics": "Атлетика", "deception": "Обман", "history": "История",
		"insight": "Проницательность", "intimidation": "Запугивание", "investigation": "Расследование",
		"medicine": "Медицина", "nature": "Природа", "perception": "Восприятие",
		"performance": "Выступление", "persuasion": "Убеждение", "religion": "Религия",
		"sleight-of-hand": "Ловкость рук", "stealth": "Скрытность", "survival": "Выживание",
	}
	for slug, expected := range want {
		entry, ok := skills[slug].(map[string]any)
		if !ok {
			t.Errorf("%s is missing", slug)
			continue
		}
		if name, _ := entry["name"].(string); name != expected {
			t.Errorf("%s name = %q, want %q", slug, name, expected)
		}
	}

	// Skill choices are proficiency references, so the proficiency catalogue
	// carries a second user-facing copy of every skill name. Keep it locked to
	// the same vocabulary or the build screen and finished sheet disagree.
	proficienciesPath := filepath.Join("..", "..", "data", "translations", "ru", "proficiencies.json")
	proficiencies, ok := readTranslationJSON(t, proficienciesPath).(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object", proficienciesPath)
	}
	for slug, expected := range want {
		key := "skill-" + slug
		entry, ok := proficiencies[key].(map[string]any)
		if !ok {
			t.Errorf("%s is missing", key)
			continue
		}
		if name, _ := entry["name"].(string); name != "Навык: "+expected {
			t.Errorf("%s name = %q, want %q", key, name, "Навык: "+expected)
		}
	}
}

func TestRussianLanguageAndAlignmentNamesMatchReferences(t *testing.T) {
	wants := map[string]map[string]string{
		"languages.json": {
			"abyssal": "Язык Бездны", "celestial": "Небесный", "common": "Общий",
			"deep-speech": "Глубинная речь", "draconic": "Драконий", "dwarvish": "Дварфский",
			"elvish": "Эльфийский", "giant": "Великаний", "gnomish": "Гномий",
			"goblin": "Гоблинский", "halfling": "Язык полуросликов", "infernal": "Инфернальный",
			"orc": "Орочий", "primordial": "Первичный", "sylvan": "Сильван",
			"undercommon": "Подземный общий",
		},
		"alignments.json": {
			"chaotic-evil": "Хаотично-злой", "chaotic-good": "Хаотично-добрый",
			"chaotic-neutral": "Хаотично-нейтральный", "lawful-evil": "Законно-злой",
			"lawful-good": "Законно-добрый", "lawful-neutral": "Законно-нейтральный",
			"neutral": "Нейтральный", "neutral-evil": "Нейтрально-злой",
			"neutral-good": "Нейтрально-добрый",
		},
	}
	for file, want := range wants {
		path := filepath.Join("..", "..", "data", "translations", "ru", file)
		entries, ok := readTranslationJSON(t, path).(map[string]any)
		if !ok {
			t.Fatalf("%s is not an object", path)
		}
		for slug, expected := range want {
			entry, ok := entries[slug].(map[string]any)
			if !ok {
				t.Errorf("%s: %s is missing", file, slug)
				continue
			}
			if name, _ := entry["name"].(string); name != expected {
				t.Errorf("%s: %s name = %q, want %q", file, slug, name, expected)
			}
		}
	}
}

func TestRussianSubraceAndInstrumentNamesMatchReferences(t *testing.T) {
	wants := map[string]map[string]string{
		"subraces.json": {
			"high-elf": "Высший эльф", "hill-dwarf": "Холмовой дварф",
			"lightfoot-halfling": "Легконогий полурослик", "rock-gnome": "Скальный гном",
		},
		"equipment.json": {
			"bagpipes": "Волынка", "dulcimer": "Цимбалы", "horn": "Рожок",
			"pan-flute": "Свирель", "shawm": "Шалмей",
		},
		"proficiencies.json": {
			"bagpipes": "Волынка", "dulcimer": "Цимбалы", "horn": "Рожок",
			"pan-flute": "Свирель", "shawm": "Шалмей",
		},
	}
	for file, want := range wants {
		path := filepath.Join("..", "..", "data", "translations", "ru", file)
		entries, ok := readTranslationJSON(t, path).(map[string]any)
		if !ok {
			t.Fatalf("%s is not an object", path)
		}
		for slug, expected := range want {
			entry, ok := entries[slug].(map[string]any)
			if !ok {
				t.Errorf("%s: %s is missing", file, slug)
				continue
			}
			if name, _ := entry["name"].(string); name != expected {
				t.Errorf("%s: %s name = %q, want %q", file, slug, name, expected)
			}
		}
	}
}

func TestBackgroundFeaturesAreCatalogEntries(t *testing.T) {
	root := filepath.Join("..", "..", "data", "srd_5.1")
	backgrounds, ok := readTranslationJSON(t, filepath.Join(root, "backgrounds.json")).([]any)
	if !ok {
		t.Fatal("backgrounds.json is not an array")
	}
	features, ok := readTranslationJSON(t, filepath.Join(root, "features.json")).([]any)
	if !ok {
		t.Fatal("features.json is not an array")
	}
	available := map[string]bool{}
	for _, raw := range features {
		entry, _ := raw.(map[string]any)
		slug, _ := entry["slug"].(string)
		available[slug] = true
	}
	for _, raw := range backgrounds {
		entry, _ := raw.(map[string]any)
		feature, _ := entry["feature"].(string)
		if feature != "" && !available[feature] {
			t.Errorf("background feature %q has no features catalog entry", feature)
		}
	}
}

func readTranslationJSON(t *testing.T, path string) any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return value
}

func translationLeaves(value any, path string, out map[string]string) {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			translationLeaves(child, path+"/"+key, out)
		}
	case []any:
		for i, child := range value {
			translationLeaves(child, path+"/"+strconv.Itoa(i), out)
		}
	case string:
		out[path] = value
	case nil, bool, float64:
	default:
		panic(fmt.Sprintf("unexpected JSON value %T", value))
	}
}

// The files the two prose checks below cover: every collection whose Russian
// carries prose with numbers in it.
//
// Adding a file here is a promise that its Russian has been brought up to the
// same standard, so a new collection starts outside the list and joins it once
// somebody has done that work.
var proseChecked = []string{
	"spells.json", "magic-items.json", "features.json", "equipment.json", "traits.json",
}

// Unit words only, never a fragment. "диаметр" and "периметр" are ordinary
// Russian words that spell prose uses, and RE2 has no lookbehind, so the
// preceding character is matched and checked instead of assumed.
//
// The abbreviations are here because the word list alone missed five leaves:
// a donkey carried "190 кг" and a cube of force measured "2,5 см", which are
// the same fault spelled shorter.
var metricRE = regexp.MustCompile(`(?i)(` +
	// Spelled out, anywhere.
	`(^|[^а-яёa-z])(метр(ов|ах|ам|е|а|ы)?|сантиметр\p{Cyrillic}*|миллиметр\p{Cyrillic}*|` +
	`километр\p{Cyrillic}*|килограмм(ов|а)?|грамм(ов|а)?)([^а-яёa-z]|$)` +
	`|` +
	// Abbreviated, but only straight after a number. A unit always follows a
	// quantity, and that is what separates "2,5 см" from "(см. главу 5)" --
	// "см." is also how Russian abbreviates "смотри", see, which occurs five
	// times in this corpus and is not a measurement at all.
	`\d\s*(кг|км|см|мм)([^а-яёa-z]|$)` +
	`)`)

// Numbers and dice. Dice first so "8d6" is one token and not an 8 and a 6, and
// `\d*d` so the SRD's bare "d4" counts as a die.
var numberRE = regexp.MustCompile(`\d*d\d+|\d+`)

// Thousands separators, which the two languages write differently: English
// "1,500 gp" against Russian "1 500 зм". Both had to be handled -- the comma
// alone still read the Russian as a 1 and a 500.
//
// Exactly three digits and a word boundary, which is what keeps this off the
// Russian decimal comma: "2,5 см" has one digit after it and stays two tokens.
var thousandsRE = regexp.MustCompile(`(\d)[,\x{00A0}\x{202F} ](\d\d\d)([^\d]|$)`)

// Measurements in prose stay imperial.
//
// The client renders a spell's structured range as "футов" (spell.range.feet
// in web/locales/ru.json). Prose that converts to metres therefore prints
// "36 метров" in a description sitting directly beneath "120 футов" in the
// facts panel of the same spell -- which is what it used to do, in 39 places.
func TestRussianProseKeepsImperialUnits(t *testing.T) {
	for _, name := range proseChecked {
		leaves := russianLeaves(t, name)
		for _, path := range sortedKeys(leaves) {
			if m := strings.TrimSpace(metricRE.FindString(leaves[path])); m != "" {
				t.Errorf("%s%s converts a measurement to metric (%q); prose keeps feet and pounds", name, path, m)
			}
		}
	}
}

// Every number and every die in the English prose survives translation.
//
// This corpus is rules text. "8d6" coming back as "6d8", or a 120-foot range
// as 12, is a wrong rule rather than a clumsy sentence, and nothing else in
// the build would notice. cmd/llm applies the same rule to a model's response;
// this one covers the hand edits that never pass through cmd/llm at all.
func TestRussianProseKeepsEveryNumber(t *testing.T) {
	englishDir := filepath.Join("..", "..", "data", "srd_5.1", "i18n", "en")
	for _, name := range proseChecked {
		en := map[string]string{}
		translationLeaves(readTranslationJSON(t, filepath.Join(englishDir, name)), "", en)
		ru := russianLeaves(t, name)
		for _, path := range sortedKeys(en) {
			translated, ok := ru[path]
			if !ok || translated == "" {
				continue // absence is the coverage test's business, not this one's
			}
			want, got := numberSet(en[path]), numberSet(translated)
			if !slices.Equal(want, got) {
				t.Errorf("%s%s: English has %v, Russian has %v", name, path, want, got)
			}
		}
	}
}

// numberSet is the distinct numbers and dice in a string, sorted.
//
// A set rather than a tally, because English writes an area as "5 feet square"
// and Russian writes it as "5 на 5 футов": the idiom repeats the number, so
// counting occurrences rejects correct translations. A number that changes,
// transposes or vanishes still changes the set, which is what this guards.
func numberSet(s string) []string {
	for thousandsRE.MatchString(s) {
		s = thousandsRE.ReplaceAllString(s, "${1}${2}${3}")
	}
	return slices.Compact(slices.Sorted(slices.Values(numberRE.FindAllString(s, -1))))
}

func russianLeaves(t *testing.T, name string) map[string]string {
	t.Helper()
	path := filepath.Join("..", "..", "data", "translations", "ru", name)
	out := map[string]string{}
	translationLeaves(readTranslationJSON(t, path), "", out)
	return out
}

// Sorted so a failing run names the same leaf first every time.
func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
