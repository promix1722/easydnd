package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var testPNG = base64.StdEncoding.EncodeToString([]byte("\x89PNG fake bytes"))

func newTestGenerator(t *testing.T, handler http.HandlerFunc, opts ...Option) (*Generator, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	opts = append([]Option{
		WithBaseURL(srv.URL),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	}, opts...)
	g := New("test-key", "gpt-image-2.5-sunburst", opts...)
	g.baseDelay = time.Millisecond // tests must not wait out real backoff
	return g, &calls
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": []map[string]any{{"b64_json": testPNG}},
	})
}

func TestGeneratePostsImageRequest(t *testing.T) {
	var gotAuth, gotBody string
	g, calls := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		if r.URL.Path != "/images/generations" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		okHandler(w, r)
	})

	png, err := g.Generate(context.Background(), "a glowing rune")
	if err != nil {
		t.Fatal(err)
	}
	if string(png) != "\x89PNG fake bytes" {
		t.Fatalf("unexpected image bytes %q", png)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("auth header wrong: %q", gotAuth)
	}
	// Service defaults are the artwork contract: 1024, low, transparent.
	for _, want := range []string{`"model":"gpt-image-2.5-sunburst"`, `"size":"1024x1024"`, `"quality":"low"`, `"background":"transparent"`, `"prompt":"a glowing rune"`} {
		if !strings.Contains(gotBody, want) {
			t.Fatalf("request body missing %s: %s", want, gotBody)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("want 1 request, got %d", calls.Load())
	}
}

func TestGenerateRetries429WithRetryAfter(t *testing.T) {
	var first atomic.Bool
	g, calls := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
		if first.CompareAndSwap(false, true) {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		okHandler(w, r)
	})
	if _, err := g.Generate(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("429 should have been retried, calls=%d", calls.Load())
	}
}

func TestGenerateRetries5xx(t *testing.T) {
	var count atomic.Int32
	g, _ := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
		if count.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		okHandler(w, r)
	})
	if _, err := g.Generate(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 3 {
		t.Fatalf("want 3 attempts, got %d", count.Load())
	}
}

func TestGenerateDoesNotRetry4xx(t *testing.T) {
	g, calls := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "prompt rejected", "type": "moderation"},
		})
	})
	_, err := g.Generate(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Fatalf("4xx must not be retried, calls=%d", calls.Load())
	}
	// The API's own error field travels; the raw body does not.
	if !strings.Contains(err.Error(), "prompt rejected") {
		t.Fatalf("error should carry the API message, got %v", err)
	}
	var serr *statusError
	if !errors.As(err, &serr) {
		t.Fatalf("error should be a statusError, got %T", err)
	}
}

func TestGenerateRespectsContext(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	g, _ := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release // hang until the test lets go
		okHandler(w, r)
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := g.Generate(ctx, "x")
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Generate did not return after cancellation")
	}
	close(release)
}

func TestGenerateRejectsEmptyResponse(t *testing.T) {
	g, _ := newTestGenerator(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	})
	if _, err := g.Generate(context.Background(), "x"); err == nil {
		t.Fatal("an imageless response must fail")
	}
}

func TestGenerateFailsAfterMaxAttempts(t *testing.T) {
	g, calls := newTestGenerator(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	start := time.Now()
	if _, err := g.Generate(context.Background(), "x"); err == nil {
		t.Fatal("persistent 503 must fail")
	}
	if calls.Load() != maxAttempts {
		t.Fatalf("want %d attempts, got %d", maxAttempts, calls.Load())
	}
	// Doubling delays across 4 gaps span ~15s worst case; cancellation or a
	// runaway loop would show up as far more. This bound just proves the
	// loop terminated at all inside the test timeout.
	if time.Since(start) > time.Minute {
		t.Fatal("retry loop did not bound itself")
	}
}
