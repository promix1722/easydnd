package character

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// RemoveCustomOption deletes a note: the one kind of custom entry nothing can
// be built on. Every other kind may be the character's class or a spell it
// knows, and is switched off through its own Selected flag instead.
func (s *Service) RemoveCustomOption(ctx context.Context, owner domain.OwnerID, id domain.ID, locale rules.Locale, optionID string) (Revision, error) {
	character, cat, err := s.load(ctx, owner, id, locale)
	if err != nil {
		return Revision{}, err
	}
	if err = checkRevision(ctx, character); err != nil {
		return Revision{}, err
	}
	kept := make([]domain.Event, 0, character.Log.Len())
	for _, e := range character.Log.Events {
		if e.Custom == nil || e.Custom.ID != optionID {
			kept = append(kept, e)
			continue
		}
		if e.Custom.Kind != "note" {
			return Revision{}, types.NewValidationError("only a note can be deleted").Because("custom.notRemovable")
		}
	}
	if len(kept) == character.Log.Len() {
		return Revision{}, types.NewNotFoundError("custom option %q", optionID).Because("custom.notFound")
	}
	log, err := domain.Rebuild(kept)
	if err != nil {
		return Revision{}, err
	}
	if err = ValidateImported(cat, log); err != nil {
		return Revision{}, err
	}
	sheet, err := domain.Project(log, cat)
	if err != nil {
		return Revision{}, err
	}
	if err = s.repo.Commit(ctx, id, character.Revision, log, commandID(ctx), nil); err != nil {
		return Revision{}, err
	}
	return Revision{Revision: character.Revision + 1, Seq: log.LastSeq(), Sheet: sheet}, nil
}

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
	log, err := UpsertCustom(character.Log, cat, option)
	if err != nil {
		return Revision{}, err
	}
	if err = CheckSheet(character.Log, log, cat, s.limits); err != nil {
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
func UpsertCustom(log domain.Log, cat *catalog.Catalog, option domain.CustomOption) (domain.Log, error) {
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
	if option.Level != nil && (option.Kind == "spell" && *option.Level > 9 || option.Kind == "class" && (*option.Level < 1 || *option.Level > MaxLevel(cat))) {
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
	if err := checkCustomItem(cat, option); err != nil {
		return log, err
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
		for _, c := range CatalogCandidates(cat, expected) {
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
		// A writer that knows nothing of item mechanics -- the AI Wizard
		// renaming what it imported -- does not erase what a person filled in.
		if existing.ID == option.ID && option.Item == nil {
			option.Item = existing.Item
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
	if err := ValidateImported(cat, updated); err != nil {
		return log, err
	}
	return updated, nil
}

// checkCustomItem bounds a custom item's mechanics and checks every word in
// them against the character's own rules. The owner's route, the DM's and the
// AI Wizard all write through UpsertCustom, so this is the one place it is
// decided.
func checkCustomItem(cat *catalog.Catalog, option domain.CustomOption) error {
	it := option.Item
	if it == nil {
		return nil
	}
	bad := func(field string) error {
		return types.NewFieldValidationError("invalid custom item", types.FieldError{Field: "item." + field, Rule: "invalid", Reason: "custom.item.invalid"})
	}
	within := func(n, most int) bool { return n >= 0 && n <= most }
	if option.Kind != "item" {
		return bad("kind")
	}
	if it.Weapon != nil && it.Armor != nil {
		return bad("armor")
	}
	if _, ok := cat.ItemIcons[it.Icon]; it.Icon != "" && !ok {
		return bad("icon")
	}
	if it.Category != "" && !cat.EquipmentCategories.Has(it.Category) {
		return bad("category")
	}
	if it.Slot > catalog.SlotOffHand {
		return bad("slot")
	}
	if it.Weight < 0 || it.Weight > 100000 {
		return bad("weight")
	}
	if !within(it.Cost.Amount, 1e9) || it.Cost.Unit > rules.Platinum || (it.Cost.Amount > 0 && it.Cost.Unit == rules.CoinNone) {
		return bad("cost")
	}
	if a := it.Armor; a != nil {
		if a.Category == catalog.ArmorCategoryNone || a.Category > catalog.Shield {
			return bad("armor.category")
		}
		if !within(a.BaseAC, 30) {
			return bad("armor.baseAC")
		}
		if a.MaxDexBonus != nil && (!a.AddsDexBonus || !within(*a.MaxDexBonus, 10)) {
			return bad("armor.maxDexBonus")
		}
		if !within(a.StrengthMinimum, 30) {
			return bad("armor.strengthMinimum")
		}
	}
	if w := it.Weapon; w != nil {
		if w.Category == catalog.WeaponCategoryNone || w.Category > catalog.MartialWeapon {
			return bad("weapon.category")
		}
		if w.Range == catalog.WeaponRangeNone || w.Range > catalog.RangedWeapon {
			return bad("weapon.range")
		}
		for field, damage := range map[string]*rules.Damage{"weapon.damage": w.Damage, "weapon.twoHandedDamage": w.TwoHandedDamage} {
			if damage == nil {
				continue
			}
			d := damage.Dice
			if len(d.Terms) > 4 || d.Bonus < -100 || d.Bonus > 100 || d.PlusAbility || (damage.Type != "" && !cat.DamageTypes.Has(damage.Type)) {
				return bad(field)
			}
			for _, term := range d.Terms {
				if term.Count < 1 || term.Count > 100 || term.Faces < 1 || term.Faces > 100 {
					return bad(field)
				}
			}
		}
		for _, feet := range []rules.Feet{w.NormalRange, w.LongRange, w.ThrowNormal, w.ThrowLong} {
			if !within(int(feet), 10000) {
				return bad("weapon.normalRange")
			}
		}
		if len(w.Properties) > 20 {
			return bad("weapon.properties")
		}
		for _, property := range w.Properties {
			if !cat.WeaponProperties.Has(property) {
				return bad("weapon.properties")
			}
		}
	}
	return nil
}

// Candidates describe identity only. A separate choice validation determines
// whether this character may select the identified entry for a given purpose.
type Candidate struct {
	Ref     string  `json:"ref"`
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
	Level   *int    `json:"level,omitempty"`
	Details any     `json:"details,omitempty"`
}

// CatalogCandidates lists every entry of one kind in the selected rules.
func CatalogCandidates(cat *catalog.Catalog, kind string) []Candidate {
	fields := map[string]string{"race": "Races", "subrace": "Subraces", "class": "Classes", "subclass": "Subclasses", "background": "Backgrounds", "feat": "Feats", "feature": "Features", "trait": "Traits", "spell": "Spells", "item": "Items", "magic-item": "MagicItems", "language": "Languages", "skill": "Skills", "proficiency": "Proficiencies", "alignment": "Alignments"}
	field, ok := fields[kind]
	if !ok {
		return nil
	}
	rk, ok := rules.ParseRefKind(kind)
	if !ok {
		return nil
	}
	collection := reflect.ValueOf(cat).Elem().FieldByName(field)
	if !collection.IsValid() {
		return nil
	}
	entries := collection.MethodByName("All").Call(nil)[0]
	out := []Candidate{}
	for i := 0; i < entries.Len(); i++ {
		v := entries.Index(i)
		entry := v.FieldByName("Entry").Interface().(catalog.Entry)
		c := Candidate{Ref: rules.NewRef(rk, entry.Slug).Canonical(), Name: entry.Name, Details: v.Interface()}
		if l := v.FieldByName("Level"); l.IsValid() && l.Kind() == reflect.Int {
			n := int(l.Int())
			c.Level = &n
		}
		out = append(out, c)
	}
	return out
}
