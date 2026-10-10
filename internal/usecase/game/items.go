package game

import (
	"context"
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/character"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// This file is the one place in the codebase where somebody other than its
// owner writes to a character, and it writes exactly three things: a count in
// the backpack, a coin in the purse, and one new custom item. A table hands things over -- the DM
// gives out treasure, one player passes another a potion -- and a rule that
// made the recipient type it in themselves would be a rule nobody follows.
//
// What grants the write is the game: the recipient is seated in it, and the
// actor either runs its table or is giving up something of their own.
// character.Service.owned is still untouched; nothing here edits a build.

// maxGrant bounds one hand-over. It is a typo guard, not a rule of the game.
const maxGrant = 100

// changeCharacter appends one change event to a character's log, whoever owns
// it. decide is handed the sheet as it stands and answers with absolute
// writes, so the event reads the same as one the owner's sheet would send.
//
// ponytail: a write by the owner between the read and the commit fails the
// revision check and surfaces as a 400; retry here if that is ever seen in play.
func (s *Service) changeCharacter(
	ctx context.Context, id character.ID,
	decide func(character.State, *catalog.Catalog) ([]character.Change, error),
) error {
	c, err := s.characters.Get(ctx, id)
	if err != nil {
		return err
	}
	cat, err := catalog.LoadLocked(ctx, s.catalog, rules.DefaultLocale, c.Log.RulesLock())
	if err != nil {
		return err
	}
	// A character's own items are part of its rules: one it was given can be given on.
	cat = character.WithCustomCatalog(c.Log, cat)
	state, err := character.Project(c.Log, cat)
	if err != nil {
		return err
	}
	changes, err := decide(state, cat)
	if err != nil {
		return err
	}
	working := c.Log.Clone()
	if err := working.Append(character.Event{Type: character.EventChange, At: s.now(), Changes: changes}); err != nil {
		return err
	}
	if _, err := character.Project(working, cat); err != nil {
		return err
	}
	return s.characters.Commit(ctx, id, c.Revision, working)
}

// seat finds one entry of a game's roster.
func (s *Service) seat(ctx context.Context, id domain.ID, entryID string) (domain.Entry, error) {
	roster, err := s.games.Characters(ctx, id)
	if err != nil {
		return domain.Entry{}, err
	}
	index := slices.IndexFunc(roster, func(e domain.Entry) bool { return e.ID == entryID })
	if index < 0 {
		return domain.Entry{}, types.NewNotFoundError("entry not found")
	}
	if roster[index].Kind != "player" {
		return domain.Entry{}, types.NewValidationError("only player entries carry items")
	}
	return roster[index], nil
}

func held(stacks []character.ItemStack, slug rules.Slug) int {
	n := 0
	for _, stack := range stacks {
		if stack.Item == slug {
			n += stack.Count
		}
	}
	return n
}

func setCount(list string, slug rules.Slug, n int) character.Change {
	return character.Change{Path: character.Path("equipment." + list + "." + string(slug)), Op: character.OpSet, Value: character.IntValue(n)}
}

// receive puts count more of an item in a backpack, or refuses an item the
// recipient's own rule packs have never heard of.
func receive(slug rules.Slug, count int) func(character.State, *catalog.Catalog) ([]character.Change, error) {
	return func(state character.State, cat *catalog.Catalog) ([]character.Change, error) {
		if !cat.Items.Has(slug) && !cat.MagicItems.Has(slug) {
			return nil, types.NewValidationError("item %q is not in the recipient's catalogue", slug).Because("item.unknownToRecipient")
		}
		return []character.Change{setCount("backpack", slug, held(state.Equipment.Backpack, slug)+count)}, nil
	}
}

func checkCount(count int) error {
	if count < 1 || count > maxGrant {
		return types.NewValidationError("count must be between 1 and %d", maxGrant)
	}
	return nil
}

// GrantItem is the DM handing a seated character something new.
func (s *Service) GrantItem(ctx context.Context, actor user.ID, id domain.ID, entryID string, slug rules.Slug, count int) error {
	if _, err := s.dm(ctx, actor, id, "give out items"); err != nil {
		return err
	}
	if err := checkCount(count); err != nil {
		return err
	}
	entry, err := s.seat(ctx, id, entryID)
	if err != nil {
		return err
	}
	return s.changeCharacter(ctx, entry.Character, receive(slug, count))
}

// AdjustCoins is the DM paying a seated character, or charging them: delta is
// signed, and a purse never goes below nothing.
func (s *Service) AdjustCoins(ctx context.Context, actor user.ID, id domain.ID, entryID string, unit rules.CoinUnit, delta int) error {
	if _, err := s.dm(ctx, actor, id, "change a purse"); err != nil {
		return err
	}
	if unit == rules.CoinNone || delta == 0 || delta > 1_000_000 || delta < -1_000_000 {
		return types.NewValidationError("invalid coin amount")
	}
	entry, err := s.seat(ctx, id, entryID)
	if err != nil {
		return err
	}
	return s.changeCharacter(ctx, entry.Character, func(state character.State, _ *catalog.Catalog) ([]character.Change, error) {
		total := state.Equipment.Purse[unit] + delta
		if total < 0 {
			return nil, types.NewValidationError("purse holds less than that").Because("coins.notEnough")
		}
		return []character.Change{{Path: character.Path("equipment.purse." + unit.String()), Op: character.OpSet, Value: character.IntValue(total)}}, nil
	})
}

// GiveItem moves count of an item from the actor's own seated character to
// another one at the same game. What is worn stays worn: it is taken from the
// backpack, then from the loot.
func (s *Service) GiveItem(ctx context.Context, actor user.ID, id domain.ID, fromID, toID string, slug rules.Slug, count int) error {
	if _, _, err := s.at(ctx, actor, id); err != nil {
		return err
	}
	if err := checkCount(count); err != nil {
		return err
	}
	from, err := s.seat(ctx, id, fromID)
	if err != nil {
		return err
	}
	if from.Owner != actor {
		return types.NewAccessDeniedError("only its owner gives a character's items away")
	}
	to, err := s.seat(ctx, id, toID)
	if err != nil {
		return err
	}
	if to.Character == from.Character {
		return types.NewValidationError("a character cannot give to itself")
	}
	// Asked before anything is taken, so an item the recipient cannot hold
	// never leaves the giver.
	toCat, err := s.CharacterCatalog(ctx, to.Owner, to.Character, rules.DefaultLocale)
	if err != nil {
		return err
	}
	if _, err := receive(slug, count)(character.State{}, toCat); err != nil {
		return err
	}

	err = s.changeCharacter(ctx, from.Character, func(state character.State, _ *catalog.Catalog) ([]character.Change, error) {
		backpack, loot := held(state.Equipment.Backpack, slug), held(state.Equipment.Loot, slug)
		if backpack+loot < count {
			if held(state.Equipment.Equipped, slug) > 0 {
				return nil, types.NewValidationError("item %q is worn", slug).Because("item.worn")
			}
			return nil, types.NewValidationError("not carrying %d of %q", count, slug).Because("item.notCarried")
		}
		fromBackpack := min(backpack, count)
		changes := []character.Change{}
		if fromBackpack > 0 {
			changes = append(changes, setCount("backpack", slug, backpack-fromBackpack))
		}
		if count > fromBackpack {
			changes = append(changes, setCount("loot", slug, loot-(count-fromBackpack)))
		}
		return changes, nil
	})
	if err != nil {
		return err
	}
	// ponytail: two commits, not one transaction. A crash between them loses
	// the item and never duplicates it; a repository method that commits two
	// characters at once is the upgrade.
	if err := s.changeCharacter(ctx, to.Character, receive(slug, count)); err != nil {
		if back := s.changeCharacter(ctx, from.Character, receive(slug, count)); back != nil {
			s.log.ErrorContext(ctx, "item lost in a failed hand-over", "game", id, "item", slug, "count", count, "error", back)
		}
		return err
	}
	return nil
}

// GrantCustomItem is the DM handing a seated character something no catalogue
// holds: GrantItem for an item written on the spot. It appends one new
// definition, in the backpack, and takes no id -- so it can replace, move or
// remove nothing the owner has -- through the same UpsertCustom and the same
// limits the owner's own route is held to.
func (s *Service) GrantCustomItem(ctx context.Context, actor user.ID, id domain.ID, entryID string, locale rules.Locale, name, description string, item *character.CustomItem) error {
	if _, err := s.dm(ctx, actor, id, "give out items"); err != nil {
		return err
	}
	entry, err := s.seat(ctx, id, entryID)
	if err != nil {
		return err
	}
	c, err := s.characters.Get(ctx, entry.Character)
	if err != nil {
		return err
	}
	cat, err := catalog.LoadLocked(ctx, s.catalog, locale, c.Log.RulesLock())
	if err != nil {
		return err
	}
	cat = character.WithCustomCatalog(c.Log, cat)
	if item == nil {
		item = &character.CustomItem{}
	}
	log, err := charuc.UpsertCustom(c.Log, cat, character.CustomOption{Kind: "item", Name: name, Description: description, Placement: "backpack", Count: 1, Selected: true, Item: item})
	if err != nil {
		return err
	}
	if err = charuc.CheckSheet(c.Log, log, cat, s.limits); err != nil {
		return err
	}
	return s.characters.Commit(ctx, c.ID, c.Revision, log)
}
