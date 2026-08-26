package folder

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	domain "github.com/promix1722/easydnd/internal/domain/character"
)

// ReorderParams is the body of PUT /v1/folders/order.
type ReorderParams struct {
	// Folders is every folder the account owns except the default one, in
	// the order they should be listed. Not a single move: see Reorder.
	Folders []string `json:"folders"`
}

// Reorder handles PUT /v1/folders/order.
//
// A PUT of the whole order rather than a PATCH of one folder's position, and
// the difference is not stylistic. "Move this one up" applied to a list that
// has changed since the caller drew it moves the wrong folder; a complete order
// either matches what the account has or is refused, and sending it twice
// leaves the same result. That is what makes a drag-and-drop client safe to
// write without a version on every row.
//
// The default folder is not in the body. It leads the listing, so there is no
// position for it to take, and naming it is a 400.
//
// Its own path segment rather than a field on the folder resource, because the
// order is a fact about the collection: no single folder's PATCH can express
// "these three, in this sequence".
func (h *Handler) Reorder(c *gin.Context) {
	var params ReorderParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	ids := make([]domain.FolderID, 0, len(params.Folders))
	for _, id := range params.Folders {
		ids = append(ids, domain.FolderID(id))
	}

	if err := h.service.ReorderFolders(c.Request.Context(), h.owner(c), ids); err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
