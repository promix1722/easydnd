package game

import (
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
}

func statsOf(state character.State) domain.Stats {
	return domain.Stats{Name: state.Identity.Name, MaxHP: state.Base.HitPoints.Max,
		ArmorClass: state.Status.ArmorClass, Spellcasting: state.Status.Spellcasting,
		Speeds: state.Base.Speeds, Senses: state.Base.Senses, Abilities: state.Abilities}
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
		for _, entry := range entries {
			if !slices.ContainsFunc(roster, func(e domain.Entry) bool { return e.Kind == "player" && e.Character == entry.Character }) {
				roster = append(roster, entry)
			}
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
		if e.Kind == "monster" {
			stats = *e.Monster
			if !role.AtLeast(group.RoleDM) {
				out = append(out, Participant{Entry: domain.Entry{ID: e.ID, Kind: "monster"}, Stats: &domain.Stats{Name: stats.Name}})
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
			stats = statsOf(state)
		}
		out = append(out, Participant{Entry: e, Stats: &stats, CanEdit: role.AtLeast(group.RoleDM) || (e.Kind == "player" && e.Owner == actor && !e.Locked)})
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
	return s.games.MutateEntries(ctx, id, func(entries []domain.Entry) ([]domain.Entry, error) { return append(entries, entry), nil })
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
