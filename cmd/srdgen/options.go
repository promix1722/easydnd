package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/promix1722/easydnd/internal/adapter/catalog/file"
)

// refKindByURL maps an upstream API path segment to the wire's reference kind.
//
// Deriving the kind from the URL rather than from context is what lets a bare
// {index, name, url} triple become a typed reference: "skill-acrobatics" is a
// proficiency, "acrobatics" is a skill, and only the URL distinguishes them.
var refKindByURL = map[string]string{
	"ability-scores":       "ability",
	"alignments":           "alignment",
	"backgrounds":          "background",
	"classes":              "class",
	"conditions":           "condition",
	"damage-types":         "damage-type",
	"equipment":            "item",
	"equipment-categories": "equipment-category",
	"feats":                "feat",
	"features":             "feature",
	"languages":            "language",
	"magic-items":          "magic-item",
	"magic-schools":        "magic-school",
	"proficiencies":        "proficiency",
	"races":                "race",
	"skills":               "skill",
	"spells":               "spell",
	"subclasses":           "subclass",
	"subraces":             "subrace",
	"traits":               "trait",
	"weapon-properties":    "weapon-property",
}

// ref turns an upstream reference into the wire's "kind:slug" form.
func (g *generator) ref(r apiRef) file.Ref {
	kind := kindFromURL(r.URL)
	if kind == "" {
		g.warnf("cannot type reference %q (url %q)", r.Index, r.URL)
		return ""
	}
	return file.Ref(kind + ":" + r.Index)
}

// kindFromURL extracts the collection segment of an upstream URL.
func kindFromURL(url string) string {
	parts := strings.Split(strings.Trim(url, "/"), "/")
	// "api/2014/<collection>/<index>"; monsters and rules are vendored but
	// not modelled, and fall through to the empty string.
	if len(parts) < 3 {
		return ""
	}
	return refKindByURL[parts[2]]
}

// choiceKindFor maps upstream's loose `type` field onto the wire's vocabulary.
var choiceKindFor = map[string]string{
	"proficiencies":      "proficiency",
	"proficiency":        "proficiency",
	"languages":          "language",
	"language":           "language",
	"equipment":          "equipment",
	"ability_bonuses":    "ability-bonus",
	"ability-scores":     "ability",
	"spell":              "spell",
	"spells":             "spell",
	"trait":              "trait",
	"traits":             "trait",
	"feature":            "feature",
	"features":           "feature",
	"ideals":             "ideal",
	"expertise":          "expertise",
	"damage":             "damage",
	"action":             "action",
	"attack":             "attack",
	"string":             "text",
	"personality_traits": "personality",
	"bonds":              "bond",
	"flaws":              "flaw",
}

// choice converts an upstream choice, giving it a stable prompt id.
//
// The prompt is what a character's stored answer refers to, so it must be
// derived from position rather than from any text that a later data refresh
// might reword. prefix names the owning entry and field; index disambiguates
// siblings.
func (g *generator) choice(up *upChoice, prefix string, index int) *file.Choice {
	if up == nil {
		return nil
	}
	c := g.choiceValue(*up, prefix, index)
	return &c
}

func (g *generator) choiceValue(up upChoice, prefix string, index int) file.Choice {
	prompt := fmt.Sprintf("%s/%d", prefix, index)
	// An unmapped type is reported rather than slugified: inventing a kind
	// the loader has never heard of would fail at load with no clue where it
	// came from.
	kind := choiceKindFor[up.Type]
	if kind == "" && up.Type != "" {
		g.warnf("unknown choice type %q at %s", up.Type, prompt)
	}
	return file.Choice{
		Prompt: prompt,
		Choose: up.Choose,
		Kind:   kind,
		From:   g.optionSet(up.From, prompt),
	}
}

func (g *generator) choices(ups []upChoice, prefix string) []file.Choice {
	if len(ups) == 0 {
		return nil
	}
	out := make([]file.Choice, 0, len(ups))
	for i, up := range ups {
		out = append(out, g.choiceValue(up, prefix, i))
	}
	return out
}

func (g *generator) optionSet(up upOptionSet, prompt string) file.OptionSet {
	switch up.OptionSetType {
	case "options_array":
		set := file.OptionSet{Kind: file.OptionSetExplicit}
		for i, raw := range up.Options {
			opt, ok := g.option(raw, prompt, i)
			if ok {
				set.Options = append(set.Options, opt)
			}
		}
		return set
	case "equipment_category":
		category := ""
		if up.EquipmentCategory != nil {
			category = up.EquipmentCategory.Index
		}
		return file.OptionSet{Kind: file.OptionSetEquipmentCategory, Category: category}
	case "resource_list":
		return file.OptionSet{
			Kind:       file.OptionSetCollection,
			Collection: kindFromURL(up.ResourceListURL),
		}
	default:
		g.warnf("unknown option_set_type %q at %s", up.OptionSetType, prompt)
		return file.OptionSet{Kind: file.OptionSetExplicit}
	}
}

// option converts one upstream option. The bool reports whether it produced
// anything; a `string` option carries no mechanics and is dropped.
func (g *generator) option(raw json.RawMessage, prompt string, index int) (file.Option, bool) {
	// The schema types an options array as (Option | string)[]: the ranger's
	// favored enemy and natural explorer lists are bare strings with no
	// mechanical payload at all.
	var bare string
	if err := json.Unmarshal(raw, &bare); err == nil {
		key := slugify(bare)
		g.text(key, bare)
		return file.Option{Kind: file.OptionText, Key: key}, true
	}

	var head struct {
		OptionType string `json:"option_type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		g.warnf("undecodable option at %s: %v", prompt, err)
		return file.Option{}, false
	}

	switch head.OptionType {
	case "reference":
		var v struct {
			Item apiRef `json:"item"`
		}
		mustDecode(raw, &v)
		return file.Option{Kind: file.OptionRef, Ref: g.ref(v.Item), Count: 1}, true

	case "counted_reference":
		var v struct {
			Count int    `json:"count"`
			Of    apiRef `json:"of"`
		}
		mustDecode(raw, &v)
		return file.Option{Kind: file.OptionRef, Ref: g.ref(v.Of), Count: v.Count}, true

	case "choice":
		var v struct {
			Choice upChoice `json:"choice"`
		}
		mustDecode(raw, &v)
		nested := g.choiceValue(v.Choice, prompt, index)
		return file.Option{Kind: file.OptionNested, Choice: &nested}, true

	case "multiple":
		var v struct {
			Items []json.RawMessage `json:"items"`
		}
		mustDecode(raw, &v)
		opt := file.Option{Kind: file.OptionBundle}
		// A bundle's items are namespaced under the bundle's own index.
		// Without that, a nested choice inside bundle option 1 and a nested
		// choice that *is* option 0 both derive "<prompt>/0" and the two
		// become indistinguishable -- which is exactly what happened to the
		// rogue's Expertise, whose two branches ("two skills" and "one skill
		// plus thieves' tools") shared the id rogue-expertise-1/expertise/0/0
		// while asking for different numbers of picks.
		inner := fmt.Sprintf("%s/%d", prompt, index)
		for i, item := range v.Items {
			sub, ok := g.option(item, inner, i)
			if ok {
				opt.Items = append(opt.Items, sub)
			}
		}
		return opt, true

	case "ability_bonus":
		var v struct {
			AbilityScore apiRef `json:"ability_score"`
			Bonus        int    `json:"bonus"`
		}
		mustDecode(raw, &v)
		return file.Option{Kind: file.OptionAbilityBonus, Ability: v.AbilityScore.Index, Bonus: v.Bonus}, true

	case "score_prerequisite":
		var v struct {
			AbilityScore apiRef `json:"ability_score"`
			MinimumScore int    `json:"minimum_score"`
		}
		mustDecode(raw, &v)
		return file.Option{Kind: file.OptionScoreMinimum, Ability: v.AbilityScore.Index, Minimum: v.MinimumScore}, true

	case "damage":
		var v struct {
			DamageDice string  `json:"damage_dice"`
			DamageType *apiRef `json:"damage_type"`
			Notes      string  `json:"notes"`
		}
		mustDecode(raw, &v)
		opt := file.Option{Kind: file.OptionDamage, Dice: v.DamageDice}
		if v.DamageType != nil {
			opt.DamageType = v.DamageType.Index
		}
		return opt, true

	case "breath":
		// A dragonborn breath weapon: a named area attack with its own save.
		// It is stored as a damage option keyed by ancestry, since that is
		// all the character sheet needs to show.
		var v struct {
			Name   string `json:"name"`
			Damage []struct {
				DamageType        *apiRef           `json:"damage_type"`
				DamageAtCharLevel map[string]string `json:"damage_at_character_level"`
			} `json:"damage"`
		}
		mustDecode(raw, &v)
		opt := file.Option{Kind: file.OptionDamage, Key: slugify(v.Name)}
		if len(v.Damage) > 0 {
			if v.Damage[0].DamageType != nil {
				opt.DamageType = v.Damage[0].DamageType.Index
			}
			// Level 1 is the base die; the rest is class-level scaling that
			// the trait's prose already states.
			opt.Dice = v.Damage[0].DamageAtCharLevel["1"]
		}
		return opt, true

	case "action":
		var v struct {
			ActionName string `json:"action_name"`
			Count      int    `json:"count"`
			Type       string `json:"type"`
		}
		mustDecode(raw, &v)
		return file.Option{
			Kind:     file.OptionAction,
			Key:      slugify(v.ActionName),
			Count:    v.Count,
			Recharge: slugify(v.Type),
		}, true

	case "ideal":
		var v struct {
			Alignments []apiRef `json:"alignments"`
			Desc       string   `json:"desc"`
		}
		mustDecode(raw, &v)
		opt := file.Option{Kind: file.OptionText, Key: slugify(firstWords(v.Desc, 6))}
		for _, a := range v.Alignments {
			opt.Alignments = append(opt.Alignments, a.Index)
		}
		g.text(opt.Key, v.Desc)
		return opt, true

	case "string":
		// Free text with no mechanical content: a background's suggested
		// personality trait. It becomes a text option whose prose lives in
		// the locale bundle.
		var v struct {
			String string `json:"string"`
		}
		mustDecode(raw, &v)
		key := slugify(firstWords(v.String, 6))
		g.text(key, v.String)
		return file.Option{Kind: file.OptionText, Key: key}, true

	default:
		g.warnf("unknown option_type %q at %s", head.OptionType, prompt)
		return file.Option{}, false
	}
}

// mustDecode re-decodes a raw option once its type is known. The outer decode
// already proved the bytes are valid JSON, so a failure here would mean a
// field type mismatch, which the caller's warning path already covers.
func mustDecode(raw json.RawMessage, into any) {
	_ = json.Unmarshal(raw, into)
}

// firstWords takes the opening words of a sentence, for use as a slug.
func firstWords(s string, n int) string {
	fields := strings.Fields(s)
	if len(fields) > n {
		fields = fields[:n]
	}
	return strings.Join(fields, " ")
}
