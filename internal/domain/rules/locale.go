package rules

// Locale is a language tag naming one translation of the catalogue's prose.
//
// Localization is a property of *prose*, never of mechanics. A spell's level,
// range and damage are the same in every language; only its name and
// description differ. That split is why the on-disk data keeps mechanics in
// language-neutral files and prose in per-locale overlays, and why a resolved
// *catalog.Catalog carries no locale-keyed maps: it has already picked one.
type Locale string

// The locales the catalogue ships.
const (
	LocaleEN Locale = "en"
	LocaleRU Locale = "ru"
)

// DefaultLocale is the fallback for any key a locale has not translated.
// English is the language SRD 5.1 is published in, so it is the only locale
// guaranteed to be complete.
const DefaultLocale = LocaleEN

// String returns the language tag.
func (l Locale) String() string { return string(l) }

// SupportedLocales lists the locales the catalogue may contain, most complete
// first. Callers must not mutate the returned slice.
func SupportedLocales() []Locale { return []Locale{LocaleEN, LocaleRU} }

// IsSupported reports whether l is a locale the catalogue knows how to load.
func (l Locale) IsSupported() bool {
	for _, known := range SupportedLocales() {
		if l == known {
			return true
		}
	}
	return false
}
