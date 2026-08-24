package character

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// CreateParams is the body of POST /v1/characters.
//
// A name, and an alignment if the player already has one in mind. It used to
// take the generation method and the six base scores as well, and that seeded
// eight selections into one log entry -- which is exactly why neither the
// name nor the scores had anything a player could point at and change.
//
// The scores are an ordinary open choice now. A freshly created character has
// character/abilities outstanding, answered with its own entry, and the
// method travels with that answer rather than with creation.
type CreateParams struct {
	Name      string `json:"name"`
	Alignment string `json:"alignment"`

	// Folder files the character. Empty means the caller's default folder,
	// which is created on the spot if this is their first character.
	Folder string `json:"folder"`
}

// CreateResponse is what a newly created character looks like.
type CreateResponse struct {
	ID    string `json:"id"`
	Seq   int    `json:"seq"`
	Sheet Sheet  `json:"sheet"`
}

// Create handles POST /v1/characters.
func (h *Handler) Create(c *gin.Context) {
	var params CreateParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	ctx := c.Request.Context()
	locale := helpers.Locale(c)
	created, err := h.service.Create(ctx, h.owner(c), domain.FolderID(params.Folder), charuc.NewCharacter{
		Name:      params.Name,
		Alignment: rules.Slug(params.Alignment),
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
