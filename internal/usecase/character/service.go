// Package character implements the character application usecases.
//
// It depends on the character domain, the catalogue domain and
// internal/types, and on nothing else. In particular it never sees a
// *gin.Context: handlers pass a plain context.Context and plain arguments
// across this boundary, which is what keeps the HTTP framework out of the
// application layer.
package character

import (
	"context"
	"log/slog"
	"time"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// Service holds the character usecases. Every dependency arrives through the
// constructor; there are no package-level singletons.
type Service struct {
	repo     domain.Repository
	catalog  catalog.Source
	importer SheetImporter
	log      *slog.Logger

	// clock is injected so that an import stamps a time a test can predict.
	// Nil means the real clock; see the now method.
	clock func() time.Time
}

// NewService wires a Service over the given repository, catalogue source and
// sheet importer.
//
// The importer may be nil, in which case Import reports that the feature is
// not configured rather than panicking. That is not a convenience: a build
// that ships without an importer should fail the one route that needs one, not
// every route that does not.
func NewService(
	repo domain.Repository, source catalog.Source, importer SheetImporter, log *slog.Logger,
) *Service {
	return &Service{repo: repo, catalog: source, importer: importer, log: log}
}

// now reads the clock, defaulting to the real one.
func (s *Service) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now().UTC()
}

// NewCharacter is the opening state a character is created with.
//
// The scores are the *base* array as generated -- point buy, standard array,
// dice. Racial bonuses are applied by the projection, every time, which is
// what lets a player change their mind about a race without re-entering six
// numbers.
type NewCharacter struct {
	Name      string
	Alignment rules.Slug
	Method    rules.Slug
	Abilities map[rules.Ability]int
}

// minScore and maxScore bound an ability score.
//
// The bounds are deliberately wide. Point buy and the standard array are far
// narrower, but validating them here would make a legitimate DM ruling --
// a boon, a cursed item, a homebrew race -- impossible to record. The client
// enforces the generation method it is offering; the server enforces only
// what no rule can produce.
const (
	minScore = 1
	maxScore = 30
)

// Create starts a new character for owner and seeds it with an init event.
func (s *Service) Create(ctx context.Context, owner domain.OwnerID, opening NewCharacter) (domain.Character, error) {
	if err := validateOpening(opening); err != nil {
		return domain.Character{}, err
	}

	created, err := s.repo.Create(ctx, owner)
	if err != nil {
		return domain.Character{}, err
	}
	if err := s.repo.Append(ctx, created.ID, 0, initEvent(opening)); err != nil {
		return domain.Character{}, err
	}
	return s.repo.Get(ctx, created.ID)
}

func validateOpening(opening NewCharacter) error {
	var fields []types.FieldError
	if opening.Name == "" {
		fields = append(fields, types.FieldError{
			Field: "name", Rule: "required", Message: "a character needs a name",
		})
	}
	for ability, score := range opening.Abilities {
		if score < minScore || score > maxScore {
			fields = append(fields, types.FieldError{
				Field:   "abilities." + ability.Slug().String(),
				Rule:    "range",
				Message: "an ability score must be between 1 and 30",
			})
		}
	}
	if len(fields) > 0 {
		return types.NewFieldValidationError("the character could not be created", fields...)
	}
	return nil
}

// initEvent builds the opening event. Everything it carries is a change,
// because everything it carries is an input the rules derive from rather than
// a catalogue entry the character selected.
func initEvent(opening NewCharacter) domain.Event {
	changes := []domain.Change{
		{Path: "identity.name", Op: domain.OpSet, Value: domain.StringValue(opening.Name)},
	}
	if !opening.Alignment.IsZero() {
		changes = append(changes, domain.Change{
			Path: "identity.alignment", Op: domain.OpSet, Value: domain.SlugValue(opening.Alignment),
		})
	}
	if !opening.Method.IsZero() {
		changes = append(changes, domain.Change{
			Path: "abilities.method", Op: domain.OpSet, Value: domain.SlugValue(opening.Method),
		})
	}
	// Ordered, so that two identical requests produce identical logs.
	for _, ability := range rules.Abilities() {
		score, ok := opening.Abilities[ability]
		if !ok {
			continue
		}
		changes = append(changes, domain.Change{
			Path:  domain.Path("abilities." + ability.Slug().String()),
			Op:    domain.OpSet,
			Value: domain.IntValue(score),
		})
	}
	return domain.Event{Type: domain.EventInit, Changes: changes}
}

// List returns summaries of the characters owned by owner.
func (s *Service) List(ctx context.Context, owner domain.OwnerID, locale rules.Locale) ([]domain.Summary, error) {
	characters, err := s.repo.List(ctx, owner)
	if err != nil {
		return nil, err
	}
	cat, err := s.catalog.Load(ctx, locale)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Summary, 0, len(characters))
	for _, c := range characters {
		out = append(out, domain.Summarize(c.ID, c.Owner, c.Log, cat))
	}
	return out, nil
}

// Get returns a character and its log.
func (s *Service) Get(ctx context.Context, owner domain.OwnerID, id domain.ID) (domain.Character, error) {
	return s.owned(ctx, owner, id)
}

// owned fetches a character and refuses it to anyone but its owner.
//
// The refusal is a NotFoundError rather than an AccessDeniedError, and that is
// deliberate: 403 on somebody else's id confirms the id exists, which turns a
// guessable identifier into an enumeration oracle. To a caller who does not
// own it, a character is indistinguishable from one that was never created.
//
// Every read and write path goes through here rather than through the
// repository directly, because "the handler remembered to check" is not an
// invariant -- it is a habit, and habits are what a missing check looks like
// in review.
func (s *Service) owned(
	ctx context.Context, owner domain.OwnerID, id domain.ID,
) (domain.Character, error) {
	character, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Character{}, err
	}
	if character.Owner != owner {
		return domain.Character{}, types.NewNotFoundError("character %q", id)
	}
	return character, nil
}

// Sheet returns a character's projected state, with catalogue prose in the
// requested locale.
//
// This is the read path the whole event-sourced design exists to serve: fetch
// the log, load the catalogue for the locale, and fold one against the other.
func (s *Service) Sheet(
	ctx context.Context, owner domain.OwnerID, id domain.ID, locale rules.Locale,
) (domain.State, error) {
	character, cat, err := s.load(ctx, owner, id, locale)
	if err != nil {
		return domain.State{}, err
	}
	return domain.Project(character.Log, cat)
}

// Prompts returns what the character still has to decide.
func (s *Service) Prompts(
	ctx context.Context, owner domain.OwnerID, id domain.ID, locale rules.Locale,
) ([]domain.Prompt, error) {
	character, cat, err := s.load(ctx, owner, id, locale)
	if err != nil {
		return nil, err
	}
	return domain.Prompts(character.Log, cat)
}

// Apply validates events against the catalogue and appends them to a
// character's log, returning the sequence the log now ends at.
//
// Validation is the substance here: an answer must name a prompt the
// character actually has open, and its picks must be options that prompt
// actually offers. Without that check a typo is not an error but a silently
// missing proficiency, discovered weeks later as a wrong number on a sheet.
func (s *Service) Apply(
	ctx context.Context,
	owner domain.OwnerID,
	id domain.ID,
	locale rules.Locale,
	expectedSeq int,
	events ...domain.Event,
) (int, error) {
	if len(events) == 0 {
		return 0, types.NewValidationError("no events to apply")
	}
	character, cat, err := s.load(ctx, owner, id, locale)
	if err != nil {
		return 0, err
	}
	if got := character.Log.LastSeq(); got != expectedSeq {
		return 0, types.NewValidationError(
			"character %q is at sequence %d, not %d", id, got, expectedSeq)
	}

	if err := validateEvents(character.Log, cat, events); err != nil {
		return 0, err
	}
	if err := s.repo.Append(ctx, id, expectedSeq, events...); err != nil {
		return 0, err
	}
	return expectedSeq + len(events), nil
}

// Truncate drops every event after afterSeq: the build flow's Back button,
// and un-taking a level.
func (s *Service) Truncate(
	ctx context.Context, owner domain.OwnerID, id domain.ID, expectedSeq, afterSeq int,
) error {
	if _, err := s.owned(ctx, owner, id); err != nil {
		return err
	}
	return s.repo.Truncate(ctx, id, expectedSeq, afterSeq)
}

// Delete removes a character.
func (s *Service) Delete(ctx context.Context, owner domain.OwnerID, id domain.ID) error {
	if _, err := s.owned(ctx, owner, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Catalog returns the compendium for a locale.
//
// It is exposed because the prompt endpoint serves Choice values, and
// rendering an option's prose means resolving a key against the catalogue.
// The alternative -- having the domain's Prompt carry resolved text -- would
// put locale-dependent strings inside a package that is deliberately
// language-neutral.
func (s *Service) Catalog(ctx context.Context, locale rules.Locale) (*catalog.Catalog, error) {
	return s.catalog.Load(ctx, locale)
}

// load fetches a character and the catalogue for a locale together, since
// almost every read path needs both.
func (s *Service) load(
	ctx context.Context, owner domain.OwnerID, id domain.ID, locale rules.Locale,
) (domain.Character, *catalog.Catalog, error) {
	character, err := s.owned(ctx, owner, id)
	if err != nil {
		return domain.Character{}, nil, err
	}
	cat, err := s.catalog.Load(ctx, locale)
	if err != nil {
		return domain.Character{}, nil, err
	}
	return character, cat, nil
}
