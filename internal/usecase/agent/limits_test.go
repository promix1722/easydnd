package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
	agentuc "github.com/promix1722/easydnd/internal/usecase/agent"
)

func TestWizardRunsPerDayLimit(t *testing.T) {
	model := modelFunc(func(context.Context, agentuc.AgentRequest, func(string)) (agentuc.AgentResponse, error) {
		return agentuc.AgentResponse{}, nil
	})
	svc := newService(t)
	l := types.DefaultLimits
	l.WizardRunsPerDay = 1
	svc.SetLimits(l)
	a := agentuc.NewAgent(svc, answering{model}, agentuc.AgentConfig{Workers: 1})
	defer a.Close()

	if _, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), ""); err != nil {
		t.Fatal(err)
	}
	_, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "")
	var refused *types.ValidationError
	if !errors.As(err, &refused) || refused.Reason != "limit.wizardRuns" {
		t.Fatalf("error = %v, want limit.wizardRuns", err)
	}
	if _, err := a.Create(context.Background(), "somebody-else", "", rules.DefaultLocale, agentFile(), ""); err != nil {
		t.Fatalf("another owner's first run: %v", err)
	}
}
