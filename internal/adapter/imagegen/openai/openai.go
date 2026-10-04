// Package openai adapts OpenAI's images API to the spellicon.Generator port.
//
// It is the same request the `llm images` dev tool has always sent -- the CLI
// now builds on this package rather than carrying its own copy -- with the
// retry policy that pattern established: 429 and 5xx are retried with the
// server's Retry-After when it gives one and doubling delays when it does
// not, and any other failure is returned at once, because retrying a rejected
// prompt only spends more credit on the same rejection.
package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

const (
	// defaultBase is the API root; an option overrides it only so tests can
	// stand in a httptest server, not for configuration.
	defaultBase = "https://api.openai.com/v1"

	// Transient attempts stay bounded; pacing and backoff also consume the
	// caller's deadline.
	maxAttempts = 5
)

// ImageOptions are the per-request fields `llm images` exposes as flags. An
// empty field is omitted from the request and the API's default applies --
// the same semantics the flags have always had.
type ImageOptions struct {
	Size       string
	Quality    string
	Background string
}

// Generator generates PNG images through the images/generations endpoint.
type Generator struct {
	key       string
	model     string
	base      string
	client    *http.Client
	options   ImageOptions
	log       *slog.Logger
	baseDelay time.Duration
	limiter   *rate.Limiter
}

// Option adjusts provider transport, image settings, and shared request pacing.
type Option func(*Generator)

// WithBaseURL points the generator at another API root. Tests only: a
// configured base is a production footgun, so it is not a YAML setting.
func WithBaseURL(base string) Option {
	return func(g *Generator) { g.base = base }
}

// WithHTTPClient swaps the HTTP client, e.g. for a test transport.
func WithHTTPClient(client *http.Client) Option {
	return func(g *Generator) {
		if client != nil {
			g.client = client
		}
	}
}

// WithImageOptions replaces the whole option set the service defaults ship
// with -- the CLI uses it to pass exactly its flags, where an empty value
// means "leave it to the API".
func WithImageOptions(options ImageOptions) Option {
	return func(g *Generator) { g.options = options }
}

// WithLogger sets where retry notices go; the default is the process logger.
func WithLogger(log *slog.Logger) Option {
	return func(g *Generator) {
		if log != nil {
			g.log = log
		}
	}
}

// WithRequestsPerMinute spaces all image attempts across this generator,
// including retries. A single-token burst prevents ten workers sending at once.
func WithRequestsPerMinute(requestsPerMinute int) Option {
	return func(g *Generator) {
		if requestsPerMinute > 0 {
			g.limiter = rate.NewLimiter(rate.Limit(requestsPerMinute)/60, 1)
		}
	}
}

// New builds the service default: 1024px, low quality, transparent
// background -- the artwork contract the Makefile's spell-icons run and the
// dev queue share. Callers needing the generic flag semantics pass
// WithImageOptions.
func New(apiKey, model string, opts ...Option) *Generator {
	g := &Generator{
		key:       apiKey,
		model:     model,
		base:      defaultBase,
		client:    &http.Client{Timeout: 5 * time.Minute},
		options:   ImageOptions{Size: "1024x1024", Quality: "low", Background: "transparent"},
		log:       slog.Default(),
		baseDelay: time.Second,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Generate draws one image and returns its PNG bytes.
func (g *Generator) Generate(ctx context.Context, prompt string) ([]byte, error) {
	body := map[string]any{"model": g.model, "prompt": prompt}
	if g.options.Size != "" {
		body["size"] = g.options.Size
	}
	if g.options.Quality != "" {
		body["quality"] = g.options.Quality
	}
	if g.options.Background != "" {
		body["background"] = g.options.Background
	}

	var resp struct {
		Data []struct {
			B64 string `json:"b64_json"`
		} `json:"data"`
	}
	if err := g.post(ctx, "/images/generations", body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 || resp.Data[0].B64 == "" {
		return nil, errors.New("response carried no image")
	}
	png, err := base64.StdEncoding.DecodeString(resp.Data[0].B64)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return png, nil
}

// statusError is a non-2xx response. The message is the API's own error
// field, never the raw body -- upstream echoes can carry request text the
// caller should not be handed wholesale.
type statusError struct {
	status  string
	message string
	retry   time.Duration // Retry-After, if the server sent one
}

func (e *statusError) Error() string {
	if e.message != "" {
		return e.status + ": " + e.message
	}
	return e.status
}

// post sends one JSON request and decodes the JSON response.
func (g *Generator) post(ctx context.Context, path string, in, out any) error {
	payload, err := json.Marshal(in)
	if err != nil {
		return err
	}
	for attempt := 1; ; attempt++ {
		retryable, delay, err := g.roundTrip(ctx, path, payload, out)
		if err == nil {
			return nil
		}
		if !retryable || attempt == maxAttempts {
			return err
		}
		if delay <= 0 {
			delay = g.baseDelay << (attempt - 1)
		}
		g.log.Info("image request retrying", "path", path, "attempt", attempt, "delay", delay, "error", err)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// roundTrip performs one attempt. retryable reports whether another attempt
// is worth paying for, and delay carries the server's Retry-After.
func (g *Generator) roundTrip(ctx context.Context, path string, payload []byte, out any) (retryable bool, delay time.Duration, err error) {
	if g.limiter != nil {
		if err := g.limiter.Wait(ctx); err != nil {
			return false, 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.base+path, bytes.NewReader(payload))
	if err != nil {
		return false, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+g.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return false, 0, err
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return false, 0, err
	}
	if resp.StatusCode == http.StatusOK {
		return false, 0, json.Unmarshal(data, out)
	}

	serr := &statusError{status: resp.Status}
	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &env) == nil {
		serr.message = env.Error.Message
	}
	if s, perr := strconv.Atoi(resp.Header.Get("Retry-After")); perr == nil && s > 0 {
		serr.retry = time.Duration(s) * time.Second
	}
	retryable = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
	return retryable, serr.retry, serr
}
