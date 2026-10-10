package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
)

// FolderRepository is the durable folder store.
//
// Error strings are copied verbatim from the in-memory adapter, as in the
// other stores here: the HTTP layer renders them into response bodies.
type FolderRepository struct {
	pool *pgxpool.Pool
}

// NewFolderRepository returns a store over the given pool.
func NewFolderRepository(pool *pgxpool.Pool) *FolderRepository {
	return &FolderRepository{pool: pool}
}

var _ domain.FolderRepository = (*FolderRepository)(nil)

const folderColumns = `id, owner_id, name, is_default, position`

func folderNotFound(id domain.FolderID) error {
	return types.NewNotFoundError("folder %q", id).Because("folder.notFound")
}

func scanFolder(row pgx.Row) (domain.Folder, error) {
	var f domain.Folder
	err := row.Scan(&f.ID, &f.Owner, &f.Name, &f.Default, &f.Position)
	return f, err
}

// EnsureDefault returns owner's default folder, creating it on first use.
//
// folders_one_default_idx is what makes this safe under two first requests
// arriving together: the second INSERT hits the partial unique index and does
// nothing, and both then read the one row.
func (r *FolderRepository) EnsureDefault(ctx context.Context, owner domain.OwnerID) (domain.Folder, error) {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO folders (owner_id, name, is_default, position)
		 VALUES ($1, $2, true, (SELECT count(*) FROM folders WHERE owner_id = $1))
		 ON CONFLICT (owner_id) WHERE is_default DO NOTHING`,
		string(owner), domain.DefaultFolderName)
	if err != nil {
		return domain.Folder{}, types.WrapServerError(err, "ensure default folder")
	}
	f, err := scanFolder(r.pool.QueryRow(ctx,
		`SELECT `+folderColumns+` FROM folders WHERE owner_id = $1 AND is_default`, string(owner)))
	if err != nil {
		return domain.Folder{}, types.WrapServerError(err, "load default folder")
	}
	return f, nil
}

// Create stores a new folder for owner. It lands last, as in memory: its
// position is the count of folders the owner already has.
func (r *FolderRepository) Create(ctx context.Context, owner domain.OwnerID, name string) (domain.Folder, error) {
	f, err := scanFolder(r.pool.QueryRow(ctx,
		`INSERT INTO folders (owner_id, name, is_default, position)
		 VALUES ($1, $2, false, (SELECT count(*) FROM folders WHERE owner_id = $1))
		 RETURNING `+folderColumns,
		string(owner), name))
	if err != nil {
		return domain.Folder{}, types.WrapServerError(err, "insert folder")
	}
	return f, nil
}

// Get returns the folder with the given ID.
func (r *FolderRepository) Get(ctx context.Context, id domain.FolderID) (domain.Folder, error) {
	f, err := scanFolder(r.pool.QueryRow(ctx,
		`SELECT `+folderColumns+` FROM folders WHERE id = $1`, string(id)))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Folder{}, folderNotFound(id)
	case err != nil:
		return domain.Folder{}, types.WrapServerError(err, "load folder")
	}
	return f, nil
}

// List returns every folder owned by owner, the default one first, then by
// position with the id breaking a tie.
func (r *FolderRepository) List(ctx context.Context, owner domain.OwnerID) ([]domain.Folder, error) {
	return r.list(ctx, r.pool, owner, "")
}

func (r *FolderRepository) list(ctx context.Context, q querier, owner domain.OwnerID, lock string) ([]domain.Folder, error) {
	rows, err := q.Query(ctx,
		`SELECT `+folderColumns+` FROM folders WHERE owner_id = $1
		  ORDER BY is_default DESC, position, id `+lock, string(owner))
	if err != nil {
		return nil, types.WrapServerError(err, "list folders")
	}
	defer rows.Close()

	out := make([]domain.Folder, 0)
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, types.WrapServerError(err, "scan folder")
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, types.WrapServerError(err, "list folders")
	}
	return out, nil
}

// Reorder sets the order of owner's non-default folders.
//
// The owner's rows are locked for the transaction, so the set the check ran
// against is the set the positions are written to.
func (r *FolderRepository) Reorder(ctx context.Context, owner domain.OwnerID, ids []domain.FolderID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return types.WrapServerError(err, "begin reorder folders")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	have, err := r.list(ctx, tx, owner, "FOR UPDATE")
	if err != nil {
		return err
	}
	if err := domain.CheckReorder(have, ids); err != nil {
		return err
	}
	// Numbered from one, leaving zero to the default folder.
	keys := make([]string, len(ids))
	positions := make([]int, len(ids))
	for i, id := range ids {
		keys[i], positions[i] = string(id), i+1
	}
	_, err = tx.Exec(ctx,
		`UPDATE folders AS f SET position = u.position
		   FROM unnest($1::text[], $2::int[]) AS u (id, position)
		  WHERE f.id = u.id`, keys, positions)
	if err != nil {
		return types.WrapServerError(err, "reorder folders")
	}
	if err := tx.Commit(ctx); err != nil {
		return types.WrapServerError(err, "commit reorder folders")
	}
	return nil
}

// Rename changes a folder's name. The default folder is renameable.
func (r *FolderRepository) Rename(ctx context.Context, id domain.FolderID, name string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE folders SET name = $2 WHERE id = $1`, string(id), name)
	if err != nil {
		return types.WrapServerError(err, "rename folder")
	}
	if tag.RowsAffected() == 0 {
		return folderNotFound(id)
	}
	return nil
}

// Delete removes a folder, refusing the default one. It says nothing about
// the characters filed in it; that is the usecase's cascade.
func (r *FolderRepository) Delete(ctx context.Context, id domain.FolderID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM folders WHERE id = $1 AND NOT is_default`, string(id))
	if err != nil {
		return types.WrapServerError(err, "delete folder")
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	// Nothing went: either the folder is not there, or it is the default.
	if _, err := r.Get(ctx, id); err != nil {
		return err
	}
	return types.NewValidationError("folder %q is the default folder and cannot be deleted", id)
}
