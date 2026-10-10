package game

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/character"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// Participant is the caller-filtered compact view. A monster's private fields
// are absent for players, before anything crosses the transport boundary.
type Participant struct {
	Entry   domain.Entry
	Stats   *domain.Stats
	CanEdit bool
	// Pools are a player's spendable resources with this game's spent count.
	Pools []character.ResourcePool
}

func statsOf(state character.State) domain.Stats {
	stats := domain.Stats{Name: state.Identity.Name, Image: state.Identity.Image, MaxHP: state.Base.HitPoints.Max,
		ArmorClass: state.Status.ArmorClass, Spellcasting: state.Status.Spellcasting,
		Speeds: state.Base.Speeds, Senses: state.Base.Senses, Abilities: state.Abilities}
	if len(state.Identity.Classes) > 0 {
		stats.Class = state.Identity.Classes[0].Class
	}
	return stats
}

// scalingPrefix marks a pool that the sheet calls a scaling value.
const scalingPrefix = "scaling/"

// consumables is everything a game can spend from a sheet: its pools, and its
// plain-number scaling values -- Extra Attacks: 1, Maneuvers: 3 -- as pools of
// that many uses. A pack calls those parameters because nothing in the rules
// spends them, but a table still counts them off within a turn, and the
// tracker is where counting is done. A value that is a die, a fraction or a
// word has no number of uses and is left out.
//
// Nothing restores one but a long rest and the row's own plus button: a
// scaling value has no recovery policy to read.
func consumables(state character.State) map[rules.Slug]character.ResourcePool {
	out := make(map[rules.Slug]character.ResourcePool, len(state.Resources.Pools)+len(state.Resources.Parameters))
	for id, pool := range state.Resources.Pools {
		out[id] = pool
	}
	for slug, value := range state.Resources.Parameters {
		if value.Number > 0 && value.Dice == "" && value.Text == "" && value.Rational == nil && value.Boolean == nil {
			id := scalingPrefix + slug
			out[id] = character.ResourcePool{ID: id, Definition: slug, Name: value.Name, Group: "scaling", Max: value.Number}
		}
	}
	return out
}

// poolsOf orders a character's non-empty pools -- spell slots by level, then
// named pools, scaling values, hit dice last -- and fills in what this game
// has spent.
func poolsOf(state character.State, used map[string]int) []character.ResourcePool {
	rank := func(p character.ResourcePool) int {
		switch p.Group {
		case "spell-slots":
			return 0
		case "scaling":
			return 2
		case "hit-dice":
			return 3
		}
		return 1
	}
	all := consumables(state)
	out := make([]character.ResourcePool, 0, len(all))
	for id, pool := range all {
		if pool.Max > 0 {
			pool.Used = min(used[string(id)], pool.Max)
			out = append(out, pool)
		}
	}
	slices.SortFunc(out, func(a, b character.ResourcePool) int {
		return cmp.Or(cmp.Compare(rank(a), rank(b)), cmp.Compare(a.SlotLevel, b.SlotLevel), cmp.Compare(a.ID, b.ID))
	})
	return out
}

func (s *Service) project(ctx context.Context, id character.ID, locale rules.Locale) (character.Character, character.State, error) {
	c, err := s.characters.Get(ctx, id)
	if err != nil {
		return c, character.State{}, err
	}
	cat, err := catalog.LoadLocked(ctx, s.catalog, locale, c.Log.RulesLock())
	if err != nil {
		return c, character.State{}, err
	}
	state, err := character.Project(c.Log, cat)
	return c, state, err
}

func (s *Service) seatEntries(ctx context.Context, id domain.ID, ids []character.ID) error {
	entries := make([]domain.Entry, 0, len(ids))
	for _, cid := range ids {
		c, state, err := s.project(ctx, cid, rules.DefaultLocale)
		if err != nil {
			return err
		}
		entries = append(entries, domain.Entry{ID: "pc_" + string(cid), Kind: "player", Character: cid,
			Owner: user.ID(c.Owner), AddedAt: s.now(), HP: state.Base.HitPoints.Current, TempHP: state.Base.HitPoints.Temporary})
	}
	return s.games.MutateEntries(ctx, id, func(roster []domain.Entry) ([]domain.Entry, error) {
		was := len(roster)
		for _, entry := range entries {
			if !slices.ContainsFunc(roster, func(e domain.Entry) bool { return e.Kind == "player" && e.Character == entry.Character }) {
				roster = append(roster, entry)
			}
		}
		if len(roster) > was && len(roster) > s.limits.GameEntries {
			return nil, types.LimitReached("gameEntries", s.limits.GameEntries)
		}
		return roster, nil
	})
}

// Participants reads live player base stats without touching their game values.
func (s *Service) Participants(ctx context.Context, actor user.ID, id domain.ID, locale rules.Locale) ([]Participant, error) {
	_, role, err := s.at(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	entries, err := s.games.Characters(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]Participant, 0, len(entries))
	for _, e := range entries {
		var stats domain.Stats
		var pools []character.ResourcePool
		if e.Kind == "monster" {
			stats = *e.Monster
			if !role.AtLeast(group.RoleDM) {
				out = append(out, Participant{Entry: domain.Entry{ID: e.ID, Kind: "monster"}, Stats: &domain.Stats{Name: stats.Name, Image: stats.Image, Class: stats.Class}})
				continue
			}
		} else {
			_, state, err := s.project(ctx, e.Character, locale)
			if types.IsNotFound(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			stats, pools = statsOf(state), poolsOf(state, e.Used)
		}
		out = append(out, Participant{Entry: e, Stats: &stats, Pools: pools, CanEdit: role.AtLeast(group.RoleDM) || (e.Kind == "player" && e.Owner == actor && !e.Locked)})
	}
	return out, nil
}

// EntryPatch is field-based so edits to independent values do not overwrite
// each other. InitiativeSet distinguishes an explicit clear from omission.
type EntryPatch struct {
	HP            *int
	TempHP        *int
	InitiativeSet bool
	Initiative    *int
	Tags          *[]string
	Locked        *bool
	Stats         *StatsPatch
	// Used sets the spent count of the named pools and leaves the others alone.
	Used map[string]int
}

// StatsPatch keeps independent monster base fields from overwriting each other.
type StatsPatch struct {
	Name         *string
	MaxHP        *int
	ArmorClass   *int
	Spellcasting *[]character.SpellcastingSummary
	Speeds       *[]character.Speed
	Senses       *[]character.Sense
	Abilities    *character.Abilities
}

func (patch StatsPatch) apply(stats *domain.Stats) {
	if patch.Name != nil {
		stats.Name = *patch.Name
	}
	if patch.MaxHP != nil {
		stats.MaxHP = *patch.MaxHP
	}
	if patch.ArmorClass != nil {
		stats.ArmorClass = *patch.ArmorClass
	}
	if patch.Spellcasting != nil {
		stats.Spellcasting = *patch.Spellcasting
	}
	if patch.Speeds != nil {
		stats.Speeds = *patch.Speeds
	}
	if patch.Senses != nil {
		stats.Senses = *patch.Senses
	}
	if patch.Abilities != nil {
		stats.Abilities = *patch.Abilities
	}
}

func validateStats(stats *domain.Stats) error {
	name, err := validateName(stats.Name)
	if err != nil {
		return err
	}
	stats.Name = name
	if stats.MaxHP < 0 || stats.ArmorClass < 0 {
		return types.NewValidationError("negative base stat")
	}
	for _, speed := range stats.Speeds {
		if speed.Kind < character.Walking || speed.Kind > character.Burrowing || speed.Distance < 0 {
			return types.NewValidationError("invalid speed")
		}
	}
	for _, sense := range stats.Senses {
		if sense.Kind < character.Darkvision || sense.Kind > character.Truesight || sense.Distance < 0 {
			return types.NewValidationError("invalid sense")
		}
	}
	for _, ability := range rules.Abilities() {
		if stats.Abilities.Score(ability) < 1 || stats.Abilities.Score(ability) > 30 {
			return types.NewValidationError("ability score outside 1..30")
		}
	}
	for _, casting := range stats.Spellcasting {
		if casting.SaveDC < 0 {
			return types.NewValidationError("negative spell DC")
		}
	}
	return nil
}

func (s *Service) PatchEntry(ctx context.Context, actor user.ID, id domain.ID, entryID string, patch EntryPatch) error {
	_, role, err := s.at(ctx, actor, id)
	if err != nil {
		return err
	}
	// Capacities come from the sheet, read before the roster lock is taken.
	var pools map[rules.Slug]character.ResourcePool
	if len(patch.Used) > 0 {
		roster, err := s.games.Characters(ctx, id)
		if err != nil {
			return err
		}
		index := slices.IndexFunc(roster, func(e domain.Entry) bool { return e.ID == entryID })
		if index < 0 {
			return types.NewNotFoundError("entry not found")
		}
		if roster[index].Kind != "player" {
			return types.NewValidationError("only player entries have resources")
		}
		_, state, err := s.project(ctx, roster[index].Character, rules.DefaultLocale)
		if err != nil {
			return err
		}
		pools = consumables(state)
	}
	return s.games.MutateEntries(ctx, id, func(entries []domain.Entry) ([]domain.Entry, error) {
		for i := range entries {
			e := &entries[i]
			if e.ID != entryID {
				continue
			}
			master := role.AtLeast(group.RoleDM)
			if !master && (e.Kind != "player" || e.Owner != actor || e.Locked || patch.Locked != nil || patch.Stats != nil) {
				return nil, types.NewAccessDeniedError("entry is not editable")
			}
			if patch.HP != nil {
				if *patch.HP < 0 {
					return nil, types.NewValidationError("negative HP")
				}
				e.HP = *patch.HP
			}
			if patch.TempHP != nil {
				if *patch.TempHP < 0 {
					return nil, types.NewValidationError("negative temporary HP")
				}
				e.TempHP = *patch.TempHP
			}
			if patch.InitiativeSet {
				e.Initiative = patch.Initiative
			}
			if patch.Tags != nil {
				if len(*patch.Tags) > 20 {
					return nil, types.NewValidationError("too many tags")
				}
				tags := make([]string, 0, len(*patch.Tags))
				for _, tag := range *patch.Tags {
					tag = strings.TrimSpace(tag)
					if tag == "" || !utf8.ValidString(tag) || utf8.RuneCountInString(tag) > 100 {
						return nil, types.NewValidationError("invalid tag")
					}
					if !slices.Contains(tags, tag) {
						tags = append(tags, tag)
					}
				}
				e.Tags = tags
			}
			for pool, n := range patch.Used {
				if n < 0 || n > pools[rules.Slug(pool)].Max {
					return nil, types.NewValidationError("resource %s has no such use", pool)
				}
				if e.Used == nil {
					e.Used = map[string]int{}
				}
				e.Used[pool] = n
			}
			if patch.Locked != nil {
				if e.Kind != "player" {
					return nil, types.NewValidationError("monsters cannot be unlocked")
				}
				e.Locked = *patch.Locked
			}
			if patch.Stats != nil {
				if e.Kind != "monster" {
					return nil, types.NewValidationError("player base stats follow their sheet")
				}
				patch.Stats.apply(e.Monster)
				if err := validateStats(e.Monster); err != nil {
					return nil, err
				}
			}
			return entries, nil
		}
		return nil, types.NewNotFoundError("entry not found")
	})
}

// AddMonster copies an owned character privately, or creates an editable stub.
func (s *Service) AddMonster(ctx context.Context, actor user.ID, id domain.ID, source character.ID, locale rules.Locale) error {
	if _, err := s.dm(ctx, actor, id, "add monsters"); err != nil {
		return err
	}
	stats := domain.Stats{Name: "NPC", MaxHP: 10, ArmorClass: 10, Speeds: []character.Speed{{Kind: character.Walking, Distance: 30}}}
	stats.Abilities.Scores = make(map[rules.Ability]int, len(rules.Abilities()))
	for _, ability := range rules.Abilities() {
		stats.Abilities.Scores[ability] = 10
	}
	hp, tempHP := 10, 0
	if source != "" {
		if _, err := s.owned(ctx, actor, source); err != nil {
			return err
		}
		_, state, err := s.project(ctx, source, locale)
		if err != nil {
			return err
		}
		stats = statsOf(state)
		hp, tempHP = state.Base.HitPoints.Current, state.Base.HitPoints.Temporary
	}
	eid, err := newGameID()
	if err != nil {
		return err
	}
	entry := domain.Entry{ID: "mon_" + strings.TrimPrefix(string(eid), gameIDPrefix), Kind: "monster", AddedAt: s.now(), HP: hp, TempHP: tempHP, Monster: &stats}
	return s.games.MutateEntries(ctx, id, func(entries []domain.Entry) ([]domain.Entry, error) {
		if len(entries) >= s.limits.GameEntries {
			return nil, types.LimitReached("gameEntries", s.limits.GameEntries)
		}
		return append(entries, entry), nil
	})
}

// Rest gives participants their spent uses back. A long rest returns
// everything -- Hit Dice included, where the rules would return half. A short
// rest returns only the pools whose own recovery says a short rest refills
// them, read from each player's sheet before the roster lock is taken.
func (s *Service) Rest(ctx context.Context, actor user.ID, id domain.ID, short bool) error {
	if _, err := s.dm(ctx, actor, id, "call a rest"); err != nil {
		return err
	}
	restored := map[string][]string{}
	if short {
		roster, err := s.games.Characters(ctx, id)
		if err != nil {
			return err
		}
		for _, e := range roster {
			if e.Kind != "player" || len(e.Used) == 0 {
				continue
			}
			_, state, err := s.project(ctx, e.Character, rules.DefaultLocale)
			if types.IsNotFound(err) {
				continue
			}
			if err != nil {
				return err
			}
			for pool := range e.Used {
				if state.Resources.Pools[rules.Slug(pool)].RestoredBy("short-rest") {
					restored[e.ID] = append(restored[e.ID], pool)
				}
			}
		}
	}
	return s.games.MutateEntries(ctx, id, func(entries []domain.Entry) ([]domain.Entry, error) {
		for i := range entries {
			if !short {
				entries[i].Used = nil
				continue
			}
			for _, pool := range restored[entries[i].ID] {
				delete(entries[i].Used, pool)
			}
		}
		return entries, nil
	})
}

func (s *Service) DeleteEntry(ctx context.Context, actor user.ID, id domain.ID, entryID string) error {
	if _, err := s.dm(ctx, actor, id, "remove entries"); err != nil {
		return err
	}
	return s.games.MutateEntries(ctx, id, func(entries []domain.Entry) ([]domain.Entry, error) {
		index := slices.IndexFunc(entries, func(e domain.Entry) bool { return e.ID == entryID })
		if index < 0 {
			return nil, types.NewNotFoundError("entry not found")
		}
		return slices.Delete(entries, index, index+1), nil
	})
}

// OrderEntries either moves one entry a single step or stably sorts all entries.
func (s *Service) OrderEntries(ctx context.Context, actor user.ID, id domain.ID, entryID string, direction int, byInitiative bool) error {
	if _, err := s.dm(ctx, actor, id, "order entries"); err != nil {
		return err
	}
	return s.games.MutateEntries(ctx, id, func(entries []domain.Entry) ([]domain.Entry, error) {
		if byInitiative {
			slices.SortStableFunc(entries, func(a, b domain.Entry) int {
				if a.Initiative == nil {
					if b.Initiative == nil {
						return 0
					}
					return 1
				}
				if b.Initiative == nil {
					return -1
				}
				if *a.Initiative > *b.Initiative {
					return -1
				}
				if *a.Initiative < *b.Initiative {
					return 1
				}
				return 0
			})
			return entries, nil
		}
		if direction != -1 && direction != 1 {
			return nil, types.NewValidationError("direction must be -1 or 1")
		}
		index := slices.IndexFunc(entries, func(e domain.Entry) bool { return e.ID == entryID })
		if index < 0 {
			return nil, types.NewNotFoundError("entry not found")
		}
		next := index + direction
		if next >= 0 && next < len(entries) {
			entries[index], entries[next] = entries[next], entries[index]
		}
		return entries, nil
	})
}

// MoveEntryBefore moves one entry relative to a stable ID, or to the end when
// beforeID is empty. It preserves entries added or reordered by another master.
func (s *Service) MoveEntryBefore(ctx context.Context, actor user.ID, id domain.ID, entryID, beforeID string) error {
	if _, err := s.dm(ctx, actor, id, "order entries"); err != nil {
		return err
	}
	return s.games.MutateEntries(ctx, id, func(entries []domain.Entry) ([]domain.Entry, error) {
		from := slices.IndexFunc(entries, func(e domain.Entry) bool { return e.ID == entryID })
		if from < 0 {
			return nil, types.NewNotFoundError("entry not found")
		}
		if entryID == beforeID {
			return entries, nil
		}
		to := len(entries)
		if beforeID != "" {
			to = slices.IndexFunc(entries, func(e domain.Entry) bool { return e.ID == beforeID })
			if to < 0 {
				return nil, types.NewNotFoundError("destination entry not found")
			}
		}
		moved := entries[from]
		entries = slices.Delete(entries, from, from+1)
		if from < to {
			to--
		}
		return slices.Insert(entries, to, moved), nil
	})
}
