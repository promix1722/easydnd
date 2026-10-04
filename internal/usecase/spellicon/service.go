// Package spellicon generates a single pack icon from a raw prompt. It owns
// provider requests and image conversion; callers own input and file output.
package spellicon

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/HugoSmits86/nativewebp"
	"golang.org/x/image/draw"
)

const DefaultModel = "gpt-image-2.5-sunburst"
const DefaultTimeout = 5 * time.Minute
const maxResponseBytes = 32 << 20
const maxAttempts = 5

// Options permits transport injection for tests and model/deadline overrides.
type Options struct {
	Model   string
	Timeout time.Duration
	Client  *http.Client
}

type Service struct {
	key, model string
	timeout    time.Duration
	client     *http.Client
	baseURL    string
	retryDelay time.Duration
}

func New(apiKey string, opts Options) *Service {
	if opts.Model == "" {
		opts.Model = DefaultModel
	}
	if opts.Timeout == 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.Client == nil {
		opts.Client = &http.Client{}
	}
	return &Service{key: apiKey, model: opts.Model, timeout: opts.Timeout, client: opts.Client,
		baseURL: "https://api.openai.com/v1", retryDelay: time.Second}
}

// Generate returns a transparent 128px WebP. No paid request is made for
// missing credentials, empty prompts, or invalid configuration.
func (s *Service) Generate(ctx context.Context, prompt string) ([]byte, error) {
	if strings.TrimSpace(s.key) == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if strings.TrimSpace(s.model) == "" || s.timeout <= 0 {
		return nil, fmt.Errorf("model and positive timeout are required")
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	payload, err := json.Marshal(map[string]string{"model": s.model, "prompt": prompt,
		"size": "1024x1024", "quality": "low", "background": "transparent"})
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		data, retry, delay, err := s.request(ctx, payload)
		if err == nil {
			return convert(ctx, data)
		}
		if !retry || attempt == maxAttempts-1 {
			return nil, err
		}
		if delay <= 0 {
			delay = s.retryDelay * time.Duration(1<<attempt)
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("image generation exhausted retries")
}

func (s *Service) request(ctx context.Context, payload []byte) ([]byte, bool, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/images/generations", bytes.NewReader(payload))
	if err != nil {
		return nil, false, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, false, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, false, 0, err
	}
	if len(data) > maxResponseBytes {
		return nil, false, 0, fmt.Errorf("image response exceeds size limit")
	}
	if resp.StatusCode != http.StatusOK {
		var delay time.Duration
		if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds > 0 {
			// The caller deadline bounds even a very long provider delay.
			if seconds > 86400 {
				seconds = 86400
			}
			delay = time.Duration(seconds) * time.Second
		} else if at, err := http.ParseTime(resp.Header.Get("Retry-After")); err == nil {
			delay = time.Until(at)
		}
		return nil, resp.StatusCode == 429 || resp.StatusCode >= 500, delay, fmt.Errorf("image generation: HTTP %d", resp.StatusCode)
	}
	var result struct {
		Data []struct {
			B64 string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, false, 0, fmt.Errorf("image response: %w", err)
	}
	if len(result.Data) == 0 || result.Data[0].B64 == "" {
		return nil, false, 0, fmt.Errorf("response carried no image")
	}
	pngData, err := base64.StdEncoding.DecodeString(result.Data[0].B64)
	return pngData, false, 0, err
}

func convert(ctx context.Context, data []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode PNG: %w", err)
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 {
		return nil, fmt.Errorf("image dimensions exceed limit")
	}
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode PNG: %w", err)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := nativewebp.Encode(&out, dst, nil); err != nil {
		return nil, fmt.Errorf("encode WebP: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
