package spellicon

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/image/webp"
)

func pngFixture(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for y := 64; y < 192; y++ {
		for x := 64; x < 192; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(b.Bytes())
}

func TestGenerateSettingsAndTransparentWebP(t *testing.T) {
	for _, model := range []string{"", "override-model"} {
		t.Run(model, func(t *testing.T) {
			encoded := pngFixture(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/images/generations" || r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer secret" {
					t.Error("wrong request")
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				expected := model
				if expected == "" {
					expected = DefaultModel
				}
				if body["model"] != expected || body["prompt"] != "a glowing rune" || body["background"] != "transparent" || body["size"] != "1024x1024" || body["quality"] != "low" {
					t.Errorf("settings: %v", body)
				}
				fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, encoded)
			}))
			defer server.Close()
			service := New("secret", Options{Model: model})
			service.baseURL = server.URL
			data, err := service.Generate(context.Background(), "a glowing rune")
			if err != nil {
				t.Fatal(err)
			}
			img, err := webp.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Dx() != 128 || img.Bounds().Dy() != 128 {
				t.Fatal(img.Bounds())
			}
			_, _, _, alpha := img.At(0, 0).RGBA()
			if alpha != 0 {
				t.Fatal("lost transparency")
			}
			_, _, _, alpha = img.At(64, 64).RGBA()
			if alpha != 65535 {
				t.Fatal("lost opaque subject")
			}
		})
	}
}

func TestFailuresAndRetryLimit(t *testing.T) {
	for _, tc := range []struct {
		name, body        string
		status, wantCalls int
	}{
		{"rate limited", "", 429, 5}, {"server failure", "", 503, 5}, {"rejected", "", 400, 1},
		{"empty", `{"data":[]}`, 200, 1}, {"bad JSON", "{", 200, 1},
		{"bad base64", `{"data":[{"b64_json":"!"}]}`, 200, 1},
		{"bad PNG", `{"data":[{"b64_json":"eA=="}]}`, 200, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			s := New("secret", Options{})
			s.baseURL = server.URL
			s.retryDelay = time.Millisecond
			if _, err := s.Generate(context.Background(), "rune"); err == nil {
				t.Fatal("expected error")
			}
			if int(calls.Load()) != tc.wantCalls {
				t.Fatalf("calls=%d", calls.Load())
			}
		})
	}
}

func TestRetryAfterRespectsCancellation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(429)
	}))
	defer server.Close()
	s := New("secret", Options{Timeout: 30 * time.Millisecond})
	s.baseURL = server.URL
	_, err := s.Generate(context.Background(), "rune")
	if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
		t.Fatalf("calls=%d error=%v", calls.Load(), err)
	}
}

func TestInvalidInputMakesNoRequest(t *testing.T) {
	for _, tc := range []struct {
		key, prompt string
		opts        Options
	}{{"", "rune", Options{}}, {"secret", " ", Options{}}, {"secret", "rune", Options{Timeout: -1}}, {"secret", "rune", Options{Model: " "}}} {
		s := New(tc.key, tc.opts)
		s.baseURL = ":"
		if _, err := s.Generate(context.Background(), tc.prompt); err == nil {
			t.Fatal("accepted invalid input")
		}
	}
}

func TestTransientFailureThenSuccess(t *testing.T) {
	encoded := pngFixture(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, encoded)
	}))
	defer server.Close()
	s := New("secret", Options{})
	s.baseURL = server.URL
	s.retryDelay = time.Millisecond
	data, err := s.Generate(context.Background(), "rune")
	if err != nil || calls.Load() != 2 {
		t.Fatalf("calls=%d error=%v", calls.Load(), err)
	}
	if _, err = webp.Decode(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}
