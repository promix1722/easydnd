// Command llm is a development-machine tool that calls the OpenAI API. It is
// not part of the deployed service and nothing imports it; it exists so that
// producing entity artwork and draft translations is a re-runnable command
// instead of a copy-paste session with a chat window.
//
// Both subcommands are generic input-to-output: they read a JSON file (or a
// flag) and write plain files, deliberately unaware of where in the repo --
// or outside it -- the result belongs.
//
//	llm images    -prompt "an acid arrow" [-name acid-arrow]   one image
//	llm images    -in prompts.json                             many images
//	llm translate -in en.json -out ru.json -to ru              translated copy
//
// The batch prompts file is a flat object, name to prompt; a name becomes the
// output filename. The spell icons build theirs with
// `node web/scripts/spell-icons.mjs prompts` and run the whole chain through
// `make spell-icons`; a one-off can still be built by hand:
//
//	jq 'map({(.slug): ("icon of " + .slug)}) | add' data/srd_5.1/spells.json
//
// The API key is read from OPENAI_API_KEY. `-dry-run` on either subcommand
// shows what would be requested without needing the key or spending credit.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const apiBase = "https://api.openai.com/v1"

// maxAttempts bounds the retry loop in post. Five retries with doubling
// delays spans about half a minute, enough to ride out a per-minute rate
// limit without hiding a real outage.
const maxAttempts = 5

// client needs an explicit timeout only because the default has none; image
// generation legitimately takes tens of seconds.
var client = &http.Client{Timeout: 5 * time.Minute}

func main() {
	log.SetFlags(0)
	log.SetPrefix("llm: ")

	if len(os.Args) < 2 {
		log.Fatal("usage: llm images|translate [flags]; -h on either lists them")
	}
	var err error
	switch os.Args[1] {
	case "images":
		err = imagesCmd(os.Args[2:])
	case "translate":
		err = translateCmd(os.Args[2:])
	default:
		err = fmt.Errorf("unknown subcommand %q; want images or translate", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}

func apiKey() (string, error) {
	if k := os.Getenv("OPENAI_API_KEY"); k != "" {
		return k, nil
	}
	return "", fmt.Errorf("OPENAI_API_KEY is not set")
}

// post sends one JSON request and decodes the JSON response. 429 and 5xx are
// retried with the server's Retry-After when it gives one, doubling delays
// when it does not; any other failure is returned at once, because retrying
// a rejected prompt only spends more credit on the same rejection.
func post(key, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	for attempt := 1; ; attempt++ {
		req, err := http.NewRequest(http.MethodPost, apiBase+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusOK {
			return json.Unmarshal(data, out)
		}

		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if !retryable || attempt == maxAttempts {
			return fmt.Errorf("%s: %s: %s", path, resp.Status, strings.TrimSpace(string(data)))
		}
		delay := time.Duration(1<<(attempt-1)) * time.Second
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			delay = time.Duration(s) * time.Second
		}
		log.Printf("%s from %s, retrying in %s", resp.Status, path, delay)
		time.Sleep(delay)
	}
}
