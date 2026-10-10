package character

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// EventsResponse is the body of GET /v1/characters/{id}/events.
type EventsResponse struct {
	Rules    helpers.RulesLock `json:"rules"`
	Revision int               `json:"revision"`
	Seq      int               `json:"seq"`
	Events   []Event           `json:"events"`
}

// Events handles GET /v1/characters/{id}/events.
func (h *Handler) Events(c *gin.Context) {
	character, err := h.service.Get(c.Request.Context(), h.owner(c), idOf(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	cat, err := h.service.CharacterCatalog(c.Request.Context(), h.owner(c), idOf(c), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	selections, err := domain.ResolvedSelections(character.Log, cat)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	conv := catalogapi.NewConverter(cat)
	events := make([]Event, 0, character.Log.Len())
	for _, e := range character.Log.Events {
		wire := eventOf(e)
		for _, answer := range e.Choices {
			nested := false
			for _, parent := range e.Choices {
				if strings.HasPrefix(answer.Prompt.String(), parent.Prompt.String()+"/") {
					nested = true
				}
			}
			if nested {
				continue
			}
			resolved, found := selections[answer.Prompt]
			if found && wire.ChoiceKind == "" {
				wire.ChoiceSource = refString(resolved.Source)
				wire.ChoiceKind = resolved.Kind.String()
				wire.Purpose = resolved.Purpose
			}
			for _, option := range resolved.Options {
				wire.Selections = append(wire.Selections, conv.OptionValue(option))
			}
		}
		events = append(events, wire)
	}
	c.JSON(http.StatusOK, EventsResponse{
		Rules: helpers.RulesLockOf(character.Log.RulesLock()), Revision: character.Revision, Seq: character.Log.LastSeq(), Events: events})
}
