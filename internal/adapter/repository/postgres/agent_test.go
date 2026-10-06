package postgres_test

import (
	"context"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/postgres"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	agentuc "github.com/promix1722/easydnd/internal/usecase/agent"
)

func TestAgentStore(t *testing.T) {
	cfg := testConfig(t)
	repotest.RunAgentStore(t, func(t *testing.T) agentuc.Store {
		pool := testPool(t, cfg)
		if _, err := pool.Exec(context.Background(), `TRUNCATE agent_sessions CASCADE`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		return postgres.NewAgentStore(pool)
	})
}
