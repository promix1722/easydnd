package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
)

// CharacterRepository is the durable character store.
//
// A character is one row holding its whole log as JSON, which is the storage
// shape docs/dnd.md fixes: the log is small, and one record is what makes the
// revision check a single UPDATE. The rules a write applies -- the revision
// and sequence guards, event stamping -- are the domain's (Character.Commit
// and friends); this adapter loads the row under a lock, calls them, and
// stores the result.
//
// The ids come from a sequence with the same shape the in-memory store
// mints, so nothing downstream can tell the two apart, and -- the point --
// an id never names a different character after a restart.
type CharacterRepository struct {
	pool *pgxpool.Pool
}

// NewCharacterRepository returns a store over the given pool.
func NewCharacterRepository(pool *pgxpool.Pool) *CharacterRepository {
	return &CharacterRepository{pool: pool}
}

var _ domain.Repository = (*CharacterRepository)(nil)

const characterColumns = `id, owner_id, folder_id, public, revision, log, checkpoints, commands`

func characterNotFound(id domain.ID) error {
	return types.NewNotFoundError("character %q", id).Because("character.notFound")
}

// scanCharacter reads one row in characterColumns order.
func scanCharacter(row pgx.Row) (domain.Character, error) {
	var c domain.Character
	var log, checkpoints, commands []byte
	if err := row.Scan(&c.ID, &c.Owner, &c.Folder, &c.Public, &c.Revision, &log, &checkpoints, &commands); err != nil {
		return domain.Character{}, err
	}
	if err := json.Unmarshal(log, &c.Log); err != nil {
		return domain.Character{}, types.WrapServerError(err, "decode character log")
	}
	if checkpoints != nil {
		if err := json.Unmarshal(checkpoints, &c.Checkpoints); err != nil {
			return domain.Character{}, types.WrapServerError(err, "decode character checkpoints")
		}
	}
	if commands != nil {
		if err := json.Unmarshal(commands, &c.Commands); err != nil {
			return domain.Character{}, types.WrapServerError(err, "decode character commands")
		}
	}
	return c, nil
}

// encode renders the three JSON columns. Empty checkpoints and commands are
// NULL rather than "[]" and "{}", so a character that never had either reads
// back with the nil the in-memory store returns.
func encode(c domain.Character) (log, checkpoints, commands []byte, err error) {
	if log, err = json.Marshal(c.Log); err != nil {
		return nil, nil, nil, types.WrapServerError(err, "encode character log")
	}
	if len(c.Checkpoints) > 0 {
		if checkpoints, err = json.Marshal(c.Checkpoints); err != nil {
			return nil, nil, nil, types.WrapServerError(err, "encode character checkpoints")
		}
	}
	if len(c.Commands) > 0 {
		if commands, err = json.Marshal(c.Commands); err != nil {
			return nil, nil, nil, types.WrapServerError(err, "encode character commands")
		}
	}
	return log, checkpoints, commands, nil
}

// Create stores a new empty character for owner, filed in folder.
func (r *CharacterRepository) Create(ctx context.Context, owner domain.OwnerID, folder domain.FolderID) (domain.Character, error) {
	c := domain.Character{Owner: owner, Folder: folder}
	return r.insert(ctx, c)
}

// CreateWithLog stores a new character and its first log in one write.
func (r *CharacterRepository) CreateWithLog(ctx context.Context, owner domain.OwnerID, folder domain.FolderID, log domain.Log) (domain.Character, error) {
	c, err := domain.NewWithLog(owner, folder, log)
	if err != nil {
		return domain.Character{}, err
	}
	return r.insert(ctx, c)
}

func (r *CharacterRepository) insert(ctx context.Context, c domain.Character) (domain.Character, error) {
	log, checkpoints, commands, err := encode(c)
	if err != nil {
		return domain.Character{}, err
	}
	err = r.pool.QueryRow(ctx,
		`INSERT INTO characters (owner_id, folder_id, public, revision, log, checkpoints, commands)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		string(c.Owner), string(c.Folder), c.Public, c.Revision, log, checkpoints, commands,
	).Scan(&c.ID)
	if err != nil {
		return domain.Character{}, types.WrapServerError(err, "insert character")
	}
	return c, nil
}

// Get returns the character with the given ID.
func (r *CharacterRepository) Get(ctx context.Context, id domain.ID) (domain.Character, error) {
	c, err := scanCharacter(r.pool.QueryRow(ctx,
		`SELECT `+characterColumns+` FROM characters WHERE id = $1`, string(id)))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Character{}, characterNotFound(id)
	case err != nil:
		return domain.Character{}, types.WrapServerError(err, "load character")
	}
	return c, nil
}

// List returns every character owned by owner, oldest first.
func (r *CharacterRepository) List(ctx context.Context, owner domain.OwnerID) ([]domain.Character, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+characterColumns+` FROM characters WHERE owner_id = $1 ORDER BY created_at, id`,
		string(owner))
	if err != nil {
		return nil, types.WrapServerError(err, "list characters")
	}
	defer rows.Close()

	out := make([]domain.Character, 0)
	for rows.Next() {
		c, err := scanCharacter(rows)
		if err != nil {
			return nil, types.WrapServerError(err, "scan character")
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, types.WrapServerError(err, "list characters")
	}
	return out, nil
}

// update loads a character under a row lock, applies fn, and stores it. fn's
// error comes back as is and leaves the row untouched; it is where the domain
// guards -- stale revision, stale sequence, a log that does not validate --
// are raised.
func (r *CharacterRepository) update(ctx context.Context, id domain.ID, fn func(*domain.Character) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return types.WrapServerError(err, "begin update character")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	c, err := scanCharacter(tx.QueryRow(ctx,
		`SELECT `+characterColumns+` FROM characters WHERE id = $1 FOR UPDATE`, string(id)))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return characterNotFound(id)
	case err != nil:
		return types.WrapServerError(err, "load character")
	}
	if err := fn(&c); err != nil {
		return err
	}
	log, checkpoints, commands, err := encode(c)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`UPDATE characters SET folder_id = $2, public = $3, revision = $4, log = $5, checkpoints = $6, commands = $7 WHERE id = $1`,
		string(id), string(c.Folder), c.Public, c.Revision, log, checkpoints, commands)
	if err != nil {
		return types.WrapServerError(err, "update character")
	}
	if err := tx.Commit(ctx); err != nil {
		return types.WrapServerError(err, "commit update character")
	}
	return nil
}

// SetFolder files a character in another folder.
func (r *CharacterRepository) SetFolder(ctx context.Context, id domain.ID, folder domain.FolderID) error {
	return r.update(ctx, id, func(c *domain.Character) error {
		c.Folder = folder
		return nil
	})
}

// SetPublic opens or hides a character.
func (r *CharacterRepository) SetPublic(ctx context.Context, id domain.ID, public bool) error {
	return r.update(ctx, id, func(c *domain.Character) error {
		c.Public = public
		return nil
	})
}

// Append adds events to a character's log, rejecting a stale expectedSeq.
func (r *CharacterRepository) Append(ctx context.Context, id domain.ID, expectedSeq int, events ...domain.Event) error {
	return r.update(ctx, id, func(c *domain.Character) error {
		if err := c.ExpectSeq(expectedSeq); err != nil {
			return err
		}
		if err := c.Log.Append(events...); err != nil {
			return err
		}
		c.Revision += max(1, len(events))
		return nil
	})
}

// Truncate drops every event after afterSeq, rejecting a stale expectedSeq.
func (r *CharacterRepository) Truncate(ctx context.Context, id domain.ID, expectedSeq, afterSeq int) error {
	return r.update(ctx, id, func(c *domain.Character) error {
		if err := c.ExpectSeq(expectedSeq); err != nil {
			return err
		}
		if err := c.Log.Truncate(afterSeq); err != nil {
			return err
		}
		c.Revision++
		return nil
	})
}

// Rewrite replaces a character's whole log, rejecting a stale expectedSeq.
func (r *CharacterRepository) Rewrite(ctx context.Context, id domain.ID, expectedSeq int, log domain.Log) error {
	return r.update(ctx, id, func(c *domain.Character) error {
		if err := c.ExpectSeq(expectedSeq); err != nil {
			return err
		}
		if err := log.Validate(); err != nil {
			return err
		}
		c.Log = log.Clone()
		c.Revision++
		return nil
	})
}

// Commit is the atomic write boundary for all application log mutations.
func (r *CharacterRepository) Commit(ctx context.Context, id domain.ID, expectedRevision int, log domain.Log, command string, checkpoint *domain.Checkpoint) error {
	return r.update(ctx, id, func(c *domain.Character) error {
		return c.Commit(expectedRevision, log, command, checkpoint)
	})
}

// Delete removes a character.
func (r *CharacterRepository) Delete(ctx context.Context, id domain.ID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM characters WHERE id = $1`, string(id))
	if err != nil {
		return types.WrapServerError(err, "delete character")
	}
	if tag.RowsAffected() == 0 {
		return characterNotFound(id)
	}
	return nil
}
