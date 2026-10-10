package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
	agentuc "github.com/promix1722/easydnd/internal/usecase/agent"
)

// AgentStore keeps AI Wizard sessions in agent_sessions, agent_events and
// agent_files. Every write a turn makes is one statement or one transaction
// whose first statement names the lease holder and the generation, so a turn
// that was stopped or taken over cannot write.
type AgentStore struct{ pool *pgxpool.Pool }

func NewAgentStore(pool *pgxpool.Pool) *AgentStore { return &AgentStore{pool: pool} }

var _ agentuc.Store = (*AgentStore)(nil)

const agentColumns = "id, owner_id, status, revision, generation, finished, created_at"

// agentErr is WrapServerError that leaves a nil error nil.
func agentErr(err error, message string) error {
	if err == nil {
		return nil
	}
	return types.WrapServerError(err, "%s", message)
}

// querier is what both the pool and a transaction can do.
type querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func scanAgent(row pgx.Row, document *[]byte) (agentuc.Record, error) {
	var rec agentuc.Record
	dest := []any{&rec.ID, &rec.Owner, &rec.Status, &rec.Revision, &rec.Generation, &rec.Finished, &rec.Created}
	if document != nil {
		dest = append(dest, document)
	}
	err := row.Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return rec, agentuc.ErrSessionNotFound()
	}
	if err != nil {
		return rec, agentErr(err, "read import session")
	}
	if document != nil {
		rec.Document = *document
	}
	return rec, nil
}

// agentEvents reads a session's events past after, and how many it has in all.
func agentEvents(ctx context.Context, q querier, rec *agentuc.Record, after int) error {
	if err := q.QueryRow(ctx, "SELECT coalesce(max(id), 0) FROM agent_events WHERE session_id=$1", rec.ID).Scan(&rec.Count); err != nil {
		return agentErr(err, "count import events")
	}
	if after < 0 || after >= rec.Count {
		return nil
	}
	rows, err := q.Query(ctx, "SELECT body FROM agent_events WHERE session_id=$1 AND id>$2 ORDER BY id", rec.ID, after)
	if err != nil {
		return agentErr(err, "read import events")
	}
	defer rows.Close()
	for rows.Next() {
		var body []byte
		var e agentuc.AgentEvent
		if err = rows.Scan(&body); err == nil {
			err = json.Unmarshal(body, &e)
		}
		if err != nil {
			return agentErr(err, "read import event")
		}
		rec.Events = append(rec.Events, e)
	}
	return agentErr(rows.Err(), "read import events")
}

// agentWrite stores what a Save or an Update adds: the events past rec.Count and
// the new attachments.
func agentWrite(ctx context.Context, q querier, rec agentuc.Record) error {
	for _, e := range rec.Events {
		if e.ID <= rec.Count {
			continue
		}
		body, err := json.Marshal(e)
		if err == nil {
			_, err = q.Exec(ctx, "INSERT INTO agent_events(session_id, id, body) VALUES($1, $2, $3)", rec.ID, e.ID, body)
		}
		if err != nil {
			return agentErr(err, "store import event")
		}
	}
	for _, f := range rec.Files {
		if _, err := q.Exec(ctx, "INSERT INTO agent_files(session_id, name, mime, data) VALUES($1, $2, $3, $4)", rec.ID, f.Name, f.MIME, f.Data); err != nil {
			return agentErr(err, "store import attachment")
		}
	}
	return nil
}

func (r *AgentStore) Create(ctx context.Context, rec agentuc.Record, limit int) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, agentErr(err, "begin create import session")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `INSERT INTO agent_sessions(id, owner_id, status, revision, generation, finished, created_at, touched_at, document)
		SELECT $1, $2, $3, $4, $5, $6, $7, now(), $8 WHERE (SELECT count(*) FROM agent_sessions) < $9`,
		rec.ID, rec.Owner, rec.Status, rec.Revision, rec.Generation, rec.Finished, rec.Created, rec.Document, limit)
	if err != nil {
		return false, agentErr(err, "create import session")
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	rec.Count = 0
	if err = agentWrite(ctx, tx, rec); err != nil {
		return false, err
	}
	return true, agentErr(tx.Commit(ctx), "commit create import session")
}

func (r *AgentStore) Get(ctx context.Context, id string) (agentuc.Record, error) {
	var document []byte
	rec, err := scanAgent(r.pool.QueryRow(ctx, "UPDATE agent_sessions SET touched_at=now() WHERE id=$1 RETURNING "+agentColumns+", document", id), &document)
	if err != nil {
		return rec, err
	}
	return rec, agentEvents(ctx, r.pool, &rec, 0)
}

func (r *AgentStore) Tail(ctx context.Context, id string, after int) (agentuc.Record, error) {
	rec, err := scanAgent(r.pool.QueryRow(ctx, "SELECT "+agentColumns+" FROM agent_sessions WHERE id=$1", id), nil)
	if err != nil {
		return rec, err
	}
	return rec, agentEvents(ctx, r.pool, &rec, after)
}

func (r *AgentStore) List(ctx context.Context, owner domain.OwnerID) ([]agentuc.Record, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+agentColumns+", document FROM agent_sessions WHERE owner_id=$1 ORDER BY created_at", owner)
	if err != nil {
		return nil, agentErr(err, "list import sessions")
	}
	defer rows.Close()
	out := []agentuc.Record{}
	for rows.Next() {
		var document []byte
		rec, err := scanAgent(rows, &document)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, agentErr(rows.Err(), "list import sessions")
}

func (r *AgentStore) Files(ctx context.Context, id string) ([]agentuc.AgentFile, error) {
	rows, err := r.pool.Query(ctx, "SELECT name, mime, data FROM agent_files WHERE session_id=$1 ORDER BY name", id)
	if err != nil {
		return nil, agentErr(err, "read import attachments")
	}
	defer rows.Close()
	out := []agentuc.AgentFile{}
	for rows.Next() {
		var f agentuc.AgentFile
		if err = rows.Scan(&f.Name, &f.MIME, &f.Data); err != nil {
			return nil, agentErr(err, "read import attachment")
		}
		out = append(out, f)
	}
	return out, agentErr(rows.Err(), "read import attachments")
}

func (r *AgentStore) Update(ctx context.Context, id string, change func(*agentuc.Record) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return agentErr(err, "begin update import session")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var document []byte
	rec, err := scanAgent(tx.QueryRow(ctx, "SELECT "+agentColumns+", document FROM agent_sessions WHERE id=$1 FOR UPDATE", id), &document)
	if err == nil {
		err = agentEvents(ctx, tx, &rec, 0)
	}
	if err == nil {
		err = change(&rec)
	}
	if err != nil {
		return err
	}
	// A session that is no longer running is nobody's: the lease goes with
	// the status, which is what agent_sessions_lease_check demands.
	if _, err = tx.Exec(ctx, `UPDATE agent_sessions SET status=$2, revision=$3, generation=$4, finished=$5, document=$6, touched_at=now(),
		lease_owner=CASE WHEN $2='running' THEN lease_owner END, lease_until=CASE WHEN $2='running' THEN lease_until END WHERE id=$1`,
		rec.ID, rec.Status, rec.Revision, rec.Generation, rec.Finished, rec.Document); err != nil {
		return agentErr(err, "update import session")
	}
	if err = agentWrite(ctx, tx, rec); err != nil {
		return err
	}
	return agentErr(tx.Commit(ctx), "commit update import session")
}

func (r *AgentStore) Claim(ctx context.Context, instance string, lease time.Duration, busy []string) (agentuc.Record, bool, error) {
	// SKIP LOCKED is what lets every worker of every process ask at once and
	// each be handed a different session, or none.
	var document []byte
	rec, err := scanAgent(r.pool.QueryRow(ctx, `UPDATE agent_sessions SET status='running', lease_owner=$1, lease_until=now()+make_interval(secs => $2), revision=revision+1, touched_at=now()
		WHERE id=(SELECT id FROM agent_sessions WHERE (status='queued' OR (status='running' AND lease_until<now())) AND NOT id=ANY(coalesce($3::text[], '{}'))
			ORDER BY touched_at LIMIT 1 FOR UPDATE SKIP LOCKED)
		RETURNING `+agentColumns+", document", instance, lease.Seconds(), busy), &document)
	if types.IsNotFound(err) {
		return rec, false, nil
	}
	if err != nil {
		return rec, false, err
	}
	return rec, true, agentEvents(ctx, r.pool, &rec, 0)
}

func (r *AgentStore) Save(ctx context.Context, rec agentuc.Record, instance string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, agentErr(err, "begin save import session")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE agent_sessions SET status=$2, revision=$3, finished=$4, document=$5, touched_at=now(),
		lease_owner=CASE WHEN $2='running' THEN lease_owner END, lease_until=CASE WHEN $2='running' THEN lease_until END
		WHERE id=$1 AND lease_owner=$6 AND generation=$7`,
		rec.ID, rec.Status, rec.Revision, rec.Finished, rec.Document, instance, rec.Generation)
	if err != nil {
		return false, agentErr(err, "save import session")
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if err = agentWrite(ctx, tx, rec); err != nil {
		return false, err
	}
	return true, agentErr(tx.Commit(ctx), "commit save import session")
}

func (r *AgentStore) Append(ctx context.Context, id, instance string, generation int, e agentuc.AgentEvent) (bool, error) {
	body, err := json.Marshal(e)
	if err != nil {
		return false, agentErr(err, "encode import event")
	}
	// The UPDATE takes the row's lock, so this waits its turn behind an
	// Update and then finds the generation moved.
	tag, err := r.pool.Exec(ctx, `WITH held AS (UPDATE agent_sessions SET touched_at=now() WHERE id=$1 AND lease_owner=$2 AND generation=$3 RETURNING id)
		INSERT INTO agent_events(session_id, id, body) SELECT id, $4, $5 FROM held`, id, instance, generation, e.ID, body)
	if err != nil {
		return false, agentErr(err, "append import event")
	}
	return tag.RowsAffected() == 1, nil
}

func (r *AgentStore) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM agent_sessions WHERE id=$1", id)
	return agentErr(err, "delete import session")
}

func (r *AgentStore) Sweep(ctx context.Context, idle time.Duration) (int, error) {
	tag, err := r.pool.Exec(ctx, "DELETE FROM agent_sessions WHERE touched_at < now()-make_interval(secs => $1) AND (status<>'running' OR lease_until<now())", idle.Seconds())
	return int(tag.RowsAffected()), agentErr(err, "sweep import sessions")
}
