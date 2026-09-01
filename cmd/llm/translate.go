package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
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
A key ending in _one, _few, _many or _other names that plural form; translate the value accordingly.`

func translateCmd(args []string) error {
	fs := flag.NewFlagSet("llm translate", flag.ExitOnError)
	in := fs.String("in", "", "JSON file to translate")
	out := fs.String("out", "", "where the translated copy is written")
	to := fs.String("to", "", "target language tag, e.g. ru")
	model := fs.String("model", "gpt-4o-mini", "text model")
	dryRun := fs.Bool("dry-run", false, "print leaf and request counts; no network, no key")
	fs.Parse(args)

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
	batches := chunk(leaves, maxLeavesPerRequest, maxCharsPerRequest)

	if *dryRun {
		chars := 0
		for _, s := range leaves {
			chars += len(s)
		}
		log.Printf("%s: %d string leaves, %d chars, %d requests", *in, len(leaves), chars, len(batches))
		return nil
	}
	key, err := apiKey()
	if err != nil {
		return err
	}

	translated := map[string]string{}
	kept := 0
	for i, batch := range batches {
		payload := map[string]string{}
		for _, p := range batch {
			payload[p] = leaves[p]
		}
		got, err := translateChunk(key, *model, *to, payload)
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
			case !placeholdersMatch(leaves[p], t):
				log.Printf("%s: placeholders altered, kept the source text", p)
				kept++
			default:
				translated[p] = t
			}
		}
		log.Printf("request %d/%d done", i+1, len(batches))
	}

	doc = splice(doc, "", translated)
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, append(body, '\n'), 0o644); err != nil {
		return err
	}
	// ponytail: encoding/json sorts object keys, so the output loses any
	// hand-picked key order; both known payloads are machine-consumed.
	// Preserve order with a token-level rewriter if anyone ever cares.
	log.Printf("wrote %s: %d of %d leaves translated to %s", *out, len(translated), len(leaves), *to)
	if kept > 0 {
		return fmt.Errorf("%d leaves kept their source text; their paths are logged above, rerun on a cut-down file or fix by hand", kept)
	}
	return nil
}

func translateChunk(key, model, locale string, leaves map[string]string) (map[string]string, error) {
	body, err := json.Marshal(leaves)
	if err != nil {
		return nil, err
	}
	req := map[string]any{
		"model":           model,
		"response_format": map[string]string{"type": "json_object"},
		"messages": []map[string]string{
			{"role": "system", "content": fmt.Sprintf(systemPrompt, locale)},
			{"role": "user", "content": string(body)},
		},
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
	return out, nil
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
// entries or maxChars of source text, whichever bites first. Sorting keeps
// related keys in the same request, so the model translates a collection's
// entries with its neighbours in view.
func chunk(leaves map[string]string, maxLeaves, maxChars int) [][]string {
	var out [][]string
	var cur []string
	chars := 0
	for _, p := range slices.Sorted(maps.Keys(leaves)) {
		if len(cur) > 0 && (len(cur) == maxLeaves || chars+len(leaves[p]) > maxChars) {
			out = append(out, cur)
			cur, chars = nil, 0
		}
		cur = append(cur, p)
		chars += len(leaves[p])
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

var placeholderRE = regexp.MustCompile(`\{\{[^}]*\}\}`)

// placeholdersMatch reports whether translation carries exactly the source's
// {{placeholder}} tokens, repeat counts included. A translation that loses or
// mangles one would render the raw braces to the user.
func placeholdersMatch(source, translation string) bool {
	a := placeholderRE.FindAllString(source, -1)
	b := placeholderRE.FindAllString(translation, -1)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
