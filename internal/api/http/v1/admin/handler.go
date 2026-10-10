// Package admin serves the superadmin's listings of every account and every
// character. The router mounts it behind middleware.RequireSuperadmin.
package admin

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	adminuc "github.com/promix1722/easydnd/internal/usecase/admin"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

type Service interface {
	Players(context.Context, user.Query) ([]user.Listed, int, error)
	Characters(context.Context, adminuc.CharacterQuery) ([]adminuc.Character, int, error)
}

type Handler struct{ svc Service }

func New(svc Service) *Handler { return &Handler{svc: svc} }

// Player is the wire form of one account row.
type Player struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
	CreatedAt   string `json:"created_at"`
	// LastUsedAt is the latest sign-in, absent for a row nobody signed in to.
	LastUsedAt string `json:"last_used_at,omitempty"`
	Passkeys   int    `json:"passkeys"`
	Anonymous  bool   `json:"anonymous"`
}

// Character is the wire form of one character row.
type Character struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Level     int     `json:"level"`
	Classes   []Class `json:"classes"`
	Owner     string  `json:"owner"`
	OwnerName string  `json:"owner_name,omitempty"`
	Public    bool    `json:"public"`
	Revision  int     `json:"revision"`
}

// Class is one entry of a class line; the client names the slug.
type Class struct {
	Class string `json:"class"`
	Level int    `json:"level"`
}

// Players handles GET /v1/admin/players.
func (h *Handler) Players(c *gin.Context) {
	q := user.Query{Text: c.Query("q")}
	var err error
	if q.Limit, q.Offset, err = page(c); err == nil {
		switch kind := c.Query("kind"); kind {
		case "":
		case "account", "guest":
			guests := kind == "guest"
			q.Guests = &guests
		default:
			err = invalidParam("kind")
		}
	}
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	rows, total, err := h.svc.Players(c.Request.Context(), q)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	players := make([]Player, 0, len(rows))
	for _, r := range rows {
		p := Player{
			ID: string(r.ID), DisplayName: r.DisplayName, Email: r.Email,
			CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339),
			Passkeys:  r.Passkeys, Anonymous: r.Anonymous,
		}
		if !r.LastUsedAt.IsZero() {
			p.LastUsedAt = r.LastUsedAt.UTC().Format(time.RFC3339)
		}
		players = append(players, p)
	}
	c.JSON(http.StatusOK, gin.H{"players": players, "total": total})
}

// Characters handles GET /v1/admin/characters.
func (h *Handler) Characters(c *gin.Context) {
	q := adminuc.CharacterQuery{Owner: c.Query("owner"), ID: c.Query("id")}
	var err error
	if q.Limit, q.Offset, err = page(c); err == nil {
		if raw, ok := c.GetQuery("public"); ok {
			public, parseErr := strconv.ParseBool(raw)
			if parseErr != nil {
				err = invalidParam("public")
			}
			q.Public = &public
		}
	}
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	rows, total, err := h.svc.Characters(c.Request.Context(), q)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	characters := make([]Character, 0, len(rows))
	for _, r := range rows {
		classes := make([]Class, 0, len(r.Classes))
		for _, cl := range r.Classes {
			classes = append(classes, Class{Class: string(cl.Class), Level: cl.Level})
		}
		characters = append(characters, Character{
			ID: r.ID.String(), Name: r.Name, Level: r.Level, Classes: classes,
			Owner: r.Owner.String(), OwnerName: r.OwnerName,
			Public: r.Public, Revision: r.Revision,
		})
	}
	c.JSON(http.StatusOK, gin.H{"characters": characters, "total": total})
}

// page reads ?limit= and ?offset=, the same contract the spell search has.
func page(c *gin.Context) (limit, offset int, err error) {
	limit = defaultPageSize
	if raw, ok := c.GetQuery("limit"); ok {
		if limit, err = strconv.Atoi(raw); err != nil || limit < 1 || limit > maxPageSize {
			return 0, 0, invalidParam("limit")
		}
	}
	if raw, ok := c.GetQuery("offset"); ok {
		if offset, err = strconv.Atoi(raw); err != nil || offset < 0 {
			return 0, 0, invalidParam("offset")
		}
	}
	return limit, offset, nil
}

func invalidParam(param string) error {
	return types.NewFieldValidationError("invalid "+param, types.FieldError{
		Field: param, Rule: "invalid",
		Reason: "field." + param + ".invalid",
	})
}
