package group

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// CreateParams is the body of POST /v1/groups.
type CreateParams struct {
	Name string `json:"name"`
}

// Create handles POST /v1/groups. Whoever creates a group owns it.
//
// A guest may do this. Their group is durable even though their session is
// not, which is a hazard rather than a feature -- see docs/backend.md, where
// the reaper that is meant to clean up after them is written down.
func (h *Handler) Create(c *gin.Context) {
	var params CreateParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	ctx := c.Request.Context()
	actor := h.actor(c)
	created, err := h.service.Create(ctx, actor, params.Name)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	// Read the group back rather than assembling a one-member roster here.
	// The response shape is then produced by exactly one code path, so it
	// cannot drift between "just created" and "opened a moment later".
	g, role, members, err := h.service.Get(ctx, actor.ID, created.ID)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusCreated, groupOf(g, role, members))
}
