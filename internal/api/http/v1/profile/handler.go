// Package profile serves the signed-in account's own portrait.
package profile

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/api/http/middleware"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

type Service interface {
	PutImage(context.Context, user.User, string) error
}

type Handler struct{ svc Service }

func New(svc Service) *Handler { return &Handler{svc: svc} }

// PutImage handles PUT /v1/profile/image; the caller cannot name another account.
func (h *Handler) PutImage(c *gin.Context) {
	account, ok := middleware.UserFrom(c)
	if !ok {
		helpers.FormatError(c, types.NewUnauthenticatedError("no session").Because("auth.noSession"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 300<<10)
	var params struct {
		Image *string `json:"image"`
	}
	if err := c.ShouldBindJSON(&params); err != nil || params.Image == nil {
		helpers.FormatError(c, types.NewValidationError("invalid portrait body"))
		return
	}
	if err := h.svc.PutImage(c.Request.Context(), account, *params.Image); err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
