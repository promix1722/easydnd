package game

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
	"github.com/promix1722/easydnd/internal/domain/character"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
	gameuc "github.com/promix1722/easydnd/internal/usecase/game"
)

// GameEntry deliberately separates public identity from private tracker values.
type GameEntry struct {
	ID          string      `json:"id"`
	Kind        string      `json:"kind"`
	Name        string      `json:"name"`
	CharacterID string      `json:"character_id,omitempty"`
	CanEdit     bool        `json:"can_edit"`
	Locked      *bool       `json:"locked,omitempty"`
	HP          *int        `json:"hp,omitempty"`
	TempHP      *int        `json:"temp_hp,omitempty"`
	Initiative  *int        `json:"initiative,omitempty"`
	Tags        *[]string   `json:"tags,omitempty"`
	Stats       *EntryStats `json:"stats,omitempty"`
	Resources   []EntryPool `json:"resources,omitempty"`
}

// EntryPool is one spendable resource; used is this game's count, not the sheet's.
type EntryPool struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Group     string `json:"group,omitempty"`
	Max       int    `json:"max"`
	Used      int    `json:"used"`
	Dice      string `json:"dice,omitempty"`
	SlotLevel int    `json:"slot_level,omitempty"`
}

type EntryStats struct {
	Name         string                      `json:"name"`
	MaxHP        int                         `json:"max_hp"`
	ArmorClass   int                         `json:"armor_class"`
	Spellcasting []characterapi.Spellcasting `json:"spellcasting"`
	Speeds       []characterapi.Speed        `json:"speeds"`
	Senses       []characterapi.Sense        `json:"senses"`
	Abilities    characterapi.Abilities      `json:"abilities"`
}

// EntryStatsPatch accepts only edited base fields; derived modifiers are ignored.
type EntryStatsPatch struct {
	Name         *string                      `json:"name"`
	MaxHP        *int                         `json:"max_hp"`
	ArmorClass   *int                         `json:"armor_class"`
	Spellcasting *[]characterapi.Spellcasting `json:"spellcasting"`
	Speeds       *[]characterapi.Speed        `json:"speeds"`
	Senses       *[]characterapi.Sense        `json:"senses"`
	Abilities    *characterapi.Abilities      `json:"abilities"`
}

func (patch EntryStatsPatch) domain() (*gameuc.StatsPatch, error) {
	stats := EntryStats{}
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
	converted, err := stats.domain()
	if err != nil {
		return nil, err
	}
	out := &gameuc.StatsPatch{Name: patch.Name, MaxHP: patch.MaxHP, ArmorClass: patch.ArmorClass}
	if patch.Spellcasting != nil {
		out.Spellcasting = &converted.Spellcasting
	}
	if patch.Speeds != nil {
		out.Speeds = &converted.Speeds
	}
	if patch.Senses != nil {
		out.Senses = &converted.Senses
	}
	if patch.Abilities != nil {
		out.Abilities = &converted.Abilities
	}
	return out, nil
}

func entryOf(p gameuc.Participant, master bool) GameEntry {
	e := p.Entry
	out := GameEntry{ID: e.ID, Kind: e.Kind, Name: p.Stats.Name, CanEdit: p.CanEdit}
	if e.Kind == "monster" && !master {
		return out
	}
	sheet := characterapi.SheetOf(character.State{Base: character.Base{Speeds: p.Stats.Speeds, Senses: p.Stats.Senses},
		Status: character.Status{Spellcasting: p.Stats.Spellcasting}, Abilities: p.Stats.Abilities})
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	out.HP, out.TempHP, out.Initiative, out.Tags = &e.HP, &e.TempHP, e.Initiative, &tags
	out.Stats = &EntryStats{Name: p.Stats.Name, MaxHP: p.Stats.MaxHP, ArmorClass: p.Stats.ArmorClass,
		Spellcasting: sheet.Status.Spellcasting, Speeds: sheet.Base.Speeds, Senses: sheet.Base.Senses, Abilities: sheet.Abilities}
	for _, pool := range p.Pools {
		out.Resources = append(out.Resources, EntryPool{ID: string(pool.ID), Name: pool.Name, Group: pool.Group,
			Max: pool.Max, Used: pool.Used, Dice: pool.Dice, SlotLevel: pool.SlotLevel})
	}
	if e.Kind == "player" {
		out.CharacterID = string(e.Character)
		out.Locked = &e.Locked
	}
	return out
}

func (stats EntryStats) domain() (domain.Stats, error) {
	out := domain.Stats{Name: stats.Name, MaxHP: stats.MaxHP, ArmorClass: stats.ArmorClass, Abilities: character.Abilities{Scores: make(map[rules.Ability]int)}}
	for key, score := range stats.Abilities.Scores {
		ability, ok := rules.ParseAbility(key)
		if !ok {
			return out, types.NewValidationError("unknown ability")
		}
		out.Abilities.Scores[ability] = score
	}
	for _, speed := range stats.Speeds {
		kinds := map[string]character.SpeedKind{"walking": character.Walking, "flying": character.Flying, "climbing": character.Climbing, "swimming": character.Swimming, "burrowing": character.Burrowing}
		kind, ok := kinds[speed.Kind]
		if !ok {
			return out, types.NewValidationError("unknown speed")
		}
		out.Speeds = append(out.Speeds, character.Speed{Kind: kind, Distance: rules.Feet(speed.Distance)})
	}
	for _, sense := range stats.Senses {
		kinds := map[string]character.SenseKind{"darkvision": character.Darkvision, "blindsight": character.Blindsight, "tremorsense": character.Tremorsense, "truesight": character.Truesight}
		kind, ok := kinds[sense.Kind]
		if !ok {
			return out, types.NewValidationError("unknown sense")
		}
		out.Senses = append(out.Senses, character.Sense{Kind: kind, Distance: rules.Feet(sense.Distance)})
	}
	for _, casting := range stats.Spellcasting {
		ability, _ := rules.ParseAbility(casting.Ability)
		out.Spellcasting = append(out.Spellcasting, character.SpellcastingSummary{Class: rules.Slug(casting.Class), Ability: ability, SaveDC: casting.SaveDC, AttackBonus: casting.AttackBonus})
	}
	return out, nil
}

func (h *Handler) PatchEntry(c *gin.Context) {
	var params struct {
		HP         *int             `json:"hp"`
		TempHP     *int             `json:"temp_hp"`
		Initiative json.RawMessage  `json:"initiative"`
		Tags       *[]string        `json:"tags"`
		Locked     *bool            `json:"locked"`
		Stats      *EntryStatsPatch `json:"stats"`
		Used       map[string]int   `json:"used"`
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&params); err != nil {
		helpers.FormatError(c, types.NewValidationError("invalid entry patch"))
		return
	}
	patch := gameuc.EntryPatch{HP: params.HP, TempHP: params.TempHP, Tags: params.Tags, Locked: params.Locked, Used: params.Used, InitiativeSet: len(params.Initiative) > 0}
	if patch.InitiativeSet {
		if err := json.Unmarshal(params.Initiative, &patch.Initiative); err != nil {
			helpers.FormatError(c, types.NewValidationError("invalid initiative"))
			return
		}
	}
	if params.Stats != nil {
		stats, err := params.Stats.domain()
		if err != nil {
			helpers.FormatError(c, err)
			return
		}
		patch.Stats = stats
	}
	if err := h.service.PatchEntry(c.Request.Context(), h.actor(c), gameOf(c), c.Param("entry"), patch); err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.detail(c, c.Request.Context(), h.actor(c), gameOf(c), http.StatusOK)
}

func (h *Handler) LongRest(c *gin.Context) {
	if err := h.service.LongRest(c.Request.Context(), h.actor(c), gameOf(c)); err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.detail(c, c.Request.Context(), h.actor(c), gameOf(c), http.StatusOK)
}

func (h *Handler) DeleteEntry(c *gin.Context) {
	if err := h.service.DeleteEntry(c.Request.Context(), h.actor(c), gameOf(c), c.Param("entry")); err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.detail(c, c.Request.Context(), h.actor(c), gameOf(c), http.StatusOK)
}

func (h *Handler) AddMonster(c *gin.Context) {
	var params struct {
		CharacterID string `json:"character_id"`
	}
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}
	if err := h.service.AddMonster(c.Request.Context(), h.actor(c), gameOf(c), character.ID(params.CharacterID), helpers.Locale(c)); err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.detail(c, c.Request.Context(), h.actor(c), gameOf(c), http.StatusCreated)
}

func (h *Handler) OrderEntries(c *gin.Context) {
	var params struct {
		EntryID      string  `json:"entry_id"`
		Direction    int     `json:"direction"`
		ByInitiative bool    `json:"by_initiative"`
		BeforeID     *string `json:"before_id"`
	}
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}
	var err error
	if params.BeforeID != nil {
		if params.ByInitiative || params.Direction != 0 {
			helpers.FormatError(c, types.NewValidationError("choose one ordering operation"))
			return
		}
		err = h.service.MoveEntryBefore(c.Request.Context(), h.actor(c), gameOf(c), params.EntryID, *params.BeforeID)
	} else {
		err = h.service.OrderEntries(c.Request.Context(), h.actor(c), gameOf(c), params.EntryID, params.Direction, params.ByInitiative)
	}
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.detail(c, c.Request.Context(), h.actor(c), gameOf(c), http.StatusOK)
}
