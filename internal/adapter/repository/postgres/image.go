package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/promix1722/easydnd/internal/domain/imageasset"
	"github.com/promix1722/easydnd/internal/types"
)

// ImageRepository stores seeded artwork in the spell_icons table. The bytes
// live in the database rather than on disk so that a serve never depends on a
// filesystem the deploy did not ship.
type ImageRepository struct{ pool *pgxpool.Pool }

func NewImageRepository(pool *pgxpool.Pool) *ImageRepository {
	return &ImageRepository{pool: pool}
}

func (r *ImageRepository) Get(ctx context.Context, slug string) (imageasset.Image, error) {
	var img imageasset.Image
	img.Slug = slug
	err := r.pool.QueryRow(ctx,
		"SELECT revision, content_type, data FROM spell_icons WHERE slug=$1", slug).
		Scan(&img.Revision, &img.ContentType, &img.Data)
	if errors.Is(err, pgx.ErrNoRows) {
		return img, types.NewNotFoundError("image not found")
	}
	if err != nil {
		return img, err
	}
	return img, nil
}

func (r *ImageRepository) Upsert(ctx context.Context, img imageasset.Image) error {
	// The WHERE is what makes a re-import cheap: ON CONFLICT DO UPDATE without
	// it would rewrite the row -- and hand out a new tuple version for WAL,
	// vacuum and every standby to churn through -- even when nothing changed.
	// Skipping on equal revision keeps the same-seed upsert a read-only
	// no-op.
	_, err := r.pool.Exec(ctx,
		`INSERT INTO spell_icons (slug, revision, content_type, data)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (slug) DO UPDATE
		   SET revision = EXCLUDED.revision,
		       content_type = EXCLUDED.content_type,
		       data = EXCLUDED.data
		   WHERE spell_icons.revision <> EXCLUDED.revision`,
		img.Slug, img.Revision, img.ContentType, img.Data)
	return err
}
