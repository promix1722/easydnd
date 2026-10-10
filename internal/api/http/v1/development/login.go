// Package development contains routes registered only in development.
package development

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

type LoginService interface {
	Login(context.Context, string) (token string, games []string, err error)
}

type Handler struct {
	service LoginService
	cookies helpers.CookieOptions
	ttl     time.Duration
}

func New(service LoginService, cookies helpers.CookieOptions, ttl time.Duration) *Handler {
	return &Handler{service: service, cookies: cookies, ttl: ttl}
}

func (h *Handler) Login(c *gin.Context) {
	var body struct {
		Account string `json:"account"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		helpers.FormatError(c, err)
		return
	}
	token, games, err := h.service.Login(c.Request.Context(), body.Account)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.cookies.SetSession(c, token, h.ttl)
	c.JSON(http.StatusOK, gin.H{"game_ids": games})
}
