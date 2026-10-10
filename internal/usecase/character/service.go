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
	"slices"
	"time"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	"github.com/promix1722/easydnd/internal/usecase/portrait"
)

// Service holds the character usecases. Every dependency arrives through the
// constructor; there are no package-level singletons.
type PackAccess interface {
	AuthorizeLock(context.Context, user.ID, pack.Lock, pack.Lock) error
	Default() pack.Lock
}

func (s *Service) SetPackAccess(access PackAccess) { s.packAccess = access }

// What the AI Wizard (internal/usecase/agent) needs of the service it writes
// characters through. It is a second writer of the same character, not a
// client of the builder's operations: it commits its own log at its own
// revision, loads the catalogue a character's rules lock names, and says what
// it did in the same log stream.

// Source is the catalogue source characters are built against.
func (s *Service) Source() catalog.Source { return s.catalog }

// Repository is the character store.
func (s *Service) Repository() domain.Repository { return s.repo }

// Logger is the service's logger.
func (s *Service) Logger() *slog.Logger { return s.log }

// PackAccess is the pack authorisation port, nil when packs are not gated.
func (s *Service) PackAccess() PackAccess { return s.packAccess }

type Service struct {
	packAccess PackAccess
	repo       domain.Repository
	folders    domain.FolderRepository
	catalog    catalog.Source
	sharing    domain.Sharing
	copyLinks  domain.CopyLinks
	log        *slog.Logger
	limits     types.Limits

	// clock is injected so that an import stamps a time a test can predict.
	// Nil means the real clock; see the Now method.
	clock func() time.Time
}

// NewService wires a Service over the given repositories and catalogue source.
//
// Characters and folders are two stores but one service, because two of this
// package's rules span both: every character is in a folder, and deleting a
// folder deletes the characters in it. A separate folder service would have to
// reach into this one to keep either of them, which is a dependency drawn to
// avoid a field.
//
// The sharing port may be nil: a build in which nothing can hold a reference
// to a character has nothing to tell when one is deleted.
func NewService(
	repo domain.Repository,
	folders domain.FolderRepository,
	source catalog.Source,
	sharing domain.Sharing,
	log *slog.Logger,
) *Service {
	return &Service{
		repo:    repo,
		folders: folders,
		catalog: source,
		sharing: sharing,
		log:     log,
		limits:  types.DefaultLimits,
	}
}

// now reads the clock, defaulting to the real one.
func (s *Service) Now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now().UTC()
}

// NewCharacter is the opening state a character is created with: a name, and
// an alignment if the player has one in mind.
//
// It used to carry the generation method and all six ability scores as well,
// and that was eight selections seeded into one log entry. A selection with
// no entry of its own is a selection the player cannot change, which is why
// creation stopped bundling: the scores are an ordinary open choice now,
// answered from the abilities tab as their own entry, and the method travels
// with them.
type NewCharacter struct {
	Rules     pack.Lock
	Name      string
	Image     string
	Alignment rules.Slug
}

// Create starts a new character for owner and seeds it with an init event.
//
// A zero folder means the owner's default, which is materialised here if this
// is their first character. Naming a folder somebody else owns is a 404, like
// every other reach for what is not yours.
func (s *Service) Create(
	ctx context.Context, owner domain.OwnerID, folder domain.FolderID, opening NewCharacter,
) (domain.Character, error) {
	if err := validateOpening(opening); err != nil {
		return domain.Character{}, err
	}
	if err := s.CheckCharacterLimit(ctx, owner); err != nil {
		return domain.Character{}, err
	}
	folder, err := s.ResolveFolder(ctx, owner, folder)
	if err != nil {
		return domain.Character{}, err
	}

	var cat *catalog.Catalog
	if !opening.Rules.IsZero() {
		if s.packAccess == nil {
			return domain.Character{}, types.NewAccessDeniedError("pack selection unavailable")
		}
		if err := s.packAccess.AuthorizeLock(ctx, user.ID(owner), opening.Rules, pack.Lock{}); err != nil {
			return domain.Character{}, err
		}
		cat, err = catalog.LoadLocked(ctx, s.catalog, rules.DefaultLocale, opening.Rules)
	} else {
		cat, err = s.catalog.Load(ctx, rules.DefaultLocale)
	}
	if err != nil {
		return domain.Character{}, err
	}
	event := InitEvent(opening)
	event.RulesLock = cat.Lock.Clone()
	log := domain.Log{}
	if err := log.Append(event); err != nil {
		return domain.Character{}, err
	}
	// One write, so a failure cannot leave a character with no log behind.
	return s.repo.CreateWithLog(ctx, owner, folder, log)
}

// validateOpening checks the name and optional portrait carried at creation.
func validateOpening(opening NewCharacter) error {
	if !portrait.Valid(opening.Image) {
		return types.NewFieldValidationError("invalid portrait", portrait.FieldError())
	}
	if opening.Name == "" {
		return types.NewFieldValidationError("the character could not be created",
			types.FieldError{
				Field: "name", Rule: "required", Reason: "field.character.name.required",
			})
	}
	return nil
}

// initEvent builds the opening event. Everything it carries is a change,
// because everything it carries is an input the rules derive from rather than
// a catalogue entry the character selected.
//
// Its source is identity without asking, and it is the one event for which
// that is a statement rather than a lookup: no prompt offers an init event to
// a character that already exists, because the way to change a name is to
// replace this entry.
func InitEvent(opening NewCharacter) domain.Event {
	changes := []domain.Change{
		{Path: "identity.name", Op: domain.OpSet, Value: domain.StringValue(opening.Name)},
	}
	if opening.Image != "" {
		changes = append(changes, domain.Change{Path: "identity.image", Op: domain.OpSet, Value: domain.StringValue(opening.Image)})
	}
	if !opening.Alignment.IsZero() {
		changes = append(changes, domain.Change{
			Path: "identity.alignment", Op: domain.OpSet, Value: domain.SlugValue(opening.Alignment),
		})
	}
	return domain.Event{Type: domain.EventInit, Source: domain.GroupIdentity, Changes: changes}
}

// List returns summaries of the characters owned by owner.
//
// A zero folder lists all of them. A named one narrows to that folder, and must
// be one the caller owns -- otherwise an unowned id would list nothing and read
// as an empty folder rather than as somebody else's.
func (s *Service) List(
	ctx context.Context, owner domain.OwnerID, folder domain.FolderID, locale rules.Locale,
) ([]domain.Summary, error) {
	if !folder.IsZero() {
		if _, err := s.ownedFolder(ctx, owner, folder); err != nil {
			return nil, err
		}
	}
	characters, err := s.repo.List(ctx, owner)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Summary, 0, len(characters))
	for _, c := range characters {
		if !folder.IsZero() && c.Folder != folder {
			continue
		}
		locked, err := catalog.LoadLocked(ctx, s.catalog, locale, c.Log.RulesLock())
		if err != nil {
			return nil, err
		}
		out = append(out, domain.Summarize(c.ID, c.Owner, c.Folder, c.Log, locked))
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
		return domain.Character{}, types.NewNotFoundError("character %q", id).Because("character.notFound")
	}
	character.Log = normalizeImportLog(character.Log)
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
	state, _, err := s.SheetWithCatalog(ctx, owner, id, locale)
	return state, err
}

// SheetWithCatalog is Sheet plus the catalogue it was projected against, for a
// response that sends what the sheet's slugs mean along with them.
func (s *Service) SheetWithCatalog(
	ctx context.Context, owner domain.OwnerID, id domain.ID, locale rules.Locale,
) (domain.State, *catalog.Catalog, error) {
	character, cat, err := s.load(ctx, owner, id, locale)
	if err != nil {
		return domain.State{}, nil, err
	}
	state, err := domain.Project(character.Log, cat)
	return state, cat, err
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

// PromptsBefore reads the questions at an event's original position without
// changing the saved log. Replacement validation uses this same prefix.
func (s *Service) PromptsBefore(
	ctx context.Context, owner domain.OwnerID, id domain.ID, locale rules.Locale, before int,
) ([]domain.Prompt, error) {
	character, cat, err := s.load(ctx, owner, id, locale)
	if err != nil {
		return nil, err
	}
	if before < 2 || before > character.Log.LastSeq() {
		return nil, seqError("no editable event at this position", "field.seq.outOfRange")
	}
	prefix := domain.Log{Events: slices.Clone(character.Log.Events[:before-1])}
	context, err := spellEditContext(prefix, character.Log, cat, character.Log.Events[before-1])
	if err != nil {
		return nil, err
	}
	return domain.Prompts(context, cat)
}

// Apply validates events against the catalogue and appends them to a
// character's log, returning the sequence the log now ends at.
//
// Validation is the substance here: an event must select something the
// character is being offered, an answer must name a prompt they actually have
// open, and its picks must be options that prompt actually offers. Without
// those checks a typo is not an error but a silently missing proficiency,
// discovered weeks later as a wrong number on a sheet.
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
	if err := checkRevision(ctx, character); err != nil {
		return 0, err
	}
	if got := character.Log.LastSeq(); got != expectedSeq {
		return 0, types.NewValidationError(
			"character %q is at sequence %d, not %d", id, got, expectedSeq)
	}

	// Validating stamps each event with the source of the prompt it answers,
	// so the slice handed to the repository is not the slice that arrived.
	if err := ValidateAndAttribute(character.Log, cat, events); err != nil {
		return 0, err
	}
	working := character.Log.Clone()
	if err := working.Append(events...); err != nil {
		return 0, err
	}
	if err := CheckSheet(character.Log, working, cat, s.limits); err != nil {
		return 0, err
	}
	if err := s.repo.Commit(ctx, id, character.Revision, working); err != nil {
		return 0, err
	}
	return expectedSeq + len(events), nil
}

// Delete removes a character.
//
// It comes off every table it was shared with first, then out of the store.
// That ordering is chosen for what a crash leaves behind, the way
// DeleteFolder's is: stopping in the middle leaves a character nobody at a
// table can see any more but which its owner still has, and they can simply
// try again. The other order would leave rows naming a character that is gone,
// and though every read skips those, "the cleanup ran" is a worse thing to
// depend on than "the cleanup ran first".
//
// Your character is always yours to delete. Being at somebody's table does not
// make it theirs, so nothing here consults a group before agreeing.
func (s *Service) Delete(ctx context.Context, owner domain.OwnerID, id domain.ID) error {
	if _, err := s.owned(ctx, owner, id); err != nil {
		return err
	}
	if s.sharing != nil {
		if err := s.sharing.UnshareEverywhere(ctx, id); err != nil {
			return err
		}
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
	cat, err := catalog.LoadLocked(ctx, s.catalog, locale, character.Log.RulesLock())
	if err != nil {
		return domain.Character{}, nil, err
	}
	return character, domain.WithCustomCatalog(character.Log, cat), nil
}

func (s *Service) CharacterCatalog(ctx context.Context, owner domain.OwnerID, id domain.ID, locale rules.Locale) (*catalog.Catalog, error) {
	_, cat, err := s.load(ctx, owner, id, locale)
	return cat, err
}

type revisionKey struct{}

func WithRevision(ctx context.Context, revision int) context.Context {
	return context.WithValue(ctx, revisionKey{}, revision)
}
func checkRevision(ctx context.Context, c domain.Character) error {
	if expected, ok := ctx.Value(revisionKey{}).(int); ok && expected != c.Revision {
		return types.NewValidationError("stale character revision: got %d, expected %d", expected, c.Revision)
	}
	return nil
}

func (s *Service) View(ctx context.Context, owner domain.OwnerID, id domain.ID, locale rules.Locale) (domain.Character, domain.State, error) {
	c, cat, err := s.load(ctx, owner, id, locale)
	if err != nil {
		return c, domain.State{}, err
	}
	sheet, err := domain.Project(c.Log, cat)
	return c, sheet, err
}
