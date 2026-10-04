package spellicon

import "strings"

// schoolPalettes assigns one glow color per school, so the set reads as a
// system rather than hundreds of separate commissions. The words are prompt
// vocabulary, not UI tokens; the art direction is Solasta-style spellbook
// runes -- a single freehand line lit like a neon sign, kept on a transparent
// background because the client draws the glyph over its own dark rows.
var schoolPalettes = map[string]string{
	"abjuration":    "golden-white",
	"conjuration":   "emerald-teal",
	"divination":    "ice-cyan",
	"enchantment":   "magenta-pink",
	"evocation":     "fiery orange-red",
	"illusion":      "violet-purple",
	"necromancy":    "sickly green",
	"transmutation": "amber-gold",
}

// Prompt is the art direction shared by the service queue and the offline
// `llm spell-prompts` batch. Keep it in lockstep with the palette above:
// whoever changes the prompt changes it for every icon, here and nowhere
// else.
//
// Catalogue values are namespaced -- a pack school arrives as
// "dnd-2014/conjuration" -- while the palettes above name the bare schools,
// so the lookup always takes the last segment.
func Prompt(name, school string) string {
	palette, ok := schoolPalettes[school[strings.LastIndexByte(school, '/')+1:]]
	if !ok {
		palette = "muted arcane"
	}
	return "Hand-drawn glowing rune icon for the D&D spell \"" + name + "\", " +
		"in the style of a fantasy RPG spellbook pictogram: one continuous " +
		"freehand line like a chalk sigil lit from within, " + palette + " neon " +
		"light with a brighter core and a faint halo around the strokes, " +
		"slightly rough edges, meant to glow against a dark background. " +
		"Single centered subject, readable at small size. " +
		"No text, no letters, no border, no frame, transparent background."
}
