package folder

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// Delete handles DELETE /v1/folders/{id}.
//
// It deletes the characters filed in the folder along with it, and there is no
// undo: characters live in memory, so there is not even a backup to go back to.
// A client that offers this owes the player a confirmation naming the folder
// and saying how many characters it is about to destroy.
//
// The default folder is refused, with a 400 rather than a 404: it exists, the
// caller owns it, and the honest answer is that this particular folder cannot
// go.
func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.DeleteFolder(c.Request.Context(), h.owner(c), idOf(c)); err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
