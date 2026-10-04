package openai

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

type pacingTransport func(*http.Request) (*http.Response, error)

func (f pacingTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func imageReply() *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"aW1hZ2U="}]}`))}
}

func pacedGenerator(transport pacingTransport) *Generator {
	return New("test-key", "test-model", WithRequestsPerMinute(20),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithHTTPClient(&http.Client{Transport: transport}))
}

func TestConcurrentImagesShareOneRequestRate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		var starts []time.Time
		g := pacedGenerator(func(_ *http.Request) (*http.Response, error) {
			mu.Lock()
			starts = append(starts, time.Now())
			mu.Unlock()
			return imageReply(), nil
		})
		var wg sync.WaitGroup
		for range 10 {
			wg.Go(func() {
				if _, err := g.Generate(context.Background(), "rune"); err != nil {
					t.Error(err)
				}
			})
		}
		wg.Wait()
		if len(starts) != 10 {
			t.Fatalf("requests = %d, want 10", len(starts))
		}
		for i := 1; i < len(starts); i++ {
			if gap := starts[i].Sub(starts[i-1]); gap < 3*time.Second {
				t.Fatalf("requests %d and %d exceeded the shared 20/min limit: %s", i-1, i, gap)
			}
		}
	})
}

func TestRetriesConsumeTheSameRateLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var starts []time.Time
		g := pacedGenerator(func(_ *http.Request) (*http.Response, error) {
			starts = append(starts, time.Now())
			if len(starts) == 1 {
				return &http.Response{StatusCode: http.StatusTooManyRequests, Status: "429 Too Many Requests", Header: http.Header{"Retry-After": []string{"1"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`))}, nil
			}
			return imageReply(), nil
		})
		if _, err := g.Generate(context.Background(), "rune"); err != nil {
			t.Fatal(err)
		}
		if len(starts) != 2 {
			t.Fatalf("requests = %d, want 2", len(starts))
		}
		if gap := starts[1].Sub(starts[0]); gap < 3*time.Second {
			t.Fatalf("retry bypassed the shared rate limit: %s", gap)
		}
	})
}

func TestCancelledRateWaitDoesNotSendARequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		g := pacedGenerator(func(_ *http.Request) (*http.Response, error) {
			calls++
			return imageReply(), nil
		})
		if _, err := g.Generate(context.Background(), "first"); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		finished := make(chan error, 1)
		go func() { _, err := g.Generate(ctx, "cancelled"); finished <- err }()
		synctest.Wait()
		cancel()
		if err := <-finished; !errors.Is(err, context.Canceled) {
			t.Fatalf("waiting generation = %v, want context cancellation", err)
		}
		if calls != 1 {
			t.Fatalf("cancelled wait sent an extra request: %d", calls)
		}
	})
}
