package character

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

type CustomOption struct {
	Reference   string `json:"ref,omitempty"`
	ID          string `json:"id,omitempty"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Parent      string `json:"parent,omitempty"`
	Ability     string `json:"ability,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Placement   string `json:"placement,omitempty"`
	Level       *int   `json:"level,omitempty"`
	HitDie      *int   `json:"hitDie,omitempty"`
	Speed       *int   `json:"speed,omitempty"`
	Count       int    `json:"count,omitempty"`
	Selected    bool   `json:"selected"`
	// Item is a custom item's mechanics in the catalogue's own item shape,
	// except that icon is the pack label rather than the picture.
	Item *catalogapi.Item `json:"item,omitempty"`
}

// CustomItemOf reads the catalogue item shape a client writes a custom item
// in. A word outside a vocabulary is a field error naming the field.
func CustomItemOf(w *catalogapi.Item) (*domain.CustomItem, error) {
	if w == nil {
		return nil, nil
	}
	bad := func(field string) error {
		return types.NewFieldValidationError("invalid custom item", types.FieldError{Field: "item." + field, Rule: "invalid", Reason: "custom.item.invalid"})
	}
	damage := func(field string, d *catalogapi.Damage) (*rules.Damage, error) {
		if d == nil || d.Dice == "" {
			return nil, nil
		}
		dice, err := rules.ParseDice(d.Dice)
		if err != nil {
			return nil, bad(field)
		}
		return &rules.Damage{Dice: dice, Type: rules.Slug(d.Type)}, nil
	}
	out := &domain.CustomItem{Icon: w.Icon, Category: rules.Slug(w.Category), Weight: w.Weight}
	var ok bool
	if out.Slot, ok = catalog.ParseSlot(w.Slot); !ok {
		return nil, bad("slot")
	}
	if w.Cost != nil && w.Cost.Amount != 0 {
		unit, err := rules.ParseCoinUnit(w.Cost.Unit)
		if err != nil {
			return nil, bad("cost")
		}
		out.Cost = rules.Coins{Amount: w.Cost.Amount, Unit: unit}
	}
	if a := w.Armor; a != nil {
		armor := &catalog.Armor{BaseAC: a.BaseAC, AddsDexBonus: a.AddsDexBonus, MaxDexBonus: a.MaxDexBonus, StrengthMinimum: a.StrengthMinimum, StealthDisadvantage: a.StealthDisadvantage}
		if armor.Category, ok = catalog.ParseArmorCategory(a.Category); !ok {
			return nil, bad("armor.category")
		}
		out.Armor = armor
	}
	if v := w.Weapon; v != nil {
		weapon := &catalog.Weapon{NormalRange: rules.Feet(v.NormalRange), LongRange: rules.Feet(v.LongRange), ThrowNormal: rules.Feet(v.ThrowNormalRange), ThrowLong: rules.Feet(v.ThrowLongRange)}
		if weapon.Category, ok = catalog.ParseWeaponCategory(v.Category); !ok {
			return nil, bad("weapon.category")
		}
		if weapon.Range, ok = catalog.ParseWeaponRange(v.Range); !ok {
			return nil, bad("weapon.range")
		}
		var err error
		if weapon.Damage, err = damage("weapon.damage", v.Damage); err != nil {
			return nil, err
		}
		if weapon.TwoHandedDamage, err = damage("weapon.twoHandedDamage", v.TwoHandedDamage); err != nil {
			return nil, err
		}
		for _, property := range v.Properties {
			weapon.Properties = append(weapon.Properties, rules.Slug(property))
		}
		out.Weapon = weapon
	}
	return out, nil
}

func customOf(c domain.CustomOption) CustomOption {
	var item *catalogapi.Item
	if c.Item != nil {
		v := catalogapi.ItemOf(catalog.Item{Category: c.Item.Category, Slot: c.Item.Slot, Cost: c.Item.Cost, Weight: c.Item.Weight, Weapon: c.Item.Weapon, Armor: c.Item.Armor})
		v.Icon = c.Item.Icon
		item = &v
	}
	return CustomOption{Item: item, Reference: c.Reference, ID: c.ID, Kind: c.Kind, Name: c.Name, Description: c.Description, Source: c.Source, Parent: c.Parent, Ability: c.Ability, Mode: c.Mode, Placement: c.Placement, Level: c.Level, HitDie: c.HitDie, Speed: c.Speed, Count: c.Count, Selected: c.Selected}
}
func (c CustomOption) domain() (domain.CustomOption, error) {
	item, err := CustomItemOf(c.Item)
	return domain.CustomOption{Item: item, Reference: c.Reference, ID: c.ID, Kind: c.Kind, Name: c.Name, Description: c.Description, Source: c.Source, Parent: c.Parent, Ability: c.Ability, Mode: c.Mode, Placement: c.Placement, Level: c.Level, HitDie: c.HitDie, Speed: c.Speed, Count: c.Count, Selected: c.Selected}, err
}
func (h *Handler) CustomOptions(c *gin.Context) {
	character, err := h.service.Get(c.Request.Context(), h.owner(c), idOf(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	options := []CustomOption{}
	for _, v := range domain.CustomOptions(character.Log) {
		options = append(options, customOf(v))
	}
	c.JSON(http.StatusOK, gin.H{"revision": character.Revision, "options": options})
}
func (h *Handler) UpsertCustomOption(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	var p struct {
		Revision int          `json:"revision"`
		Option   CustomOption `json:"option"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		helpers.FormatError(c, err)
		return
	}
	option, err := p.Option.domain()
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	result, err := h.service.UpsertCustomOption(charuc.WithRevision(c.Request.Context(), p.Revision), h.owner(c), idOf(c), helpers.Locale(c), option)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, WriteResponse{Revision: result.Revision, Seq: result.Seq, Sheet: SheetOf(result.Sheet)})
}

// RemoveCustomOption handles DELETE /v1/characters/:id/custom-options/:option.
// The revision rides in the query: a DELETE has no body to carry it.
func (h *Handler) RemoveCustomOption(c *gin.Context) {
	revision, err := strconv.Atoi(c.Query("revision"))
	if err != nil {
		helpers.FormatError(c, types.NewFieldValidationError("invalid revision", types.FieldError{
			Field: "revision", Rule: "invalid", Reason: "field.revision.invalid",
		}))
		return
	}
	result, err := h.service.RemoveCustomOption(charuc.WithRevision(c.Request.Context(), revision), h.owner(c), idOf(c), helpers.Locale(c), c.Param("option"))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, WriteResponse{Revision: result.Revision, Seq: result.Seq, Sheet: SheetOf(result.Sheet)})
}
