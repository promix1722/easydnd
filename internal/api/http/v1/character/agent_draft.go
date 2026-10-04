package character

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// AgentDraft reuses the standard character protocol against the session log.
// Keep this allowlist explicit: saving/copying/deleting characters is not an
// editor operation and must never escape the session's draft repository.
func (h *Handler) AgentDraft(c *gin.Context) {
	if !h.agentAvailable(c) {
		return
	}
	path := strings.Trim(c.Param("path"), "/")
	parts := strings.Split(path, "/")
	var dispatch func(*Handler)
	switch c.Request.Method + " " + path {
	case "GET sheet":
		dispatch = func(d *Handler) { d.Sheet(c) }
	case "GET prompts":
		dispatch = func(d *Handler) { d.Prompts(c) }
	case "GET custom-options":
		dispatch = func(d *Handler) { d.CustomOptions(c) }
	case "POST custom-options":
		dispatch = func(d *Handler) { d.UpsertCustomOption(c) }
	case "GET events":
		dispatch = func(d *Handler) { d.Events(c) }
	case "POST events":
		dispatch = func(d *Handler) { d.AppendEvents(c) }
	case "POST events/revise":
		dispatch = func(d *Handler) { d.ReviseEvents(c) }
	default:
		if len(parts) == 2 && parts[0] == "catalog" && c.Request.Method == "GET" {
			c.Params = append(c.Params, gin.Param{Key: "collection", Value: parts[1]})
			dispatch = func(d *Handler) { d.Catalog(c) }
		}
		if len(parts) == 2 && parts[0] == "events" {
			c.Params = append(c.Params, gin.Param{Key: "seq", Value: parts[1]})
			if c.Request.Method == "PUT" {
				dispatch = func(d *Handler) { d.ReplaceEvent(c) }
			}
			if c.Request.Method == "DELETE" {
				dispatch = func(d *Handler) { d.DeleteEvent(c) }
			}
		}
	}
	if dispatch == nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	if err := h.agent.WithDraft(h.owner(c), c.Param("id"), c.Request.Method != "GET", func(service *charuc.Service) { dispatch(New(service, h.log)) }); err != nil {
		helpers.FormatError(c, err)
	}
}
