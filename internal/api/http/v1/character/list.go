package character

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// ListResponse is the body of GET /v1/characters.
type ListResponse struct {
	Characters []Summary `json:"characters"`
}

// List handles GET /v1/characters.
//
// `?folder=` narrows the listing to one folder. Naming a folder the caller does
// not own is a 404 rather than an empty list, so an unowned id cannot be
// mistaken for a folder with nothing in it.
func (h *Handler) List(c *gin.Context) {
	summaries, err := h.service.List(c.Request.Context(), h.owner(c), folderOf(c), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	out := make([]Summary, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, summaryOf(s))
	}
	c.JSON(http.StatusOK, ListResponse{Characters: out})
}
