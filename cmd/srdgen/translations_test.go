package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
