package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

func imagesCmd(args []string) error {
	fs := flag.NewFlagSet("llm images", flag.ExitOnError)
	prompt := fs.String("prompt", "", "generate a single image from this prompt")
	in := fs.String("in", "", "batch mode: JSON file, a flat object of name to prompt")
	out := fs.String("out", ".", "directory the images are written into, one <name>.png each")
	name := fs.String("name", "image", "output filename for -prompt mode, without extension")
	model := fs.String("model", "gpt-image-1", "image model")
	size := fs.String("size", "1024x1024", "image size")
	quality := fs.String("quality", "", "image quality (low, medium, high); empty leaves it to the API")
	background := fs.String("background", "", "image background (transparent, opaque); empty leaves it to the API")
	workers := fs.Int("workers", 1, "images generated concurrently; raise on a paid tier, 429s self-throttle via Retry-After")
	dryRun := fs.Bool("dry-run", false, "print the generate/skip decision per name; no network, no key")
	fs.Parse(args)

	if (*prompt == "") == (*in == "") {
		return fmt.Errorf("images: exactly one of -prompt and -in is required")
	}
	prompts := map[string]string{*name: *prompt}
	if *in != "" {
		data, err := os.ReadFile(*in)
		if err != nil {
			return err
		}
		clear(prompts)
		if err := json.Unmarshal(data, &prompts); err != nil {
			return fmt.Errorf("%s: %w (want a flat object of name to prompt)", *in, err)
		}
	}
	for n := range prompts {
		if n == "" || n == ".." || n != filepath.Base(n) {
			return fmt.Errorf("%q cannot name a file in %s", n, *out)
		}
	}

	var key string
	if !*dryRun {
		var err error
		if key, err = apiKey(); err != nil {
			return err
		}
		if err := os.MkdirAll(*out, 0o755); err != nil {
			return err
		}
	}

	body := map[string]any{"model": *model, "size": *size}
	if *quality != "" {
		body["quality"] = *quality
	}
	if *background != "" {
		body["background"] = *background
	}

	// The skip/dry-run pass stays sequential and quiet-fast; only the paid
	// network calls go to the pool. A semaphore rather than worker goroutines
	// per name, so the log keeps one line per image and the counters need one
	// mutex.
	if *workers < 1 {
		*workers = 1
	}
	var generated, skipped, failed int
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, *workers)
	for _, n := range slices.Sorted(maps.Keys(prompts)) {
		dst := filepath.Join(*out, n+".png")
		if _, err := os.Stat(dst); err == nil {
			log.Printf("skip %s, exists", dst)
			skipped++
			continue
		}
		if *dryRun {
			log.Printf("would generate %s", dst)
			generated++
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(n, dst string) {
			defer wg.Done()
			defer func() { <-sem }()
			err := generate(key, body, prompts[n], dst)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				log.Printf("%s: %v", n, err)
				failed++
				return
			}
			log.Printf("wrote %s", dst)
			generated++
		}(n, dst)
	}
	wg.Wait()

	log.Printf("generated %d, skipped %d, failed %d", generated, skipped, failed)
	if failed > 0 {
		return fmt.Errorf("%d images failed; rerunning the same command retries exactly those", failed)
	}
	return nil
}

// generate requests one image and writes it only after a full decode, so an
// interrupted run never leaves a truncated file the next run would skip.
func generate(key string, body map[string]any, prompt, dst string) error {
	req := maps.Clone(body)
	req["prompt"] = prompt
	var resp struct {
		Data []struct {
			B64 string `json:"b64_json"`
		} `json:"data"`
	}
	if err := post(key, "/images/generations", req, &resp); err != nil {
		return err
	}
	if len(resp.Data) == 0 {
		return fmt.Errorf("response carried no image")
	}
	png, err := base64.StdEncoding.DecodeString(resp.Data[0].B64)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, png, 0o644)
}
