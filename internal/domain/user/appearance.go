package user

// Appearance is a personal palette and light/dark preference.
type Appearance struct {
	Palette     string
	ColorScheme string
}

// WithDefaults fills the zero value used by older account constructors.
func (a Appearance) WithDefaults() Appearance {
	if a.Palette == "" {
		a.Palette = "dragon"
	}
	if a.ColorScheme == "" {
		a.ColorScheme = "auto"
	}
	return a
}

// Valid reports whether both fields are supported; writes require both.
func (a Appearance) Valid() bool {
	switch a.Palette {
	case "dragon", "parchment", "midnight", "moss":
	default:
		return false
	}
	switch a.ColorScheme {
	case "light", "dark", "auto":
		return true
	default:
		return false
	}
}
