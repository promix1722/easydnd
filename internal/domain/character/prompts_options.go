package character

import (
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// refOptions builds an explicit option set naming catalogue entries.
func refOptions(kind rules.RefKind, slugs []rules.Slug) rules.OptionSet {
	options := make([]rules.Option, 0, len(slugs))
	for _, slug := range slugs {
		options = append(options, rules.RefOption{Ref: rules.Ref{Kind: kind, Slug: slug}, Count: 1})
	}
	return rules.OptionSet{Kind: rules.OptionsExplicit, Options: options}
}

// heldIn reports which of a prompt's options the character already has.
//
// Only the cases where a duplicate is actually illegal are reported:
// proficiencies, languages, traits, feats, and features (including fighting
// styles shared by different classes). A second rapier is fine.
//
// It looks inside branches as well as at the options themselves, because the
// client answers a branch in the card that offered it -- so the options it
// draws are the branches' options, and the greying-out has to reach them. The
// monk's "one artisan's tool or one musical instrument" is the case: without
// this, a tool the character already had looked pickable and the server
// refused the answer. Option keys are unique within a prompt, so one flat list
// still says which option each held entry is.
func (b *promptBuilder) heldIn(c rules.Choice) []rules.Slug {
	var held []rules.Slug
	var walk func(options []rules.Option)
	walk = func(options []rules.Option) {
		for _, option := range options {
			switch opt := option.(type) {
			case rules.RefOption:
				if b.holds(opt.Ref) {
					held = append(held, rules.OptionKey(option))
				}
			case rules.NestedOption:
				walk(opt.Choice.From.Options)
			case rules.BundleOption:
				walk(opt.Items)
			}
		}
	}
	walk(c.From.Options)
	if c.From.Kind == rules.OptionsFromCollection && c.From.Collection == rules.RefLanguage {
		held = append(held, b.state.Base.Languages...)
	}
	if c.From.Kind == rules.OptionsFromCollection && c.From.Collection == rules.RefFeat {
		held = append(held, b.state.Feats...)
	}
	return held
}

// Class-specific fighting-style entries describe the same non-repeatable benefit.
func featureIdentity(slug rules.Slug) string {
	s := slug.String()
	if at := strings.Index(s, "fighting-style-"); at >= 0 {
		return s[at:]
	}
	return s
}

func (b *promptBuilder) expertiseEligible(c rules.Choice, held []rules.Slug) []rules.Slug {
	var out []rules.Slug
	for _, slug := range held {
		skill := slug
		if def, ok := b.cat.Proficiencies.Get(slug); ok {
			skill = def.Reference.Slug
		}
		if b.state.Skills.BySkill[skill].Proficiency == rules.Expertise {
			continue
		}
		used := false
		for _, feature := range b.state.Features {
			def, ok := b.cat.Features.Get(feature)
			if !ok || def.Specific == nil {
				continue
			}
			ch := oneList(def.Specific.ExpertiseOptions)
			if ch != nil && ch.Prompt != c.Prompt && slices.Contains(b.answers.slugs(ch), slug) {
				used = true
			}
		}
		if !used {
			out = append(out, slug)
		}
	}
	return out
}

func (b *promptBuilder) holds(ref rules.Ref) bool {
	switch ref.Kind {
	case rules.RefTrait:
		return slices.Contains(b.state.Traits, ref.Slug)
	case rules.RefFeat:
		return slices.Contains(b.state.Feats, ref.Slug)
	case rules.RefFeature:
		for _, held := range b.state.Features {
			if featureIdentity(held) == featureIdentity(ref.Slug) {
				return true
			}
		}
		return false
	case rules.RefLanguage:
		return slices.Contains(b.state.Base.Languages, ref.Slug)
	case rules.RefProficiency:
		if slices.Contains(b.state.Proficiencies, ref.Slug) {
			return true
		}
		def, ok := b.cat.Proficiencies.Get(ref.Slug)
		if !ok {
			return false
		}
		if def.Reference.Kind == rules.RefSkill {
			state, known := b.state.Skills.BySkill[def.Reference.Slug]
			return known && state.Proficiency != rules.NotProficient
		}
		return false
	case rules.RefSkill:
		state, known := b.state.Skills.BySkill[ref.Slug]
		return known && state.Proficiency != rules.NotProficient
	}
	return false
}

func (b *promptBuilder) packRules() {
	for _, r := range b.cat.Mechanics.Rules {
		active, err := activeRule(b.state, b.cat, r)
		if err != nil || !active {
			continue
		}
		for _, ch := range r.Choices {
			p := Prompt{Group: ruleGroup(r.Owner), Source: r.Owner, Level: b.featLevel(r.Owner), Event: PromptEvent{Type: EventRule, Ref: rules.NewRef(rules.RefRule, r.ID)}}
			// Expertise doubles a proficiency already held, whoever asks.
			p.HeldOnly = ch.Kind == rules.ChooseExpertise
			b.addChoice(&ch, p)
		}
	}
}

// ruleGroup files a rule's question with what owns the rule: a background's
// gaming set is asked with the background, not among the class levels.
func ruleGroup(owner rules.Ref) PromptGroup {
	switch owner.Kind {
	case rules.RefRace, rules.RefSubrace, rules.RefTrait:
		return GroupRace
	case rules.RefBackground:
		return GroupBackground
	}
	return GroupClass
}

func (b *promptBuilder) blockedIn(choice rules.Choice) []rules.Slug {
	var blocked []rules.Slug
	for _, requirement := range b.cat.Mechanics.ChoiceRequirements {
		if requirement.Prompt != choice.Prompt {
			continue
		}
		allowed := false
		for _, proficiency := range requirement.AnyProficiency {
			if b.holds(rules.NewRef(rules.RefProficiency, proficiency)) {
				allowed = true
			}
		}
		if !allowed {
			blocked = append(blocked, requirement.Pick)
		}
	}
	var walk func(rules.Option)
	walk = func(option rules.Option) {
		switch opt := option.(type) {
		case rules.RefOption:
			if opt.Ref.Kind == rules.RefFeature && !b.meets(opt.Ref.Slug) {
				blocked = append(blocked, rules.OptionKey(option))
			}
		case rules.NestedOption:
			blocked = append(blocked, b.blockedIn(opt.Choice)...)
		case rules.BundleOption:
			for _, item := range opt.Items {
				walk(item)
			}
		}
	}
	for _, option := range choice.From.Options {
		walk(option)
	}
	return blocked
}

// meets reports whether the character satisfies an offered feature's own
// prerequisites: Thirsting Blade's fifth level and Pact of the Blade.
//
// A level is read against the character as they stand now, not against the
// level whose prompt is offering the feature. That is what lets a twelfth-level
// warlock put Lifedrinker into the pick second level opened, and it is why
// there is no "replace an invocation" step: every earlier answer is editable,
// and what is legal in it is what is legal for the character today -- the same
// policy the spell prompts follow.
//
// The level is the one in the feature's own class when it names one, and a
// character with no level in that class meets nothing: Eldritch Adept gives a
// fighter an invocation, but only one without a prerequisite.
func (b *promptBuilder) meets(slug rules.Slug) bool {
	feature, ok := b.cat.Features.Get(slug)
	if !ok || len(feature.Prerequisites) == 0 {
		return true
	}
	level := b.state.Identity.Level()
	if !feature.Class.IsZero() {
		if level = ownerLevel(b.state, b.cat, rules.NewRef(rules.RefClass, feature.Class)); level == 0 {
			return false
		}
	}
	spells := b.state.Spells
	for _, p := range feature.Prerequisites {
		switch p.Kind {
		case catalog.PrerequisiteLevel:
			if level < p.Level {
				return false
			}
		case catalog.PrerequisiteAbility:
			if b.state.Abilities.Scores[p.Ability] < p.MinimumScore {
				return false
			}
		case catalog.PrerequisiteEntry:
			// Not through holds: that answers "would a second one be a
			// duplicate", and a spell held twice is not one.
			if p.Ref.Kind == rules.RefSpell {
				if !slices.Contains(spells.Cantrips, p.Ref.Slug) && !slices.Contains(spells.Known, p.Ref.Slug) && !slices.Contains(spells.Prepared, p.Ref.Slug) {
					return false
				}
			} else if !b.holds(p.Ref) {
				return false
			}
		}
	}
	return true
}
