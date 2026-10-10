package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/promix1722/easydnd/internal/domain/character"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/types"
)

// SharedRepository is the durable store for the characters groups have
// shared: one row per seat at a table.
type SharedRepository struct {
	pool *pgxpool.Pool
}

// NewSharedRepository returns a pool over the given pool.
func NewSharedRepository(pool *pgxpool.Pool) *SharedRepository {
	return &SharedRepository{pool: pool}
}

var _ domain.SharedRepository = (*SharedRepository)(nil)

// Share puts s in its group's pool.
func (r *SharedRepository) Share(ctx context.Context, s domain.Shared) error {
	switch {
	case s.Group == "":
		return types.NewValidationError("a shared character must name a group")
	case s.Character == "":
		return types.NewValidationError("a shared character must name a character")
	case s.Owner == "":
		return types.NewValidationError("a shared character must name an owner")
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO shared_characters (group_id, character_id, owner_id, shared_at) VALUES ($1, $2, $3, $4)`,
		string(s.Group), string(s.Character), string(s.Owner), s.SharedAt)
	switch {
	case isUniqueViolation(err, constraintSharedCharactersPK):
		return types.NewValidationError("character %q is already shared with this group", s.Character)
	case isForeignKeyViolation(err, constraintSharedCharactersGroupFK):
		return types.NewValidationError("group %q does not exist", s.Group)
	case err != nil:
		return types.WrapServerError(err, "insert shared character")
	}
	return nil
}

// Unshare takes c out of g's pool.
func (r *SharedRepository) Unshare(ctx context.Context, g group.ID, c character.ID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM shared_characters WHERE group_id = $1 AND character_id = $2`, string(g), string(c))
	if err != nil {
		return types.WrapServerError(err, "delete shared character")
	}
	if tag.RowsAffected() == 0 {
		return types.NewNotFoundError("character %q is not shared with this group", c)
	}
	return nil
}

// List returns g's whole pool, in the order characters were shared.
func (r *SharedRepository) List(ctx context.Context, g group.ID) ([]domain.Shared, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT group_id, character_id, owner_id, shared_at FROM shared_characters
		  WHERE group_id = $1 ORDER BY shared_at, character_id`, string(g))
	if err != nil {
		return nil, types.WrapServerError(err, "list shared characters")
	}
	defer rows.Close()

	out := make([]domain.Shared, 0)
	for rows.Next() {
		var s domain.Shared
		if err := rows.Scan(&s.Group, &s.Character, &s.Owner, &s.SharedAt); err != nil {
			return nil, types.WrapServerError(err, "scan shared character")
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, types.WrapServerError(err, "list shared characters")
	}
	return out, nil
}

// IsShared reports whether c is in g's pool.
func (r *SharedRepository) IsShared(ctx context.Context, g group.ID, c character.ID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM shared_characters WHERE group_id = $1 AND character_id = $2)`,
		string(g), string(c)).Scan(&ok)
	if err != nil {
		return false, types.WrapServerError(err, "check shared character")
	}
	return ok, nil
}

// GroupsSharing returns every group c has been shared into, sorted.
func (r *SharedRepository) GroupsSharing(ctx context.Context, c character.ID) ([]group.ID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT group_id FROM shared_characters WHERE character_id = $1 ORDER BY group_id`, string(c))
	if err != nil {
		return nil, types.WrapServerError(err, "list groups sharing")
	}
	defer rows.Close()

	var out []group.ID
	for rows.Next() {
		var g group.ID
		if err := rows.Scan(&g); err != nil {
			return nil, types.WrapServerError(err, "scan group sharing")
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, types.WrapServerError(err, "list groups sharing")
	}
	return out, nil
}

// UnshareEverywhere removes c from every pool.
func (r *SharedRepository) UnshareEverywhere(ctx context.Context, c character.ID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM shared_characters WHERE character_id = $1`, string(c)); err != nil {
		return types.WrapServerError(err, "unshare character everywhere")
	}
	return nil
}

// ClearGroup empties g's pool.
func (r *SharedRepository) ClearGroup(ctx context.Context, g group.ID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM shared_characters WHERE group_id = $1`, string(g)); err != nil {
		return types.WrapServerError(err, "clear shared characters")
	}
	return nil
}

// GameRepository is the durable game store.
//
// A game's roster is one JSON column on its row, the shape the in-memory
// store keeps it in. Every roster write loads it under a row lock, changes it
// in Go and writes it back, so MutateEntries is atomic by construction and
// the other roster methods are the same helper with a different change.
type GameRepository struct {
	pool *pgxpool.Pool
}

// NewGameRepository returns a store over the given pool.
func NewGameRepository(pool *pgxpool.Pool) *GameRepository {
	return &GameRepository{pool: pool}
}

var _ domain.Repository = (*GameRepository)(nil)

const gameColumns = `id, group_id, name, created_by, created_at`

func gameNotFound(id domain.ID) error {
	return types.NewNotFoundError("game %q", id).Because("game.notFound")
}

// Create stores g.
func (r *GameRepository) Create(ctx context.Context, g domain.Game) error {
	switch {
	case g.ID == "":
		return types.NewValidationError("game id must not be empty")
	case g.Group == "":
		return types.NewValidationError("a game must belong to a group")
	case g.Name == "":
		return types.NewValidationError("game name must not be empty")
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO games (`+gameColumns+`) VALUES ($1, $2, $3, $4, $5)`,
		string(g.ID), string(g.Group), g.Name, string(g.CreatedBy), g.CreatedAt)
	switch {
	case isUniqueViolation(err, constraintGamesPK):
		return types.NewValidationError("game %q already exists", g.ID)
	case isForeignKeyViolation(err, constraintGamesGroupFK):
		return types.NewValidationError("group %q does not exist", g.Group)
	case err != nil:
		return types.WrapServerError(err, "insert game")
	}
	return nil
}

// ByID returns the game.
func (r *GameRepository) ByID(ctx context.Context, id domain.ID) (domain.Game, error) {
	var g domain.Game
	err := r.pool.QueryRow(ctx, `SELECT `+gameColumns+` FROM games WHERE id = $1`, string(id)).
		Scan(&g.ID, &g.Group, &g.Name, &g.CreatedBy, &g.CreatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Game{}, gameNotFound(id)
	case err != nil:
		return domain.Game{}, types.WrapServerError(err, "load game")
	}
	return g, nil
}

// ListFor returns every game at g's table, most recently created first.
func (r *GameRepository) ListFor(ctx context.Context, g group.ID) ([]domain.Game, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+gameColumns+` FROM games WHERE group_id = $1 ORDER BY created_at DESC, id`, string(g))
	if err != nil {
		return nil, types.WrapServerError(err, "list games")
	}
	defer rows.Close()

	out := make([]domain.Game, 0)
	for rows.Next() {
		var g domain.Game
		if err := rows.Scan(&g.ID, &g.Group, &g.Name, &g.CreatedBy, &g.CreatedAt); err != nil {
			return nil, types.WrapServerError(err, "scan game")
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, types.WrapServerError(err, "list games")
	}
	return out, nil
}

// Rename changes the game's name.
func (r *GameRepository) Rename(ctx context.Context, id domain.ID, name string) error {
	if name == "" {
		return types.NewValidationError("game name must not be empty")
	}
	tag, err := r.pool.Exec(ctx, `UPDATE games SET name = $2 WHERE id = $1`, string(id), name)
	if err != nil {
		return types.WrapServerError(err, "rename game")
	}
	if tag.RowsAffected() == 0 {
		return gameNotFound(id)
	}
	return nil
}

// Delete removes the game and, with it, its roster.
func (r *GameRepository) Delete(ctx context.Context, id domain.ID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM games WHERE id = $1`, string(id))
	if err != nil {
		return types.WrapServerError(err, "delete game")
	}
	if tag.RowsAffected() == 0 {
		return gameNotFound(id)
	}
	return nil
}

// DeleteForGroup removes every game at g's table.
func (r *GameRepository) DeleteForGroup(ctx context.Context, g group.ID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM games WHERE group_id = $1`, string(g)); err != nil {
		return types.WrapServerError(err, "delete group games")
	}
	return nil
}

// Characters returns id's roster, in the order characters were added.
func (r *GameRepository) Characters(ctx context.Context, id domain.ID) ([]domain.Entry, error) {
	entries, err := readRoster(ctx, r.pool, id, "")
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// readRoster loads one game's roster; lock is "" or "FOR UPDATE".
func readRoster(ctx context.Context, q querier, id domain.ID, lock string) ([]domain.Entry, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT roster FROM games WHERE id = $1 `+lock, string(id)).Scan(&raw)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, gameNotFound(id)
	case err != nil:
		return nil, types.WrapServerError(err, "load roster")
	}
	return decodeRoster(raw)
}

func decodeRoster(raw []byte) ([]domain.Entry, error) {
	entries := []domain.Entry{}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, types.WrapServerError(err, "decode roster")
	}
	return entries, nil
}

func writeRoster(ctx context.Context, q querier, id domain.ID, entries []domain.Entry) error {
	if entries == nil {
		entries = []domain.Entry{}
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return types.WrapServerError(err, "encode roster")
	}
	if _, err := q.Exec(ctx, `UPDATE games SET roster = $2 WHERE id = $1`, string(id), raw); err != nil {
		return types.WrapServerError(err, "update roster")
	}
	return nil
}

// MutateEntries runs one roster change atomically under the game's row lock.
// A change that returns an error leaves the stored roster untouched.
func (r *GameRepository) MutateEntries(ctx context.Context, id domain.ID, change func([]domain.Entry) ([]domain.Entry, error)) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return types.WrapServerError(err, "begin mutate roster")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	entries, err := readRoster(ctx, tx, id, "FOR UPDATE")
	if err != nil {
		return err
	}
	entries, err = change(entries)
	if err != nil {
		return err
	}
	if err := writeRoster(ctx, tx, id, entries); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return types.WrapServerError(err, "commit mutate roster")
	}
	return nil
}

// AddCharacters seats cs at id's table, leaving those already seated alone.
func (r *GameRepository) AddCharacters(ctx context.Context, id domain.ID, cs []character.ID, at time.Time) error {
	return r.MutateEntries(ctx, id, func(roster []domain.Entry) ([]domain.Entry, error) {
		seated := make(map[character.ID]struct{}, len(roster))
		for _, e := range roster {
			seated[e.Character] = struct{}{}
		}
		for _, c := range cs {
			if c == "" {
				return nil, types.NewValidationError("a roster entry must name a character")
			}
			if _, ok := seated[c]; ok {
				continue
			}
			seated[c] = struct{}{}
			roster = append(roster, domain.Entry{ID: "pc_" + string(c), Kind: "player", Character: c, AddedAt: at})
		}
		return roster, nil
	})
}

// RemoveCharacter takes c off id's roster.
func (r *GameRepository) RemoveCharacter(ctx context.Context, id domain.ID, c character.ID) error {
	return r.MutateEntries(ctx, id, func(roster []domain.Entry) ([]domain.Entry, error) {
		rest := slices.DeleteFunc(roster, func(e domain.Entry) bool {
			return e.Kind != "monster" && e.Character == c
		})
		if len(rest) == len(roster) {
			return nil, types.NewNotFoundError("character %q is not in this game", c)
		}
		return rest, nil
	})
}

// RemoveFromGroupGames drops c from every game at g's table, in one
// transaction, so there is no half-way state in which it is seated at some.
func (r *GameRepository) RemoveFromGroupGames(ctx context.Context, g group.ID, c character.ID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return types.WrapServerError(err, "begin remove from group games")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `SELECT id, roster FROM games WHERE group_id = $1 ORDER BY id FOR UPDATE`, string(g))
	if err != nil {
		return types.WrapServerError(err, "list group rosters")
	}
	type change struct {
		id      domain.ID
		entries []domain.Entry
	}
	var changes []change
	for rows.Next() {
		var id domain.ID
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return types.WrapServerError(err, "scan roster")
		}
		entries, err := decodeRoster(raw)
		if err != nil {
			rows.Close()
			return err
		}
		rest := slices.DeleteFunc(entries, func(e domain.Entry) bool {
			return e.Kind != "monster" && e.Character == c
		})
		if len(rest) != len(entries) {
			changes = append(changes, change{id, rest})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return types.WrapServerError(err, "list group rosters")
	}
	for _, ch := range changes {
		if err := writeRoster(ctx, tx, ch.id, ch.entries); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return types.WrapServerError(err, "commit remove from group games")
	}
	return nil
}
