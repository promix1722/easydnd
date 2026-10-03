package character

import (
	"context"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
)

// WithDraft exposes the normal character read/edit usecases over one import
// log. The callback is synchronous and must not retain the service. Holding the
// coordinator lock makes editor reads and commits atomic against agent turns.
// Only Get and Commit are supported: drafts cannot be listed, moved or copied.
func (a *Agent) WithDraft(owner domain.OwnerID, id string, write bool, use func(*Service)) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := a.owned(owner, id)
	if err != nil {
		return err
	}
	if write && (s.Status == "saved" || s.Status == "running" || s.Status == "queued") {
		return types.NewValidationError("draft is not editable").Because("agent.changed")
	}
	repo := &agentDraftRepository{session: s, agent: a, writable: write}
	service := *a.service
	service.repo = repo
	use(&service)
	return nil
}

// Only Get and Commit are supported; unsupported persistence operations fail
// closed even if a future handler is accidentally added to the draft surface.
type agentDraftRepository struct {
	session  *AgentSession
	agent    *Agent
	writable bool
}

func (r *agentDraftRepository) Get(_ context.Context, id domain.ID) (domain.Character, error) {
	s := r.session
	if string(id) != s.ID {
		return domain.Character{}, types.NewNotFoundError("draft not found")
	}
	return domain.Character{ID: id, Owner: s.Owner, Folder: s.Folder, Log: s.Log.Clone(), Revision: s.Revision}, nil
}
func (r *agentDraftRepository) Commit(_ context.Context, id domain.ID, revision int, log domain.Log, _ string, _ *domain.Checkpoint) error {
	s := r.session
	if !r.writable || string(id) != s.ID || revision != s.Revision {
		return types.NewValidationError("draft changed").Because("agent.changed")
	}
	if err := log.Validate(); err != nil {
		return err
	}
	delta := max(1, log.Len()-s.Log.Len())
	r.agent.invalidate(s)
	s.Revision += delta - 1
	s.Log = log.Clone()
	s.Status = "paused"
	addAgentEvent(s, "edit", "", "", nil)
	s.Input = append(s.Input, raw(map[string]any{"role": "user", "content": "I edited the draft in the character editor. Re-read the build context before continuing and preserve my changes."}))
	return nil
}

func (*agentDraftRepository) Create(context.Context, domain.OwnerID, domain.FolderID) (domain.Character, error) {
	return domain.Character{}, types.NewNotImplementedError("draft operation unavailable")
}
func (*agentDraftRepository) List(context.Context, domain.OwnerID) ([]domain.Character, error) {
	return nil, types.NewNotImplementedError("draft operation unavailable")
}
func (*agentDraftRepository) SetFolder(context.Context, domain.ID, domain.FolderID) error {
	return types.NewNotImplementedError("draft operation unavailable")
}
func (*agentDraftRepository) Append(context.Context, domain.ID, int, ...domain.Event) error {
	return types.NewNotImplementedError("draft operation unavailable")
}
func (*agentDraftRepository) Truncate(context.Context, domain.ID, int, int) error {
	return types.NewNotImplementedError("draft operation unavailable")
}
func (*agentDraftRepository) Rewrite(context.Context, domain.ID, int, domain.Log) error {
	return types.NewNotImplementedError("draft operation unavailable")
}
func (*agentDraftRepository) Delete(context.Context, domain.ID) error {
	return types.NewNotImplementedError("draft operation unavailable")
}
