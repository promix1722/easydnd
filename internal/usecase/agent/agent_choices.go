package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// choiceOptions handles the list_choice_options tool.
func (a *Agent) choiceOptions(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	prompts, err := domain.Prompts(nativeLog(s.Log), cat)
	if err != nil {
		return nil, err
	}
	for _, p := range prompts {
		if promptNamed(cat, p, args.Prompt) {
			options := promptOptions(cat, p, catalogNames(cat))
			start := min(max(0, args.Offset), len(options))
			end := min(start+60, len(options))
			return map[string]any{"prompt": args.Prompt, "options": options[start:end], "offset": start, "total": len(options), "nextOffset": end}, nil
		}
	}
	return nil, fmt.Errorf("prompt not open")
}

// askUser handles the ask_user tool.
func (a *Agent) askUser(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	if s.Unattended {
		return nil, fmt.Errorf("the user asked not to be asked anything: leave this open, import what the sources state, and call prepare_review")
	}
	if strings.TrimSpace(args.Text) == "" {
		return nil, fmt.Errorf("question required")
	}
	if err := spokenIn(s.Locale, args.Text+" "+strings.Join(args.Options, " ")); err != nil {
		return nil, err
	}
	if len(args.Options) > 10 {
		return nil, fmt.Errorf("offer at most ten concise answers")
	}
	for _, option := range args.Options {
		if strings.TrimSpace(option) == "" || len(option) > 300 {
			return nil, fmt.Errorf("invalid answer option")
		}
	}
	open, err := a.openPrompts(s, cat)
	if err != nil {
		return nil, err
	}
	asking := agentQuestion{Text: args.Text, Options: append([]string(nil), args.Options...), Open: promptIDs(open)}
	// The same answers, offered again over the same open prompts, to an
	// owner who has just pressed one of them and said nothing since: that
	// is the question they answered, however it is reworded.
	if n := len(s.questions); n > 0 {
		last := s.questions[n-1]
		if justAnswered(s.Events) && foldedIn(last.Options, last.Answer) && sameFolded(asking.Options, last.Options) && sameFolded(asking.Open, last.Open) {
			return nil, fmt.Errorf("asked twice: the user has just answered this very question with %q. Never ask again what they have answered. Do as they said: what they chose to leave blank or open stays so, and prepare_review does not hold it against the draft", last.Answer)
		}
	}
	asking.About = questionAbout(cat, open, args.About, args.Options)
	// A question that follows a refused review is the one the refusal
	// asked for: about everything that was left.
	if s.offered {
		asking.About = asking.Open
	}
	if err := setStatus(s, "waiting"); err != nil {
		return nil, err
	}
	s.offered = false
	s.questions = append(s.questions, asking)
	addAgentEvent(s, "question", args.Text, "", nil)
	s.Events[len(s.Events)-1].Options = append([]string(nil), args.Options...)
	return map[string]bool{"waiting": true}, nil
}

// answerChoices handles the answer_choices tool.
func (a *Agent) answerChoices(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	var err error
	if len(args.Answers) > 100 {
		return nil, fmt.Errorf("provide 1-100 answers in the answers array")
	}
	results := []map[string]any{}
	for _, answer := range args.Answers {
		// An answer can add a custom entry or open the prompt the next one
		// names, so each is judged against the catalogue as it now stands.
		if cat, err = a.Catalog(ctx, *s); err != nil {
			return nil, err
		}
		picks, err := a.answer(ctx, s, cat, answer)
		if err != nil {
			a.service.Logger().Warn("AI wizard tool rejected", "tool", "answer_choices", "prompt", answer.Prompt, "error", err)
			failure := map[string]any{}
			_ = json.Unmarshal(agentError(err), &failure)
			failure["prompt"] = answer.Prompt
			results = append(results, failure)
			continue
		}
		results = append(results, map[string]any{"prompt": answer.Prompt, "applied": true, "picks": localKeys(cat, picks)})
	}
	open, err := a.openPrompts(s, cat)
	if err != nil {
		return nil, err
	}
	return map[string]any{"results": results, "open": open}, nil
}

// reviseChoice handles the revise_choice tool.
func (a *Agent) reviseChoice(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	var err error
	if args.Sequence <= 1 || args.Sequence > len(s.Log.Events) {
		return nil, fmt.Errorf("choice not open; re-read context")
	}
	event := s.Log.Events[args.Sequence-1]
	if event.Type == domain.EventNote {
		return nil, fmt.Errorf("this entry is import metadata, not a choice")
	}
	event.Seq = 0
	if args.Prompt != "" {
		// The prompt an entry answered is closed. It reopens on the log as it
		// stood with the entry's own answers taken back out, which is where
		// its options can be read to resolve a pick written as a name.
		before := domain.Log{Events: slices.Clone(s.Log.Events[:args.Sequence])}
		before.Events[args.Sequence-1].Choices = nil
		prompts, err := domain.Prompts(nativeLog(before), cat)
		if err != nil {
			return nil, err
		}
		picks := []rules.Slug{}
		for _, pick := range args.Picks {
			key := rules.Slug(pick)
			if i := slices.IndexFunc(prompts, func(p domain.Prompt) bool { return promptNamed(cat, p, args.Prompt) }); i >= 0 {
				if key, err = a.resolvePick(ctx, s, cat, cat.ResolveChoice(prompts[i].Choice), pick); err != nil {
					return nil, err
				}
			}
			picks = append(picks, key)
		}
		id := rules.Slug(args.Prompt)
		if i := slices.IndexFunc(prompts, func(p domain.Prompt) bool { return promptNamed(cat, p, args.Prompt) }); i >= 0 {
			id = prompts[i].Choice.Prompt
		}
		event.Choices = []domain.Answer{{Prompt: id, Picks: picks}}
	}
	if args.Ref != "" {
		found, err := a.resolve(ctx, s, cat, nil, args.Ref, nil, false)
		if err != nil {
			return nil, err
		}
		event.Ref, _ = rules.ParseRef(found.Ref)
		if strings.HasPrefix(args.Prompt, "character/") {
			event.Choices = nil
		}
	}
	log, dropped, err := charuc.Revise(s.Log, cat, args.Sequence, &event)
	if err != nil {
		return nil, err
	}
	s.Log = log
	return map[string]any{"dropped": dropped}, nil
}

// promptSlot is a prompt with the places it has, what it would take, and what
// it has been given.
type promptSlot struct {
	prompt domain.Prompt
	offers map[rules.Slug]bool
	picks  []rules.Slug
}

// matchPicks puts each wanted option into a prompt that offers it, and returns
// the ones no prompt had room for.
//
// A sheet says what a character has, never which source gave what, and the
// sources overlap: one option is on two prompts' lists and another on only
// one. This is augmenting-path matching -- an option takes a free place in a
// prompt that offers it, or the place of one that can move somewhere else --
// so a prompt is not filled with options that had other homes while the one
// option only it offers is left out.
func matchPicks(slots []*promptSlot, wanted []rules.Slug) []rules.Slug {
	var place func(rules.Slug, map[*promptSlot]bool) bool
	place = func(option rules.Slug, seen map[*promptSlot]bool) bool {
		for _, slot := range slots {
			if !slot.offers[option] || seen[slot] {
				continue
			}
			seen[slot] = true
			if len(slot.picks) < slot.prompt.Choice.Choose {
				slot.picks = append(slot.picks, option)
				return true
			}
			for i, other := range slot.picks {
				if place(other, seen) {
					slot.picks[i] = option
					return true
				}
			}
		}
		return false
	}
	unplaced := []rules.Slug{}
	for _, option := range wanted {
		if !place(option, map[*promptSlot]bool{}) {
			unplaced = append(unplaced, option)
		}
	}
	return unplaced
}

// promptPaths are the questions a build asks that a sheet answers with a
// number. A model reads the prompt's id and writes it back as a path, so the
// id is accepted as one.
var promptPaths = map[string]string{
	"character/desired-level": "identity.desiredLevel", "character/abilities": "finalAbilities",
	"character/personality-trait": "identity.personalityTraits", "character/ideal": "identity.ideals",
	"character/bond": "identity.bonds", "character/flaw": "identity.flaws", "character/alignment": "identity.alignment",
}

// answer applies one answer through the ordinary choice validator, and returns
// the option keys it settled on.
//
// The prompt is judged against the build alone. A sheet's printed skills are
// often written before its choices are answered, and a prompt computed on top
// of them would call every pick "already held".
func (a *Agent) answer(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) ([]rules.Slug, error) {
	if strings.HasPrefix(args.Prompt, "custom/spell/") {
		return a.knowSpells(ctx, s, cat, args.Picks)
	}
	native := nativeLog(s.Log)
	prompts, err := domain.Prompts(native, cat)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(prompts, func(p domain.Prompt) bool { return promptNamed(cat, p, args.Prompt) })
	if i < 0 {
		return nil, fmt.Errorf("prompt %q is not open; the open prompts are listed in this result", args.Prompt)
	}
	p := prompts[i]
	picks := args.Picks
	if len(picks) == 0 && args.Ref != "" {
		picks = []string{args.Ref}
	}
	// What a character says about itself -- its alignment, its traits, its
	// level -- is a value at a path, and the build reads it from there. Written
	// as a pick the entry is accepted and settles nothing: the question stays
	// open under an answer, which a model then answers again. A model answers
	// these like any other prompt all the same, so the answer is put where it
	// counts.
	if path := promptPaths[p.Choice.Prompt.String()]; path != "" {
		if !strings.HasPrefix(path, "identity.") || len(picks) == 0 {
			return nil, fmt.Errorf("%s is a value, not a pick: write it with plan_import, or import_facts {path: %q, value}", args.Prompt, path)
		}
		// What a source states is transcribed by plan_import and import_facts.
		// An answer here is somebody's decision, and before anybody has been
		// asked anything it can only be the model's own: an alignment guessed
		// for a blank box. It was harmless while it settled nothing; it is
		// not now.
		if !slices.ContainsFunc(s.Events, func(e AgentEvent) bool { return e.Kind == "question" }) {
			return nil, fmt.Errorf("%s was answered before the user was asked anything. If a source states it, write it as stated: import_facts {path: %q, value}. If none does, it is the user's to decide: leave it open and offer it with ask_user", args.Prompt, path)
		}
		fact := agentArgs{Path: path, Value: raw(picks), Source: args.Source}
		if len(picks) == 1 {
			fact.Value = raw(picks[0])
			if level, err := strconv.Atoi(picks[0]); err == nil && path == "identity.desiredLevel" {
				fact.Value = raw(level)
			}
		}
		result, err := a.importFact(ctx, s, cat, fact)
		if err != nil {
			return nil, err
		}
		a.recordProgress(ctx, s, "resolve_import_facts", fact, raw(result))
		keys := make([]rules.Slug, len(picks))
		for i, pick := range picks {
			keys[i] = rules.Slug(pick)
		}
		return keys, nil
	}
	if kind, structural := structuralPrompt(p); structural {
		// "Which subclass?" is settled by naming the entry, not by a pick.
		if len(picks) != 1 {
			return nil, fmt.Errorf("name exactly one %s", kind)
		}
		fact := agentArgs{Kind: kind, Name: picks[0], Source: args.Source}
		result, err := a.importFact(ctx, s, cat, fact)
		if err != nil {
			return nil, err
		}
		a.recordProgress(ctx, s, "resolve_import_facts", fact, raw(result))
		ref, _ := rules.ParseRef(result["ref"].(string))
		return []rules.Slug{ref.Slug}, nil
	}
	// "Choose four" answered with three is not an answer: the build reopens
	// the question and the three are left in the log doing nothing.
	if len(picks) < p.Choice.Choose && !p.UpTo {
		return nil, fmt.Errorf("%s takes exactly %d picks; got %d", args.Prompt, p.Choice.Choose, len(picks))
	}
	choice := cat.ResolveChoice(p.Choice)
	keys := make([]rules.Slug, 0, len(picks))
	for _, pick := range picks {
		key, err := a.resolvePick(ctx, s, cat, choice, pick)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	// A feat the sheet names is first written down as a fact, before the
	// question that grants it is open. Answering that question with it is the
	// same feat finding its place, not a second copy: the fact gives way, or
	// the pick is refused as already held and a model goes off to choose a
	// different feat nobody asked for.
	log := s.Log.Clone()
	stated := func(e domain.Event) bool {
		return e.Observed && e.Type == domain.EventFeat && len(e.Choices) == 0 && slices.Contains(keys, e.Ref.Slug)
	}
	if slices.ContainsFunc(log.Events, stated) {
		if log, err = domain.Rebuild(slices.DeleteFunc(log.Events, stated)); err != nil {
			return nil, err
		}
		native = nativeLog(log)
	}
	events := []domain.Event{{Type: p.Event.Type, Ref: p.Event.Ref, Level: p.Event.Level, Choices: []domain.Answer{{Prompt: p.Choice.Prompt, Picks: keys}}}}
	if err := charuc.ValidateAndAttribute(native, cat, events); err != nil {
		return nil, err
	}
	if err := log.Append(events...); err != nil {
		return nil, err
	}
	if _, err := domain.Project(log, cat); err != nil {
		return nil, err
	}
	s.Log = log
	named := []string{}
	for _, key := range keys {
		named = append(named, key.String())
	}
	a.recordProgress(ctx, s, "answer_choice", agentArgs{Prompt: args.Prompt, Picks: named}, nil)
	return keys, nil
}

// knowSpells puts catalogue spells on the character outside any class's or
// race's count: a sheet that lists five spells for a caster who learns three.
//
// The builder already has a place for exactly this -- its custom-source
// prompts, one for cantrips and one for levelled spells -- so the spell stays
// the catalogue's and is an ordinary answer, not a custom entry. The prompt is
// answered once and then closed, so a later spell extends that answer rather
// than being refused for want of an open question.
func (a *Agent) knowSpells(ctx context.Context, s *AgentSession, cat *catalog.Catalog, picks []string) ([]rules.Slug, error) {
	if len(picks) == 0 {
		return nil, fmt.Errorf("name the spells to add")
	}
	state, err := domain.Project(nativeLog(s.Log), cat)
	if err != nil {
		return nil, err
	}
	added, held := []rules.Slug{}, []string{}
	log := s.Log.Clone()
	for _, pick := range picks {
		found, err := a.resolve(ctx, s, cat, []string{"spell"}, pick, nil, false)
		if err != nil {
			return nil, err
		}
		ref, _ := rules.ParseRef(found.Ref)
		// The custom source is for what nothing else grants. A tiefling's
		// Thaumaturgy is already on the sheet, from the tiefling.
		if agentHolds(state, cat, ref) {
			held = append(held, found.Name)
			continue
		}
		prompt := rules.Slug("custom/spell/known")
		if spell, _ := cat.Spells.Get(ref.Slug); spell.Level == 0 {
			prompt = "custom/spell/cantrip"
		}
		i := slices.IndexFunc(log.Events, func(e domain.Event) bool {
			return !e.Observed && len(e.Choices) == 1 && e.Choices[0].Prompt == prompt
		})
		if i < 0 {
			if err := log.Append(domain.Event{Type: domain.EventChange, Source: domain.GroupClass, Choices: []domain.Answer{{Prompt: prompt}}}); err != nil {
				return nil, err
			}
			i = len(log.Events) - 1
		}
		if !slices.Contains(log.Events[i].Choices[0].Picks, ref.Slug) {
			log.Events[i].Choices[0].Picks = append(slices.Clone(log.Events[i].Choices[0].Picks), ref.Slug)
		}
		added = append(added, ref.Slug)
	}
	// One spell the build already grants is no reason to refuse the rest.
	if len(added) == 0 {
		return nil, fmt.Errorf("%s is already granted by the build; it needs no extra entry", strings.Join(held, ", "))
	}
	if err := charuc.ValidateImported(cat, log); err != nil {
		return nil, err
	}
	s.Log = log
	named := []string{}
	for _, key := range added {
		named = append(named, key.String())
	}
	a.recordProgress(ctx, s, "answer_choice", agentArgs{Picks: named}, nil)
	return added, nil
}

// promptNamed matches a prompt by its id, with or without the pack namespace
// a class slug carries into it.
func promptNamed(cat *catalog.Catalog, p domain.Prompt, id string) bool {
	return p.Choice.Prompt.String() == id || localKey(cat, p.Choice.Prompt.String()) == id
}

// structuralPrompt reports that a prompt asks which catalogue entry the
// character is, and of what kind.
func structuralPrompt(p domain.Prompt) (string, bool) {
	if !p.Event.Ref.IsZero() {
		return "", false
	}
	for kind, typ := range structuralEvents {
		if typ == p.Event.Type {
			return kind.String(), true
		}
	}
	return "", false
}

// resolvePick turns one pick as a model writes it -- an option key, a local
// slug, a printed name -- into the key of one of the prompt's own options.
func (a *Agent) resolvePick(ctx context.Context, s *AgentSession, cat *catalog.Catalog, choice rules.Choice, pick string) (rules.Slug, error) {
	pick = strings.TrimSpace(pick)
	offered := map[rules.Ref]bool{}
	kinds := []string{}
	for _, option := range choice.From.Options {
		key := rules.OptionKey(option)
		if key.String() == pick || strings.EqualFold(localKey(cat, key.String()), pick) {
			return key, nil
		}
		switch o := option.(type) {
		case rules.RefOption:
			offered[o.Ref] = true
			if !slices.Contains(kinds, o.Ref.Kind.String()) {
				kinds = append(kinds, o.Ref.Kind.String())
			}
		case rules.AbilityBonusOption:
			if abilityNames[strings.ToLower(pick)] == o.Ability.String() {
				return key, nil
			}
		}
	}
	keep := func(ref rules.Ref) bool { return offered[ref] }
	if choice.From.Kind != rules.OptionsExplicit {
		kinds, keep = []string{choice.From.Collection.String()}, nil
	}
	if len(kinds) == 0 {
		return "", fmt.Errorf("%q is not an option of %s; use one of its option keys", pick, choice.Prompt)
	}
	found, err := a.resolve(ctx, s, cat, kinds, pick, keep, true)
	if err != nil {
		// "No spell named Bless" sends a model looking for another spelling.
		// When the name is a real entry this prompt just does not offer, say
		// that instead: it is already granted, or not on this list.
		if keep != nil {
			if other, otherErr := a.resolve(ctx, s, cat, kinds, pick, nil, true); otherErr == nil {
				return "", fmt.Errorf("%s is not an option of %s: the build already grants it, or this prompt's list does not have it. Do not retry it here", other.Name, choice.Prompt)
			}
		}
		return "", err
	}
	ref, _ := rules.ParseRef(found.Ref)
	return ref.Slug, nil
}

type agentOption struct {
	Key  string `json:"key"`
	Name string `json:"name,omitempty"`
}

// catalogNames looks up an entry's display name, reading each collection once.
func catalogNames(cat *catalog.Catalog) func(rules.Ref) string {
	kinds := map[rules.RefKind]map[rules.Slug]string{}
	return func(ref rules.Ref) string {
		if kinds[ref.Kind] == nil {
			kinds[ref.Kind] = map[rules.Slug]string{}
			for _, c := range charuc.CatalogCandidates(cat, ref.Kind.String()) {
				entry, _ := rules.ParseRef(c.Ref)
				kinds[ref.Kind][entry.Slug] = c.Name
			}
		}
		return kinds[ref.Kind][ref.Slug]
	}
}

// promptOptions lists what a prompt offers: the key an answer is written with
// and, where the option is a catalogue entry, the name a sheet prints.
func promptOptions(cat *catalog.Catalog, p domain.Prompt, nameOf func(rules.Ref) string) []agentOption {
	choice := cat.ResolveChoice(p.Choice)
	out := []agentOption{}
	if choice.From.Kind != rules.OptionsExplicit {
		for _, c := range charuc.CatalogCandidates(cat, choice.From.Collection.String()) {
			ref, _ := rules.ParseRef(c.Ref)
			out = append(out, agentOption{Key: localKey(cat, ref.Slug.String()), Name: c.Name})
		}
		return out
	}
	for _, option := range choice.From.Options {
		o := agentOption{Key: localKey(cat, rules.OptionKey(option).String())}
		if ref, ok := option.(rules.RefOption); ok {
			o.Name = nameOf(ref.Ref)
		}
		out = append(out, o)
	}
	return out
}

// agentPrompts is the open questions as a model needs them: what is asked,
// how many to pick, how to answer, and the options themselves. A prompt
// without its options costs a round trip per question, which a bounded run
// cannot afford.
func agentPrompts(cat *catalog.Catalog, prompts []domain.Prompt, files bool, decided map[string]agentQuestion) []map[string]any {
	nameOf := catalogNames(cat)
	out := []map[string]any{}
	for _, p := range prompts {
		_, structural := structuralPrompt(p)
		byFact := p.Event.Type == domain.EventChange && p.Choice.Kind != rules.ChooseSpell
		// A sheet lists the gear itself. The questions answered in words --
		// personality, ideals, bonds, flaws, alignment -- are listed like any
		// other: left out, a model never learned they were unanswered and
		// never offered to fill them.
		if files && p.Choice.Kind == rules.ChooseEquipment {
			continue
		}
		entry := map[string]any{"id": localKey(cat, p.Choice.Prompt.String()), "choose": p.Choice.Choose, "kind": p.Choice.Kind.String(), "how": "answer_choices"}
		if structural {
			entry["how"] = "import_facts {kind, name} (a class with its printed level), or answer_choices with the entry's name"
		}
		if byFact {
			entry["how"] = "import_facts {path, value}"
			if path := promptPaths[p.Choice.Prompt.String()]; path != "" {
				entry["how"] = "import_facts {path: \"" + path + "\", value}, or plan_import"
				// Who the character is, as against what it is made of. A sheet
				// that leaves the box blank has not been answered by the
				// model's reading of the rest of it: told only how to write
				// one, a model wrote an alignment "from the description" and
				// the owner was never asked.
				if strings.HasPrefix(path, "identity.") && path != "identity.desiredLevel" {
					entry["how"] = "what a source states: plan_import, or import_facts {path: \"" + path + "\", value}. What no source states is the user's to decide and never inferred: leave it open, offer it with ask_user, and write the reply with answer_choices"
				}
			}
		}
		for key, set := range map[string]bool{"optional": p.Optional, "upTo": p.UpTo, "heldOnly": p.HeldOnly, "repeatable": p.Choice.Repeatable} {
			if set {
				entry[key] = true
			}
		}
		// What the owner has already said about it goes with the prompt, in
		// the list a model reads after every write: that is where it decides
		// whether to ask, and where it used to find only that the prompt was
		// still open.
		if q, ok := decided[entry["id"].(string)]; ok {
			entry["asked"] = map[string]string{"question": q.Text, "answer": q.Answer, "now": "the user has answered: do as they said and never ask this again. Left blank or open means it stays open"}
		}
		if p.Level > 0 {
			entry["level"] = p.Level
		}
		if p.Purpose != "" {
			entry["purpose"] = p.Purpose
		}
		if !p.Source.IsZero() {
			entry["source"] = localKey(cat, p.Source.String())
		}
		if len(p.Held) > 0 {
			entry["held"] = localKeys(cat, p.Held)
		}
		if len(p.Blocked) > 0 {
			entry["blocked"] = localKeys(cat, p.Blocked)
		}
		// The custom-source spell prompts offer every spell there is; a count
		// says as much as the list would.
		if options := promptOptions(cat, p, nameOf); len(options) <= 60 && p.Purpose != "custom" {
			entry["options"] = options
		} else {
			entry["optionCount"] = len(options)
		}
		out = append(out, entry)
	}
	return out
}

func (a *Agent) openPrompts(s *AgentSession, cat *catalog.Catalog) ([]map[string]any, error) {
	prompts, err := domain.Prompts(nativeLog(s.Log), cat)
	if err != nil {
		return nil, err
	}
	// The builder's two "any extra spell" questions are always open and offer
	// every spell there is. Listed, a model tries to close them; extra spells
	// reach them through assign_spells or a custom option's ref instead.
	prompts = slices.DeleteFunc(prompts, func(p domain.Prompt) bool {
		return p.Choice.Kind == rules.ChooseSpell && p.Purpose == "custom"
	})
	return agentPrompts(cat, prompts, len(s.Files) > 0, s.decided()), nil
}

// promptIDs is the ids of a list of open prompts, as a model reads them.
func promptIDs(open []map[string]any) []string {
	out := make([]string, 0, len(open))
	for _, entry := range open {
		if id, ok := entry["id"].(string); ok {
			out = append(out, id)
		}
	}
	return out
}

// questionAbout says which open prompts a question puts to the owner: the
// ones it names, and the ones whose own options it offers as answers. A
// question about a blank alignment offers alignments, whatever it says it is
// about; one that offers suggestions for a trait has to say.
func questionAbout(cat *catalog.Catalog, open []map[string]any, named, offered []string) []string {
	out := []string{}
	for _, entry := range open {
		id, _ := entry["id"].(string)
		about := slices.ContainsFunc(named, func(name string) bool { return localKey(cat, name) == id })
		if options, _ := entry["options"].([]agentOption); !about {
			own := 0
			for _, option := range options {
				if foldedIn(offered, option.Name) || foldedIn(offered, option.Key) {
					own++
				}
			}
			about = own >= 2
		}
		if about {
			out = append(out, id)
		}
	}
	return out
}

// justAnswered reports that the owner's only message since the last question is
// their answer to it.
func justAnswered(events []AgentEvent) bool {
	replies := 0
	for i := len(events) - 1; i >= 0 && events[i].Kind != "question"; i-- {
		if events[i].Kind == "user" {
			replies++
		}
	}
	return replies == 1
}
