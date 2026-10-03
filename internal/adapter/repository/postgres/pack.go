package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/types"
)

type PackRepository struct{ pool *pgxpool.Pool }

func NewPackRepository(pool *pgxpool.Pool) *PackRepository { return &PackRepository{pool: pool} }
func (r *PackRepository) List(ctx context.Context) ([]pack.Record, error) {
	rows, err := r.pool.Query(ctx, "SELECT document FROM rule_packs ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []pack.Record{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var v pack.Record
		if err = json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *PackRepository) Get(ctx context.Context, id string) (pack.Record, error) {
	var b []byte
	var v pack.Record
	err := r.pool.QueryRow(ctx, "SELECT document FROM rule_packs WHERE id=$1", id).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, types.NewNotFoundError("pack not found")
	}
	if err != nil {
		return v, err
	}
	err = json.Unmarshal(b, &v)
	return v, err
}
func (r *PackRepository) Save(ctx context.Context, v pack.Record, expected int) error {
	v.Revision = expected + 1
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	q := "UPDATE rule_packs SET revision=$3, document=$4 WHERE id=$1 AND owner_id=$2 AND revision=$5"
	args := []any{v.ID, v.Owner, v.Revision, b, expected}
	if expected == 0 {
		q = "INSERT INTO rule_packs(id,owner_id,revision,document) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING"
		args = args[:4]
	}
	tag, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return types.NewValidationError("stale pack revision").Because("pack.stale")
	}
	return nil
}
func (r *PackRepository) Shares(ctx context.Context, g string) ([]pack.Share, error) {
	rows, err := r.pool.Query(ctx, "SELECT document FROM group_rule_packs WHERE group_id=$1 ORDER BY pack_id", g)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []pack.Share{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var v pack.Share
		if err = json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *PackRepository) PutShare(ctx context.Context, v pack.Share) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, "INSERT INTO group_rule_packs(group_id,pack_id,document) VALUES($1,$2,$3) ON CONFLICT(group_id,pack_id) DO UPDATE SET document=excluded.document", v.Group, v.Pack, b)
	return err
}
func (r *PackRepository) DeleteShare(ctx context.Context, g, p string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM group_rule_packs WHERE group_id=$1 AND pack_id=$2", g, p)
	return err
}
