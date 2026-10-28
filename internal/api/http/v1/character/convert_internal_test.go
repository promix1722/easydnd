package character

import (
	"encoding/json"
	"strings"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
)

// Removing every spell of a slot saves an empty answer; sent as null, it
// crashed the builder on the next load.
func TestEmptyAnswerIsAnEmptyList(t *testing.T) {
	body, err := json.Marshal(eventOf(domain.Event{Type: domain.EventLevel, Choices: []domain.Answer{{Prompt: "warlock/spell/known/1"}}}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"picks":[]`) {
		t.Fatalf("empty picks not a list: %s", body)
	}
}
