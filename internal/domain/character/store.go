package character

import (
	"crypto/rand"

	"github.com/promix1722/easydnd/internal/types"
)

// This file holds the rules every store has to apply on a write, so that the
// in-memory adapter and the SQL one cannot drift apart: each loads a record,
// calls one of these, and stores the result. The concurrency check itself --
// the lock, or the row lock -- is the adapter's; what it guards is here.

// Stamp fills in what a stored event always carries: an id, and the schema
// version it was written under.
func (l *Log) Stamp() {
	for i := range l.Events {
		e := &l.Events[i]
		if e.ID == "" {
			e.ID = "evt_" + rand.Text()
		}
		if e.SchemaVersion == 0 {
			e.SchemaVersion = 1
		}
	}
}

// ExpectSeq reports a *types.ValidationError when the log does not end at
// expectedSeq -- the check behind Repository.Append, Truncate and Rewrite.
func (c Character) ExpectSeq(expectedSeq int) error {
	if got := c.Log.LastSeq(); got != expectedSeq {
		return types.NewValidationError("character %q is at sequence %d, not %d", c.ID, got, expectedSeq)
	}
	return nil
}

// Commit replaces the log, advancing the revision, and is the whole of
// Repository.Commit once the record is in hand. A stale expectedRevision or a
// repeated command is a *types.ValidationError, and so is a log that does not
// validate; on any error c is unchanged.
func (c *Character) Commit(expectedRevision int, log Log, command string, checkpoint *Checkpoint) error {
	if command != "" {
		if _, ok := c.Commands[command]; ok {
			return types.NewValidationError("command already committed; reload character revision")
		}
	}
	if c.Revision != expectedRevision {
		return types.NewValidationError("stale character revision: got %d, expected %d", expectedRevision, c.Revision)
	}
	if err := log.Validate(); err != nil {
		return err
	}
	updated := log.Clone()
	updated.Stamp()
	if checkpoint != nil {
		cp := *checkpoint
		cp.Log = cp.Log.Clone()
		c.Checkpoints = append(c.Checkpoints, cp)
	}
	c.Revision += max(1, updated.Len()-c.Log.Len())
	c.Log = updated
	if command != "" {
		if c.Commands == nil {
			c.Commands = map[string]int{}
		}
		c.Commands[command] = c.Revision
	}
	return nil
}

// NewWithLog builds the record Repository.CreateWithLog stores: the log
// validated, cloned and stamped, at the revision an equivalent Create followed
// by one Commit would have reached. The id is the store's to assign.
func NewWithLog(owner OwnerID, folder FolderID, log Log) (Character, error) {
	if err := log.Validate(); err != nil {
		return Character{}, err
	}
	c := Character{Owner: owner, Folder: folder, Log: log.Clone(), Revision: max(1, log.Len())}
	c.Log.Stamp()
	return c, nil
}
