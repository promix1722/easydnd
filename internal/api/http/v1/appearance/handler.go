// Package appearance serves the current account's appearance resource.
package appearance

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
	Get(context.Context, user.User) (user.Appearance, error)
	Put(context.Context, user.User, user.Appearance) (user.Appearance, error)
}

type Handler struct{ svc Service }

func New(svc Service) *Handler { return &Handler{svc: svc} }

// Get handles GET /v1/appearance.
func (h *Handler) Get(c *gin.Context) {
	account, ok := middleware.UserFrom(c)
	if !ok {
		helpers.FormatError(c, types.NewUnauthenticatedError("no session").Because("auth.noSession"))
		return
	}
	appearance, err := h.svc.Get(c.Request.Context(), account)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, appearanceWire(appearance))
}

// Appearance is the wire representation of a user's appearance preference.
type Appearance struct {
	Palette     string `json:"palette"`
	ColorScheme string `json:"color_scheme"`
}

func appearanceWire(a user.Appearance) Appearance {
	return Appearance{Palette: a.Palette, ColorScheme: a.ColorScheme}
}

// Put handles PUT /v1/appearance.
func (h *Handler) Put(c *gin.Context) {
	account, ok := middleware.UserFrom(c)
	if !ok {
		helpers.FormatError(c, types.NewUnauthenticatedError("no session").Because("auth.noSession"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	var params Appearance
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, types.NewValidationError("invalid appearance body"))
		return
	}
	saved, err := h.svc.Put(c.Request.Context(), account, user.Appearance{Palette: params.Palette, ColorScheme: params.ColorScheme})
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, appearanceWire(saved))
}
