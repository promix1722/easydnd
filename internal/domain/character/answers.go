package character

import "github.com/promix1722/easydnd/internal/domain/rules"

// answers is every choice the player has made, keyed by the prompt it
// answers.
//
// It is folded across the *whole* log, last write wins. That is what makes
// append-per-step work: a prompt posed by a trait -- the half-elf's
// skill-versatility/proficiency/0 -- does not exist until the race has been
// chosen, so its answer necessarily arrives in a later event than the race
// event. Rather than special-casing that, any event may carry an answer to
// any prompt.
//
// Last-write-wins is what makes a *later* event able to answer an *earlier*
// entry's prompt. It is not, and never was, a way to change a pick: a prompt
// that has been answered is no longer open, and Prompts stops emitting it, so
// posting it again is rejected as a prompt the character does not have. What
// changes a pick is replacing the entry that carries it -- see Rebuild and
// the usecase's Revise -- which is also what makes the fold's ordering
// irrelevant, because after a replace there is still exactly one answer to
// each prompt.
type answers map[rules.Slug][]rules.Slug

// foldAnswers collects the answers from a log in order.
func foldAnswers(log Log) answers {
	out := make(answers)
	for _, e := range log.Events {
		for _, a := range e.Choices {
			if a.Prompt.IsZero() {
				continue
			}
			out[a.Prompt] = a.Picks
		}
	}
	return out
}

// picks returns the answer to a prompt, or nil.
func (a answers) picks(prompt rules.Slug) []rules.Slug { return a[prompt] }

// answered reports whether a choice has been fully answered: an answer exists
// and supplies exactly as many picks as the prompt asks for.
//
// A partial answer counts as unanswered, so a half-finished equipment step is
// resumable rather than silently accepted.
func (a answers) answered(c rules.Choice) bool {
	got, ok := a[c.Prompt]
	if !ok {
		return false
	}
	return len(got) == c.Choose
}

// chosen walks a choice against the answers and calls visit for every option
// the player actually selected, descending through nested choices and
// bundles.
//
// Descending is the whole reason this is a walk rather than a lookup. The
// rogue's Expertise is "choose 1 of: two skills, or one skill plus thieves'
// tools" -- answering the outer prompt selects a branch, and the branch's own
// prompt is what carries the skills. A projector that only read the outer
// answer would grant no expertise at all.
//
// Options drawn from a collection or an equipment category are not listed
// inline; for those the pick *is* the slug, so visit receives a RefOption
// built from the set's kind.
func (a answers) chosen(c rules.Choice, visit func(rules.Option)) {
	if c.Prompt.IsZero() {
		return
	}
	picked := a.picks(c.Prompt)
	if len(picked) == 0 {
		return
	}

	if c.From.Kind != rules.OptionsExplicit {
		for _, slug := range picked {
			visit(rules.RefOption{Ref: rules.Ref{Kind: c.From.Collection, Slug: slug}, Count: 1})
		}
		return
	}

	for _, key := range picked {
		option, ok := rules.FindOption(c.From, key)
		if !ok {
			// An unresolvable key is reported by the caller that needs it,
			// not here: the same walk serves projection and validation, and
			// only one of those should fail on it.
			continue
		}
		a.visitOption(option, visit)
	}
}

// visitOption emits one selected option, flattening bundles and following
// nested choices into their own answers.
func (a answers) visitOption(o rules.Option, visit func(rules.Option)) {
	switch opt := o.(type) {
	case rules.NestedOption:
		a.chosen(opt.Choice, visit)
	case rules.BundleOption:
		for _, item := range opt.Items {
			a.visitOption(item, visit)
		}
	default:
		visit(o)
	}
}

// refs walks a choice and returns the catalogue references the player
// selected, ignoring options that name no entry.
func (a answers) refs(c rules.Choice) []rules.Ref {
	var out []rules.Ref
	a.chosen(c, func(o rules.Option) {
		if ref, ok := o.(rules.RefOption); ok && !ref.Ref.IsZero() {
			out = append(out, ref.Ref)
		}
	})
	return out
}

// slugs is refs reduced to the slugs, for the callers that already know the
// kind because the prompt only offers one.
func (a answers) slugs(c *rules.Choice) []rules.Slug {
	if c == nil {
		return nil
	}
	refs := a.refs(*c)
	out := make([]rules.Slug, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.Slug)
	}
	return out
}
