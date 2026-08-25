package character

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// Stub handles POST /v1/characters/stub.
//
// It takes no body at all, which is what settles where the folder goes. Create
// states its folder alongside the rest of the character's opening state,
// because it has an opening state to state; a stub's is supplied by the server,
// so the folder rides in the query as it does for an import.
//
// The route is registered only in development -- see internal/api/http/router.go
// -- so this handler does not check the environment itself. A guard in two
// places is a guard that can disagree with itself, and the routing table is
// where "does this endpoint exist?" is already answered.
func (h *Handler) Stub(c *gin.Context) {
	ctx := c.Request.Context()
	locale := helpers.Locale(c)

	created, err := h.service.CreateStub(ctx, h.owner(c), folderOf(c), locale)
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
		Sheet: SheetOf(sheet),
	})
}
