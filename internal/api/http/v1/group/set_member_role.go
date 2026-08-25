package group

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	domain "github.com/promix1722/easydnd/internal/domain/group"
)

// SetMemberRoleParams is the body of PATCH /v1/groups/{id}/members?user=U.
//
// The member is in the query and the new rank is in the body, which looks
// mixed but is not: the query says *which* row, exactly as ?after= does on the
// character events route, and the body says what to change it to.
type SetMemberRoleParams struct {
	Role string `json:"role"`
}

// SetMemberRole handles PATCH /v1/groups/{id}/members. Owner only.
//
// Asking for "owner" hands the group over: the outgoing owner becomes a DM in
// the same step. It is this route rather than a separate one because from the
// client's side both are "change their role", and splitting them would invite
// a client to promote somebody and demote itself as two requests -- with the
// group briefly having two owners, or none.
func (h *Handler) SetMemberRole(c *gin.Context) {
	var params SetMemberRoleParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	ctx := c.Request.Context()
	actor := h.actor(c).ID
	id := idOf(c)
	if err := h.service.SetRole(ctx, actor, id, targetOf(c), domain.Role(params.Role)); err != nil {
		helpers.FormatError(c, err)
		return
	}

	// The caller's own rank may have just changed -- a transfer demotes them
	// -- so the response has to be read back rather than assumed.
	g, role, members, err := h.service.Get(ctx, actor, id)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, groupOf(g, role, members))
}
