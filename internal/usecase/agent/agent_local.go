package agent

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

func folded(text string) string { return strings.ToLower(strings.TrimSpace(text)) }

func foldedIn(list []string, text string) bool {
	return text != "" && slices.ContainsFunc(list, func(entry string) bool { return folded(entry) == folded(text) })
}

// sameFolded reports that two lists hold the same entries, in any order.
func sameFolded(a, b []string) bool {
	return len(a) == len(b) && !slices.ContainsFunc(a, func(entry string) bool { return !foldedIn(b, entry) })
}

// localKey drops the pinned packs' namespaces from an option key. A key is
// not always a slug -- it can be a prompt id or a bundle of several entries --
// so only a prefix that is a pack's own name is taken off.
func localKey(cat *catalog.Catalog, key string) string {
	for _, release := range cat.Lock.Packs {
		key = strings.ReplaceAll(key, release.ID+"/", "")
	}
	return key
}

func localKeys(cat *catalog.Catalog, keys []rules.Slug) []string {
	out := make([]string, len(keys))
	for i, key := range keys {
		out[i] = localKey(cat, key.String())
	}
	return out
}

func localSlug(slug rules.Slug) string {
	if _, local, qualified := strings.Cut(slug.String(), "/"); qualified {
		return local
	}
	return slug.String()
}

// localPath drops pack namespaces from a path for display. Every tool accepts
// the local form back, so a model never has to carry "dnd-2014/" around.
func localPath(path string) string {
	parts := strings.Split(path, ".")
	for i, part := range parts {
		parts[i] = localSlug(rules.Slug(part))
	}
	return strings.Join(parts, ".")
}

func localValue(value any) any {
	switch v := value.(type) {
	case rules.Slug:
		return localSlug(v)
	case []rules.Slug:
		out := make([]string, len(v))
		for i, slug := range v {
			out[i] = localSlug(slug)
		}
		return out
	}
	return value
}

// spokenIn refuses what the assistant is about to say to the owner when it is
// not in the language they chose for the interface. The instructions say so
// and were not followed: a Russian sheet read in an English chat ended in a
// Russian summary. It is told by the script, which is all that separates the
// two languages shipped, and by the majority of letters, so that a name
// quoted from the sheet does not count against a sentence.
func spokenIn(locale rules.Locale, text string) error {
	cyrillic, latin := 0, 0
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Cyrillic, r):
			cyrillic++
		case unicode.Is(unicode.Latin, r):
			latin++
		}
	}
	switch {
	case locale == "en" && cyrillic > latin:
		return fmt.Errorf("not sent: write this in English, the language the user chose for the interface, whatever language the sources are in. Names from the sheet stay as printed")
	case locale == "ru" && latin > cyrillic:
		return fmt.Errorf("not sent: write this in Russian, the language the user chose for the interface, whatever language the sources are in. Names from the sheet stay as printed")
	}
	return nil
}
