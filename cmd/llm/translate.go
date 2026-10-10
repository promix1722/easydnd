package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// One request translates at most this many string leaves and this much source
// text. Both are far under any model's context; the caps exist so one bad
// response loses little and a partial rerun stays cheap.
const (
	maxLeavesPerRequest = 40
	maxCharsPerRequest  = 6000
)

const systemPrompt = `Translate every value of the user's JSON object into the language with IETF tag %q.
Reply with a JSON object carrying exactly the same keys and only the translated values.
Never translate, alter or drop a key. Preserve {{placeholder}} tokens byte for byte.
A key ending in _one, _few, _many or _other names that plural form; translate the value accordingly.
Preserve URLs, Markdown, numbers and game formulas. For Russian D&D prose use established 5e terminology,
write dice in Latin notation as 8d6 rather than 8к6, keep imperial measurements imperial with Russian unit
words -- футов, фунтов, миль -- and never convert them to metres or kilograms, and do not introduce proper
names absent from the source.
Use this glossary as authoritative terminology, inflecting Russian words to fit their sentence: %s`

func translateCmd(args []string) error {
	fs := flag.NewFlagSet("llm translate", flag.ExitOnError)
	in := fs.String("in", "", "JSON file to translate")
	out := fs.String("out", "", "where the translated copy is written")
	to := fs.String("to", "", "target language tag, e.g. ru")
	// Pinned to a dated snapshot, not a floating alias: ru.sources.json records
	// what produced a translation, and an alias makes that record meaningless
	// as soon as it moves.
	model := fs.String("model", "gpt-5.4-2026-03-05", "text model")
	// "Thinking" is a ChatGPT UI mode, not a model: in the API it is this
	// parameter. Blank sends none, which is the pre-5.x behaviour.
	reasoning := fs.String("reasoning", "", "reasoning effort: minimal, low, medium or high; blank sends none")
	existing := fs.String("existing", "", "partial translation whose populated leaves are preserved")
	// Without this, -existing pointing at -out preserves everything already
	// there, so a rerun meant to redo the prose translates nothing at all and
	// exits successfully. Naming the leaves to keep turns that trap into the
	// way you ask for a reroll: `-preserve name` keeps the hand-checked names
	// and re-requests every description.
	preserve := fs.String("preserve", "", "comma-separated leaf names to keep from -existing; blank keeps every populated leaf")
	glossaryPath := fs.String("glossary", "", "flat JSON object of source terms to preferred translations")
	dryRun := fs.Bool("dry-run", false, "print leaf and request counts; no network, no key")
	// ExitOnError: Parse exits on a bad flag and has no error to return.
	_ = fs.Parse(args)

	if *in == "" || *out == "" || *to == "" {
		return fmt.Errorf("translate: -in, -out and -to are all required")
	}

	data, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("%s: %w", *in, err)
	}

	leaves := map[string]string{}
	collect(doc, "", leaves)

	translated := map[string]string{}
	if *existing != "" {
		var keep []string
		if *preserve != "" {
			keep = strings.Split(*preserve, ",")
		}
		if err := loadMatchingLeaves(*existing, keep, leaves, translated); err != nil {
			return err
		}
	}
	checkpoint := *out + ".checkpoint.json"
	if err := loadCheckpoint(checkpoint, leaves, translated); err != nil {
		return err
	}

	pending := make(map[string]string, len(leaves)-len(translated))
	for path, source := range leaves {
		if _, ok := translated[path]; !ok {
			pending[path] = source
		}
	}
	batches := chunk(pending, maxLeavesPerRequest, maxCharsPerRequest)

	if *dryRun {
		chars := 0
		for _, s := range pending {
			chars += len(s)
		}
		log.Printf("%s: %d/%d leaves already translated; %d chars in %d requests", *in, len(translated), len(leaves), chars, len(batches))
		return nil
	}
	glossary, err := loadGlossary(*glossaryPath)
	if err != nil {
		return err
	}
	key, err := apiKey()
	if err != nil {
		return err
	}

	kept := 0
	for i, batch := range batches {
		payload := map[string]string{}
		for _, p := range batch {
			payload[p] = leaves[p]
		}
		got, err := translateChunk(key, *model, *reasoning, *to, glossary, payload)
		if err != nil {
			log.Printf("request %d/%d: %v", i+1, len(batches), err)
			kept += len(batch)
			continue
		}
		for _, p := range batch {
			switch t, ok := got[p]; {
			case !ok:
				log.Printf("%s: missing from the response, kept the source text", p)
				kept++
			case !valuesMatch(leaves[p], t):
				// Both token lists, because the leaf that keeps failing is the
				// one someone has to read, and "altered" alone does not say how.
				log.Printf("%s: numbers or placeholders altered, kept the source text\n  source %v %v\n  got    %v %v",
					p,
					placeholderRE.FindAllString(leaves[p], -1), numberTokens(leaves[p]),
					placeholderRE.FindAllString(t, -1), numberTokens(t))
				kept++
			default:
				translated[p] = t
			}
		}
		if err := writeJSONAtomic(checkpoint, translated); err != nil {
			return err
		}
		log.Printf("request %d/%d done", i+1, len(batches))
	}
	if kept > 0 {
		return fmt.Errorf("%d leaves were not translated; progress is saved in %s", kept, checkpoint)
	}

	doc = splice(doc, "", translated)
	if err := writeJSONAtomic(*out, doc); err != nil {
		return err
	}
	if err := os.Remove(checkpoint); err != nil && !os.IsNotExist(err) {
		return err
	}
	// ponytail: encoding/json sorts object keys, so the output loses any
	// hand-picked key order; both known payloads are machine-consumed.
	// Preserve order with a token-level rewriter if anyone ever cares.
	log.Printf("wrote %s: %d of %d leaves translated to %s", *out, len(translated), len(leaves), *to)
	return nil
}

func translateChunk(key, model, reasoning, locale, glossary string, leaves map[string]string) (map[string]string, error) {
	body, err := json.Marshal(leaves)
	if err != nil {
		return nil, err
	}
	req := map[string]any{
		"model":           model,
		"response_format": map[string]string{"type": "json_object"},
		"messages": []map[string]string{
			{"role": "system", "content": fmt.Sprintf(systemPrompt, locale, glossary)},
			{"role": "user", "content": string(body)},
		},
	}
	// Omitted rather than sent empty: the models that predate the parameter
	// reject it outright, and this tool still has to run against them.
	if reasoning != "" {
		req["reasoning_effort"] = reasoning
	}
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := post(key, "/chat/completions", req, &resp); err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("response carried no choices")
	}
	out := map[string]string{}
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &out); err != nil {
		return nil, fmt.Errorf("response was not a flat JSON object of strings: %w", err)
	}
	if len(out) != len(leaves) {
		return nil, fmt.Errorf("response carried %d keys, want %d", len(out), len(leaves))
	}
	for path := range out {
		if _, ok := leaves[path]; !ok {
			return nil, fmt.Errorf("response added unknown key %q", path)
		}
	}
	return out, nil
}

// loadMatchingLeaves copies leaves of an existing translation into out, keeping
// only paths the source still has -- so a slug or a paragraph the English has
// since dropped cannot be spliced back in.
//
// keep narrows that further to leaves whose last path segment is one of the
// named ones, which is how a reroll asks to redo the prose but not the names.
// An empty keep preserves every populated leaf, the original behaviour.
func loadMatchingLeaves(path string, keep []string, source, out map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	got := map[string]string{}
	collect(doc, "", got)
	for p, value := range got {
		if _, ok := source[p]; !ok {
			continue
		}
		if len(keep) > 0 && !slices.Contains(keep, leafName(p)) {
			continue
		}
		out[p] = value
	}
	return nil
}

// leafName returns the path's last segment: "name" for /fireball/name, and the
// array index for a paragraph of /fireball/desc.
func leafName(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}

func loadCheckpoint(path string, source, out map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var got map[string]string
	if err := json.Unmarshal(data, &got); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for p, value := range got {
		original, ok := source[p]
		if ok && valuesMatch(original, value) {
			out[p] = value
		}
	}
	return nil
}

func loadGlossary(path string) (string, error) {
	if path == "" {
		return "{}", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var glossary map[string]string
	if err := json.Unmarshal(data, &glossary); err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	body, err := json.Marshal(glossary)
	return string(body), err
}

func writeJSONAtomic(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".translate-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o644); err != nil {
		return err
	}
	if _, err := tmp.Write(append(body, '\n')); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	ok = true
	return nil
}

// collect records every non-empty string leaf under v into out, keyed by its
// JSON-pointer path. Object keys are never collected, so nothing can ever
// translate one; they stay visible inside the paths, which is what lets the
// model see an i18next plural suffix.
func collect(v any, path string, out map[string]string) {
	switch v := v.(type) {
	case map[string]any:
		for k, child := range v {
			collect(child, path+"/"+escapeKey(k), out)
		}
	case []any:
		for i, child := range v {
			collect(child, path+"/"+strconv.Itoa(i), out)
		}
	case string:
		if v != "" {
			out[path] = v
		}
	}
}

// splice is collect's inverse: it returns v with every string leaf whose path
// has an entry in tr replaced by that entry, everything else untouched.
func splice(v any, path string, tr map[string]string) any {
	switch v := v.(type) {
	case map[string]any:
		for k, child := range v {
			v[k] = splice(child, path+"/"+escapeKey(k), tr)
		}
	case []any:
		for i, child := range v {
			v[i] = splice(child, path+"/"+strconv.Itoa(i), tr)
		}
	case string:
		if t, ok := tr[path]; ok {
			return t
		}
	}
	return v
}

// escapeKey applies JSON-pointer escaping (RFC 6901), which keeps a path
// unambiguous when a key itself contains a separator.
func escapeKey(k string) string {
	return strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
}

// chunk cuts the sorted leaf paths into groups no larger than maxLeaves
// entries or maxChars of source text, whichever bites first -- but never cuts
// through the middle of one top-level entry.
//
// The boundary matters because the paths sort `blocks < desc < fields < name`,
// so cutting anywhere inside a spell could hand the model its "At Higher
// Levels" note in one conversation and the description that note refers to in
// another, with the material component somewhere else again. Whole entries
// keep a spell's prose in front of the model that is translating it.
//
// Both caps are therefore soft: an entry bigger than either goes in a request
// of its own rather than being split. They are still caps and not a target --
// several whole entries are packed into each request, because the system
// prompt carries the inlined glossary and sending one entry per request would
// resend that glossary once per spell.
//
// Sorting is what makes an entry contiguous: every leaf of `acid-arrow` shares
// the prefix `/acid-arrow/`, and `/` sorts below every character a slug can
// continue with, so no other entry can interleave.
func chunk(leaves map[string]string, maxLeaves, maxChars int) [][]string {
	paths := slices.Sorted(maps.Keys(leaves))
	var out [][]string
	var cur []string
	chars := 0
	for i := 0; i < len(paths); {
		// Take the whole of the next entry, however big it turns out to be.
		j, size := i, 0
		for j < len(paths) && entryOf(paths[j]) == entryOf(paths[i]) {
			size += len(leaves[paths[j]])
			j++
		}
		if len(cur) > 0 && (len(cur)+(j-i) > maxLeaves || chars+size > maxChars) {
			out = append(out, cur)
			cur, chars = nil, 0
		}
		cur = append(cur, paths[i:j]...)
		chars += size
		i = j
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

// entryOf returns the path's first segment: the slug of the entry a leaf
// belongs to. The flat i18next payload has no second segment, so there every
// key is its own entry and chunking falls back to packing by the caps alone.
func entryOf(path string) string {
	rest := strings.TrimPrefix(path, "/")
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return rest[:i]
	}
	return rest
}

var (
	placeholderRE = regexp.MustCompile(`\{\{[^}]*\}\}`)
	// Dice before bare numbers so that "8d6" is one token rather than an 8 and
	// a 6, and `\d*d` so that the SRD's bare "d4" is a die too. Without that,
	// a translation writing "1d4" where the source wrote "d4" would read as a
	// number appearing from nowhere.
	numberRE = regexp.MustCompile(`\d*d\d+|\d+`)
	// Thousands separators, which the two languages write differently:
	// English "1,500 gp" against Russian "1 500 зм". Both had to be handled --
	// the comma alone still read the Russian as a 1 and a 500.
	//
	// Exactly three digits then a non-digit, which keeps this off the Russian
	// decimal comma ("2,5 см" stays two tokens). Matched rather than asserted
	// because RE2 has no lookahead, and put back by the replacement; a word
	// boundary could not do it -- the SRD writes "5,000gp." with no space.
	thousandsRE = regexp.MustCompile(`(\d)[,\x{00A0}\x{202F} ](\d\d\d)([^\d]|$)`)
)

// numberTokens is the numbers and dice in s, thousands separators removed so
// that the same quantity written either way compares equal.
func numberTokens(s string) []string {
	for thousandsRE.MatchString(s) {
		s = thousandsRE.ReplaceAllString(s, "${1}${2}${3}")
	}
	return numberRE.FindAllString(s, -1)
}

// valuesMatch reports whether translation carries exactly the source's
// {{placeholder}} tokens and exactly its numbers and dice, repeat counts
// included.
//
// Placeholders matter because a mangled one renders raw braces to the user.
// Numbers matter more, and are the reason this is not just a placeholder
// check: this corpus is rules text, where "8d6" coming back as "6d8" or "120
// feet" as "12" is a wrong rule rather than a clumsy sentence, and every gate
// downstream would wave it through. Being strict is deliberate -- a false
// positive costs one re-requested leaf, a false negative ships a bug in the
// rules.
func valuesMatch(source, translation string) bool {
	return tokensMatch(placeholderRE, source, translation) &&
		numbersMatch(source, translation)
}

// numbersMatch compares the *set* of numbers and dice, not their repeat counts.
//
// Counting repeats was the obvious rule and it was wrong, because English
// states an area as "5 feet square" and Russian states it as "5 на 5 футов".
// The idiom duplicates the number by construction, so a strict count rejected
// correct translations of every "N-foot-square" in the SRD -- four of them on
// the first full run.
//
// What survives is what the check was for: a number that changes (120 -> 12),
// a die that transposes (8d6 -> 6d8) and a number that disappears are all still
// caught, because each changes the set. What is given up is a dropped repeat --
// "2d6 then 2d6" coming back with one of them -- which is the narrower risk and
// the price of not crying wolf on idiomatic Russian.
func numbersMatch(source, translation string) bool {
	a := slices.Compact(slices.Sorted(slices.Values(numberTokens(source))))
	b := slices.Compact(slices.Sorted(slices.Values(numberTokens(translation))))
	return slices.Equal(a, b)
}

func tokensMatch(re *regexp.Regexp, source, translation string) bool {
	a := re.FindAllString(source, -1)
	b := re.FindAllString(translation, -1)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
