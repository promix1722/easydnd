package character

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

var customID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)

// UpsertCustomOption shares the same scoped, revision-checked operation between
// ordinary creation/editing and AI drafts. Definitions live in the log.
func (s *Service) UpsertCustomOption(ctx context.Context, owner domain.OwnerID, id domain.ID, locale rules.Locale, option domain.CustomOption) (Revision, error) {
	character, cat, err := s.load(ctx, owner, id, locale)
	if err != nil {
		return Revision{}, err
	}
	if err = checkRevision(ctx, character); err != nil {
		return Revision{}, err
	}
	log, err := upsertCustom(character.Log, cat, option)
	if err != nil {
		return Revision{}, err
	}
	sheet, err := domain.Project(log, cat)
	if err != nil {
		return Revision{}, err
	}
	if err = s.repo.Commit(ctx, id, character.Revision, log, commandID(ctx), nil); err != nil {
		return Revision{}, err
	}
	return Revision{Revision: character.Revision + max(1, log.Len()-character.Log.Len()), Seq: log.LastSeq(), Sheet: sheet}, nil
}
func upsertCustom(log domain.Log, cat *catalog.Catalog, option domain.CustomOption) (domain.Log, error) {
	if option.ID == "" {
		var id [12]byte
		if _, err := rand.Read(id[:]); err != nil {
			return log, err
		}
		option.ID = hex.EncodeToString(id[:])
	}
	option.Name = strings.TrimSpace(option.Name)
	if !customID.MatchString(option.ID) || option.Name == "" || len(option.Name) > 300 || len(option.Description) > 16000 || len(option.Source) > 1000 {
		return log, types.NewValidationError("custom option requires an id of 1-100 letters, digits, underscores or hyphens; a name of 1-300 characters; description <=16000 and source <=1000 characters")
	}
	switch option.Kind {
	case "class", "race", "subrace", "background", "subclass", "spell", "cantrip", "item", "feat", "feature", "trait", "note":
	default:
		return log, types.NewValidationError("unsupported custom option kind")
	}
	for _, value := range []*int{option.Level, option.HitDie, option.Speed} {
		if value != nil && (*value < 0 || *value > 1000) {
			return log, types.NewValidationError("invalid custom value")
		}
	}
	if option.Level != nil && (option.Kind == "spell" && *option.Level > 9 || option.Kind == "class" && (*option.Level < 1 || *option.Level > maxLevel(cat))) {
		return log, types.NewValidationError("invalid custom level")
	}
	if option.HitDie != nil && *option.HitDie != 4 && *option.HitDie != 6 && *option.HitDie != 8 && *option.HitDie != 10 && *option.HitDie != 12 {
		return log, types.NewValidationError("invalid hit die")
	}
	if option.Count < 0 || option.Count > 100000 {
		return log, types.NewValidationError("invalid quantity")
	}
	if option.Ability != "" {
		if _, ok := rules.ParseAbility(option.Ability); !ok {
			return log, types.NewValidationError("invalid spellcasting ability")
		}
	}
	if option.Mode != "" && option.Mode != "known" && option.Mode != "prepared" && option.Mode != "granted" && option.Mode != "spellbook" {
		return log, types.NewValidationError("invalid spell status")
	}
	if option.Placement != "" && option.Placement != "backpack" && option.Placement != "equipped" && option.Placement != "loot" {
		return log, types.NewValidationError("invalid inventory location")
	}
	// Older agent updates sometimes matched a custom option to itself.
	// It remains a definition, rather than a reference to a missing pack entry.
	if ref, ok := rules.ParseRef(option.Reference); ok && ref.Slug.String() == "custom-"+option.ID {
		option.Reference = ""
	}
	if option.Reference != "" {
		ref, ok := rules.ParseRef(option.Reference)
		if !ok {
			return log, types.NewValidationError("invalid custom reference")
		}
		expected := option.Kind
		if expected == "cantrip" {
			expected = "spell"
		}
		if ref.Kind.String() != expected {
			return log, types.NewValidationError("custom reference kind mismatch")
		}
		valid := false
		for _, c := range catalogCandidates(cat, expected) {
			if c.Ref == ref.Canonical() {
				valid = true
			}
		}
		if !valid {
			return log, types.NewValidationError("unknown custom reference")
		}
		// Catalogue mechanics stay intact; the explicit custom selection records provenance.
		option.Reference = ref.Canonical()
	}
	state, err := domain.Project(log, cat)
	if err != nil {
		return log, err
	}
	if ref, ok := rules.ParseRef(option.Parent); ok {
		option.Parent = ref.Slug.String()
	}
	if option.Kind == "subclass" || option.Kind == "feature" {
		if option.Parent == "" && len(state.Identity.Classes) == 1 {
			option.Parent = state.Identity.Classes[0].Class.String()
		}
		if option.Parent != "" && !cat.Classes.Has(rules.Slug(option.Parent)) {
			return log, types.NewValidationError("unknown parent class")
		}
		if option.Kind == "subclass" && option.Parent == "" {
			return log, types.NewValidationError("select a parent class first")
		}
	}
	if option.Kind == "subrace" {
		if option.Parent == "" {
			option.Parent = state.Identity.Race.String()
		}
		if !cat.Races.Has(rules.Slug(option.Parent)) {
			return log, types.NewValidationError("select a parent race first")
		}
	}
	for _, existing := range domain.CustomOptions(log) {
		if existing.ID == option.ID && existing.Kind != option.Kind {
			return log, types.NewValidationError("custom kind cannot change")
		}
	}
	updated := log.Clone()
	note := fmt.Sprintf("import.manual:%s\n%s: %s\n%s\n%s", option.ID, option.Kind, option.Name, option.Description, option.Source)
	replaced := false
	for i, e := range updated.Events {
		if (e.Custom != nil && e.Custom.ID == option.ID) || (e.Type == domain.EventNote && strings.HasPrefix(e.Note, "import.manual:"+option.ID+"\n")) {
			updated.Events[i].Custom = &option
			updated.Events[i].Note = note
			replaced = true
			break
		}
	}
	if !replaced {
		_ = updated.Append(domain.Event{Type: domain.EventNote, Custom: &option, Note: note, At: time.Now().UTC()})
	}
	if err := validateImported(cat, updated); err != nil {
		return log, err
	}
	return updated, nil
}
