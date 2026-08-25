package character

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	domain "github.com/promix1722/easydnd/internal/domain/character"
)

// CopyParams is the body of POST /v1/characters/{id}/copy.
type CopyParams struct {
	// Folder is where the copy is filed. Empty means beside the original,
	// which is what a Copy button on a row is asking for.
	Folder string `json:"folder"`
}

// Copy handles POST /v1/characters/{id}/copy.
//
// It answers with a CreateResponse, because a copy is a character that did not
// exist a moment ago and a client has the same thing to do with it as with a
// freshly created one. The copy's name is the original's with " (copy)" on the
// end, appended as one more event rather than written over the log it was
// duplicated from.
func (h *Handler) Copy(c *gin.Context) {
	// A body is optional here: copying beside the original needs nothing
	// said, and requiring `{}` would be ceremony.
	var params CopyParams
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&params); err != nil {
			helpers.FormatError(c, err)
			return
		}
	}

	ctx := c.Request.Context()
	locale := helpers.Locale(c)
	owner := h.owner(c)

	copied, err := h.service.CopyCharacter(ctx, owner, idOf(c), domain.FolderID(params.Folder), locale)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}

	sheet, err := h.service.Sheet(ctx, owner, copied.ID, locale)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusCreated, CreateResponse{
		ID:    copied.ID.String(),
		Seq:   copied.Log.LastSeq(),
		Sheet: sheetOf(sheet),
	})
}
