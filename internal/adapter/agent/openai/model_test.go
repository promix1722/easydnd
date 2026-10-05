package openai

import (
	"context"
	"encoding/json"
	"fmt"
	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponsesStreamUsesCompleteToolArguments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["store"] != false || body["stream"] != true {
			t.Errorf("unexpected privacy/stream config: %v", body)
		}
		reasoning, _ := body["reasoning"].(map[string]any)
		if body["tool_choice"] != "required" || reasoning["effort"] != "low" || body["prompt_cache_key"] != "session-1" || !strings.Contains(body["instructions"].(string), "Unattended:") {
			t.Errorf("unexpected turn config: %v %v %v", body["tool_choice"], body["reasoning"], body["prompt_cache_key"])
		}
		for _, value := range body["tools"].([]any) {
			declaration := value.(map[string]any)
			if declaration["strict"] != false {
				t.Errorf("optional tool fields can become required: %v", declaration["name"])
			}
		}
		encoded, _ := json.Marshal(body["input"])
		if !strings.Contains(string(encoded), "image_url") || !strings.Contains(string(encoded), "input_file") {
			t.Error("missing multimodal inputs")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Reading\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"response.function_call_arguments.delta\",\"delta\":\"{bad partial\"}\n\n")
		fmt.Fprint(w, `data: {"type":"response.completed","response":{"id":"r1","usage":{"input_tokens":100,"input_tokens_details":{"cached_tokens":80},"output_tokens":7},"output":[{"type":"function_call","call_id":"call1","name":"get_build_context","arguments":"{}"}]}}`+"\n\n")
	}))
	defer server.Close()
	m := &Model{client: sdk.NewClient(option.WithAPIKey("test"), option.WithBaseURL(server.URL), option.WithMaxRetries(0)), model: "configured-model", effort: "low"}
	text := ""
	got, err := m.Respond(context.Background(), charuc.AgentRequest{Locale: "en", Session: "session-1", Unattended: true, Files: []charuc.AgentFile{{Name: "sheet.png", MIME: "image/png", Data: []byte("image")}, {Name: "sheet.pdf", MIME: "application/pdf", Data: []byte("pdf")}}}, func(s string) { text += s })
	if err != nil {
		t.Fatal(err)
	}
	if text != "Reading" || len(got.Calls) != 1 || got.Calls[0].Arguments != "{}" || got.Usage != (charuc.AgentUsage{Input: 100, Cached: 80, Output: 7}) {
		t.Fatalf("bad stream: %+v %q", got, text)
	}
}
