package file

// Prose is everything about a catalogue entry that changes with language.
//
// The shape is deliberately generic rather than one struct per entity type.
// Most entries need only Name and Desc; the handful that carry more -- a
// race's age and alignment paragraphs, a spell's material component, a
// subclass's flavour name -- put them in Fields or Blocks under the key
// constants below. That keeps the merge and the per-key fallback to one
// implementation instead of twenty.
type Prose struct {
	// Name is the display name. It is the one field every entry has.
	Name string `json:"name"`

	// Desc is the description, one string per paragraph.
	Desc []string `json:"desc,omitempty"`

	// Fields holds single-line prose keyed by the Prose* constants.
	Fields map[string]string `json:"fields,omitempty"`

	// Blocks holds multi-paragraph prose keyed by the Prose* constants.
	Blocks map[string][]string `json:"blocks,omitempty"`
}

// Keys for Prose.Fields.
const (
	ProseFullName     = "fullName"     // an ability's unabbreviated name
	ProseAbbreviation = "abbreviation" // an alignment's short form
	ProseScript       = "script"       // a language's writing system
	ProseFlavor       = "flavor"       // what a class calls its subclasses
	ProseMaterial     = "material"     // a spell's material component
	ProseCapacity     = "capacity"     // a vehicle's carrying capacity
	ProseSpeedUnit    = "speedUnit"    // a vehicle's speed unit
)

// Keys for Prose.Blocks.
const (
	ProseAge             = "age"             // a race's lifespan paragraph
	ProseAlignment       = "alignment"       // a race's alignment paragraph
	ProseSizeDesc        = "sizeDescription" // a race's size paragraph
	ProseLanguageDesc    = "languageDesc"    // a race's language paragraph
	ProseTypicalSpeakers = "typicalSpeakers" // who speaks a language
	ProseHigherLevel     = "higherLevel"     // a spell's At Higher Levels text
	ProseSpellcasting    = "spellcasting"    // a class's spellcasting rules text
)

// Bundle is one locale's prose for one collection, keyed by slug.
type Bundle map[string]Prose

// Field returns a single-line prose field, or "" when absent.
func (p Prose) Field(key string) string { return p.Fields[key] }

// Block returns a multi-paragraph prose field, or nil when absent.
func (p Prose) Block(key string) []string { return p.Blocks[key] }

// resolve merges a fallback bundle under a preferred one, key by key.
//
// The fallback is per *key*, not per *entry*: a locale that has translated a
// spell's name but not its description gets the translated name and the
// English text, rather than being pushed wholesale back to English. Partial
// translations are the normal state of a growing locale, so this is the case
// that has to work well.
func resolve(preferred, fallback Bundle) Bundle {
	if preferred == nil {
		return fallback
	}
	merged := make(Bundle, len(fallback))
	for slug, base := range fallback {
		merged[slug] = base
	}
	for slug, over := range preferred {
		base, ok := merged[slug]
		if !ok {
			merged[slug] = over
			continue
		}
		merged[slug] = mergeProse(base, over)
	}
	return merged
}

// mergeProse overlays over onto base, taking each populated field from over.
func mergeProse(base, over Prose) Prose {
	out := base
	if over.Name != "" {
		out.Name = over.Name
	}
	if len(over.Desc) > 0 {
		out.Desc = over.Desc
	}
	if len(over.Fields) > 0 {
		out.Fields = mergeMap(base.Fields, over.Fields)
	}
	if len(over.Blocks) > 0 {
		out.Blocks = mergeMap(base.Blocks, over.Blocks)
	}
	return out
}

// mergeMap overlays over onto base without mutating either.
func mergeMap[V any](base, over map[string]V) map[string]V {
	out := make(map[string]V, len(base)+len(over))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}
