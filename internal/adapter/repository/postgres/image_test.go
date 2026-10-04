package postgres_test

import (
	"context"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/postgres"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	"github.com/promix1722/easydnd/internal/domain/imageasset"
)

func TestImageRepository(t *testing.T) {
	cfg := testConfig(t)
	repotest.RunImageRepository(t, func(t *testing.T) imageasset.Repository {
		pool := testPool(t, cfg)
		// spell_icons has no foreign keys, so a bare TRUNCATE is enough.
		if _, err := pool.Exec(context.Background(), `TRUNCATE spell_icons`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		return postgres.NewImageRepository(pool)
	})
}
