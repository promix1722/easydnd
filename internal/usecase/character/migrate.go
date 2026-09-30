package character

import (
	"context"
	"reflect"
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// Mappings are explicit author/user decisions, never guesses based on names.
type Mappings struct {
	Entities map[rules.Ref]rules.Ref
	Prompts  map[rules.Slug]rules.Slug
	Options  map[rules.Slug]rules.Slug
	Paths    map[domain.Path]domain.Path
}
type MigrationIssue struct {
	Seq             int
	EventID, Reason string
}
type Migration struct {
	Revision      int
	Before, After domain.State
	Changed       []string
	Issues        []MigrationIssue
	Lock          pack.Lock
}

// Migrate revalidates the complete candidate without dropping invalid choices.
// Applying a valid preview repeats validation and commits with revision CAS.
func (s *Service) Migrate(ctx context.Context, owner domain.OwnerID, id domain.ID, locale rules.Locale, expectedRevision int, target pack.Lock, mappings Mappings, commit bool) (Migration, error) {
	c, old, err := s.load(ctx, owner, id, locale)
	if err != nil {
		return Migration{}, err
	}
	if c.Revision != expectedRevision {
		return Migration{}, types.NewValidationError("stale migration revision")
	}
	if err := target.Validate(); err != nil {
		return Migration{}, types.NewValidationError("invalid rules lock: %v", err)
	}
	next, err := catalog.LoadLocked(ctx, s.catalog, locale, target)
	if err != nil {
		return Migration{}, err
	}
	before, err := domain.Project(c.Log, old)
	if err != nil {
		return Migration{}, err
	}
	result := Migration{Revision: c.Revision, Before: before, Lock: target.Clone()}
	candidate := c.Log.Clone()
	if len(candidate.Events) == 0 {
		return Migration{}, types.NewValidationError("empty character")
	}
	candidate.Events[0].RulesLock = target.Clone()
	for i := range candidate.Events {
		e := &candidate.Events[i]
		if mapped, ok := mappings.Entities[e.Ref]; ok {
			e.Ref = mapped
		}
		if mapped, ok := mappings.Entities[rules.NewRef(rules.RefResource, e.Resource)]; ok {
			e.Resource = mapped.Slug
		}
		if e.Allocations != nil {
			mappedAllocations := map[rules.Slug]int{}
			for id, n := range e.Allocations {
				if mapped, ok := mappings.Entities[rules.NewRef(rules.RefResource, id)]; ok {
					id = mapped.Slug
				}
				mappedAllocations[id] += n
			}
			e.Allocations = mappedAllocations
		}
		for j := range e.Changes {
			if mapped, ok := mappings.Paths[e.Changes[j].Path]; ok {
				e.Changes[j].Path = mapped
			}
		}
		for j := range e.Choices {
			a := &e.Choices[j]
			if mapped, ok := mappings.Prompts[a.Prompt]; ok {
				a.Prompt = mapped
			}
			for k, pick := range a.Picks {
				if mapped, ok := mappings.Options[pick]; ok {
					a.Picks[k] = mapped
				}
			}
		}
	}
	working := domain.Log{}
	for _, event := range candidate.Events {
		staged := event
		staged.Seq = 0
		if err := validateAndAttribute(working, next, []domain.Event{staged}); err != nil {
			result.Issues = append(result.Issues, MigrationIssue{Seq: event.Seq, EventID: event.ID, Reason: "migration.invalidEvent"})
			break
		}
		if err := working.Append(staged); err != nil {
			return Migration{}, err
		}
	}
	if len(result.Issues) > 0 {
		if commit {
			return result, types.NewValidationError("migration has unresolved events")
		}
		return result, nil
	}
	after, err := domain.Project(candidate, next)
	if err != nil {
		return Migration{}, err
	}
	result.After = after
	result.Changed = changedSections(before, after)
	if commit {
		cp := domain.Checkpoint{Revision: c.Revision, Log: c.Log.Clone(), Reason: "rules-migration"}
		if err := s.repo.Commit(ctx, id, c.Revision, candidate, commandID(ctx), &cp); err != nil {
			return Migration{}, err
		}
		result.Revision++
	}
	return result, nil
}

func changedSections(a, b domain.State) []string {
	av, bv := reflect.ValueOf(a), reflect.ValueOf(b)
	t := av.Type()
	var out []string
	for i := 0; i < av.NumField(); i++ {
		if !reflect.DeepEqual(av.Field(i).Interface(), bv.Field(i).Interface()) {
			out = append(out, t.Field(i).Name)
		}
	}
	slices.Sort(out)
	return out
}

// RestoreCheckpoint is explicit: edits after the selected checkpoint are
// retained as a new checkpoint, never silently destroyed by a rollback.
func (s *Service) RestoreCheckpoint(ctx context.Context, owner domain.OwnerID, id domain.ID, locale rules.Locale, expectedRevision, index int, commit bool) (Migration, error) {
	c, cat, err := s.load(ctx, owner, id, locale)
	if err != nil {
		return Migration{}, err
	}
	if c.Revision != expectedRevision {
		return Migration{}, types.NewValidationError("stale rollback revision")
	}
	if index < 0 || index >= len(c.Checkpoints) {
		return Migration{}, types.NewValidationError("unknown checkpoint")
	}
	log := c.Checkpoints[index].Log.Clone()
	old, err := catalog.LoadLocked(ctx, s.catalog, locale, log.RulesLock())
	if err != nil {
		return Migration{}, err
	}
	before, err := domain.Project(c.Log, cat)
	if err != nil {
		return Migration{}, err
	}
	after, err := domain.Project(log, old)
	if err != nil {
		return Migration{}, err
	}
	result := Migration{Revision: c.Revision, Before: before, After: after, Changed: changedSections(before, after), Lock: log.RulesLock()}
	if commit {
		cp := domain.Checkpoint{Revision: c.Revision, Log: c.Log.Clone(), Reason: "rollback"}
		if err := s.repo.Commit(ctx, id, c.Revision, log, commandID(ctx), &cp); err != nil {
			return Migration{}, err
		}
		result.Revision += max(1, log.Len()-c.Log.Len())
	}
	return result, nil
}
