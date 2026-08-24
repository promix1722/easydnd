package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/promix1722/easydnd/internal/adapter/catalog/file"
)

// The SRD prints casting time, range and duration as English phrases. They are
// mechanics wearing prose clothing: "90 feet" has to be comparable, filterable
// and translatable, and none of that works on a string.
//
// The parsers below normalise them. The value space is small enough to have
// been enumerated in full across all 319 SRD spells -- nine casting times,
// seventeen ranges, nineteen durations -- so anything they cannot read is a
// genuine surprise and is reported rather than silently degraded.

// feetPerMile converts the two spells stated in miles.
const feetPerMile = 5280

// parseCastingTime reads "1 action", "1 bonus action", "10 minutes".
func parseCastingTime(s string) (file.CastingTime, error) {
	text := strings.ToLower(strings.TrimSpace(s))
	switch text {
	case "1 action":
		return file.CastingTime{Kind: file.CastAction, Amount: 1}, nil
	case "1 bonus action":
		return file.CastingTime{Kind: file.CastBonusAction, Amount: 1}, nil
	case "1 reaction":
		return file.CastingTime{Kind: file.CastReaction, Amount: 1}, nil
	}
	amount, unit, err := parseAmountUnit(text)
	if err != nil {
		return file.CastingTime{}, fmt.Errorf("casting time %q: %w", s, err)
	}
	return file.CastingTime{Kind: file.CastOverTime, Amount: amount, Unit: unit}, nil
}

// parseSpellRange reads "Touch", "Self", "60 feet", "1 mile", "Sight".
func parseSpellRange(s string) (file.SpellRange, error) {
	text := strings.ToLower(strings.TrimSpace(s))
	switch text {
	case "self":
		return file.SpellRange{Kind: file.RangeSelf}, nil
	case "touch":
		return file.SpellRange{Kind: file.RangeTouch}, nil
	case "sight":
		return file.SpellRange{Kind: file.RangeSight}, nil
	case "unlimited":
		return file.SpellRange{Kind: file.RangeUnlimited}, nil
	case "special":
		return file.SpellRange{Kind: file.RangeSpecial}, nil
	}

	amount, unit, found := strings.Cut(text, " ")
	if !found {
		return file.SpellRange{}, fmt.Errorf("range %q: no unit", s)
	}
	n, err := strconv.Atoi(amount)
	if err != nil {
		return file.SpellRange{}, fmt.Errorf("range %q: %w", s, err)
	}
	switch strings.TrimSuffix(unit, "s") {
	case "foot", "feet":
		// "feet" does not lose a trailing s; handle both spellings.
		return file.SpellRange{Kind: file.RangeDistance, Distance: n}, nil
	case "mile":
		return file.SpellRange{Kind: file.RangeDistance, Distance: n * feetPerMile}, nil
	default:
		if unit == "feet" {
			return file.SpellRange{Kind: file.RangeDistance, Distance: n}, nil
		}
		return file.SpellRange{}, fmt.Errorf("range %q: unknown unit %q", s, unit)
	}
}

// parseDuration reads "Instantaneous", "Up to 1 minute", "Until dispelled".
func parseDuration(s string) (file.Duration, error) {
	text := strings.ToLower(strings.TrimSpace(s))
	switch text {
	case "instantaneous":
		return file.Duration{Kind: file.DurationInstantaneous}, nil
	case "until dispelled":
		return file.Duration{Kind: file.DurationUntilDispelled}, nil
	case "special":
		return file.Duration{Kind: file.DurationSpecial}, nil
	}

	upTo := strings.HasPrefix(text, "up to ")
	text = strings.TrimPrefix(text, "up to ")

	amount, unit, err := parseAmountUnit(text)
	if err != nil {
		return file.Duration{}, fmt.Errorf("duration %q: %w", s, err)
	}
	return file.Duration{Kind: file.DurationTimed, Amount: amount, Unit: unit, UpTo: upTo}, nil
}

// parseAmountUnit reads "10 minutes" into (10, "minute").
func parseAmountUnit(text string) (int, string, error) {
	amount, unit, found := strings.Cut(text, " ")
	if !found {
		return 0, "", fmt.Errorf("no unit in %q", text)
	}
	n, err := strconv.Atoi(amount)
	if err != nil {
		return 0, "", fmt.Errorf("bad amount in %q: %w", text, err)
	}
	switch strings.TrimSuffix(unit, "s") {
	case "round":
		return n, file.UnitRound, nil
	case "minute":
		return n, file.UnitMinute, nil
	case "hour":
		return n, file.UnitHour, nil
	case "day":
		return n, file.UnitDay, nil
	default:
		return 0, "", fmt.Errorf("unknown unit %q", unit)
	}
}

// slugify normalises upstream's free-text categories -- "Simple Melee",
// "Tack, Harness, and Drawn Vehicles" -- into lower-kebab slugs.
func slugify(s string) string {
	// Apostrophes are dropped rather than treated as separators, so
	// "Artisan's Tools" becomes "artisans-tools" and not "artisan-s-tools".
	text := strings.NewReplacer("'", "", "\u2019", "").Replace(strings.ToLower(strings.TrimSpace(s)))

	var b strings.Builder
	lastDash := true
	for _, r := range text {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
