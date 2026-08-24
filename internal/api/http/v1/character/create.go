package character

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// CreateParams is the body of POST /v1/characters.
//
// The scores are the base array as generated, before any racial bonus. The
// projection applies those every time it runs, which is what lets a player
// change their mind about a race without re-entering six numbers.
type CreateParams struct {
	Name      string         `json:"name"`
	Alignment string         `json:"alignment"`
	Method    string         `json:"method"`
	Abilities map[string]int `json:"abilities"`
}

// CreateResponse is what a newly created character looks like.
type CreateResponse struct {
	ID    string `json:"id"`
	Seq   int    `json:"seq"`
	Sheet Sheet  `json:"sheet"`
}

// unknownAbilities reports ability keys the rules do not have. All of them,
// not the first: a client that sent three typos deserves to hear about three.
func unknownAbilities(keys []string) error {
	slices.Sort(keys)
	fields := make([]types.FieldError, 0, len(keys))
	for _, key := range keys {
		fields = append(fields, types.FieldError{
			Field: "abilities." + key, Rule: "unknown",
			Message: "the abilities are str, dex, con, int, wis and cha",
		})
	}
	return types.NewFieldValidationError("the character could not be created", fields...)
}

// Create handles POST /v1/characters.
func (h *Handler) Create(c *gin.Context) {
	var params CreateParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	abilities := make(map[rules.Ability]int, len(params.Abilities))
	var unknown []string
	for key, score := range params.Abilities {
		ability, ok := rules.ParseAbility(key)
		if !ok {
			unknown = append(unknown, key)
			continue
		}
		abilities[ability] = score
	}
	if len(unknown) > 0 {
		helpers.FormatError(c, unknownAbilities(unknown))
		return
	}

	ctx := c.Request.Context()
	locale := helpers.Locale(c)
	created, err := h.service.Create(ctx, h.owner(c), charuc.NewCharacter{
		Name:      params.Name,
		Alignment: rules.Slug(params.Alignment),
		Method:    rules.Slug(params.Method),
		Abilities: abilities,
	})
	if err != nil {
		helpers.FormatError(c, err)
		return
	}

	sheet, err := h.service.Sheet(ctx, h.owner(c), created.ID, locale)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusCreated, CreateResponse{
		ID:    created.ID.String(),
		Seq:   created.Log.LastSeq(),
		Sheet: sheetOf(sheet),
	})
}
