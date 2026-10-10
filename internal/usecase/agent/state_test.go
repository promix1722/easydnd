package agent

import (
	"testing"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

func TestStatusMovesOnlyAlongTransitions(t *testing.T) {
	s := &AgentSession{Status: "queued"}
	for _, to := range []string{"running", "waiting", "queued", "running", "review", "review", "paused", "queued"} {
		if err := setStatus(s, to); err != nil {
			t.Fatal(err)
		}
	}
	// Only a claimed turn asks, reviews or fails, and nothing resumes itself.
	for from, to := range map[string]string{"queued": "waiting", "waiting": "running", "paused": "running", "review": "failed", "failed": "review"} {
		s.Status = from
		if err := setStatus(s, to); err == nil || s.Status != from {
			t.Fatalf("%s -> %s was allowed", from, to)
		}
	}
}

func TestAssistantSpeaksTheInterfaceLanguage(t *testing.T) {
	for _, c := range []struct {
		locale, text string
		refused      bool
	}{
		{"en", "Готово: создана Антонина, тифлинг-варвар 3-го уровня.", true},
		{"en", "Done: Антонина is a level 3 tiefling barbarian.", false},
		{"ru", "Done: Antonina is a level 3 tiefling barbarian.", true},
		{"ru", "Готово: Antonina, тифлинг-варвар 3-го уровня.", false},
		{"de", "Готово.", false},
	} {
		if err := spokenIn(rules.Locale(c.locale), c.text); (err != nil) != c.refused {
			t.Fatalf("%s %q: %v", c.locale, c.text, err)
		}
	}
}
