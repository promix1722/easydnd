package character

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
)

// Prompt is one question the character still has to answer.
type Prompt struct {
	Choice catalogapi.Choice `json:"choice"`

	// Source names the catalogue entry posing this prompt, as "kind:slug".
	// Empty for a prompt the compendium does not pose.
	Source string `json:"source,omitempty"`

	Group string `json:"group"`
	Level int    `json:"level,omitempty"`

	// Optional reports that a character is complete without answering it.
	Optional bool `json:"optional"`

	// Advances reports that answering this prompt grants a level. It is what
	// lets one screen serve both creation and level-up.
	Advances bool `json:"advances"`

	// Event is what the answer must be posted as. A client copies it into
	// the body verbatim and adds the choices, rather than deciding for
	// itself whether a level is a class event or a level event.
	Event PromptEvent `json:"event"`

	// Held lists the options the character already has from elsewhere. They
	// are still in the option set -- removing them would make the prompt
	// depend on the order it was answered in -- so a client greys them out.
	Held []string `json:"held,omitempty"`

	// HeldOnly inverts what Held means: those options are the only legal
	// answers rather than the illegal ones. Expertise is the case -- it
	// doubles a proficiency the character already has.
	HeldOnly bool `json:"heldOnly"`
}

// PromptEvent is the event an answer must be posted as.
type PromptEvent struct {
	Type  string `json:"type"`
	Ref   string `json:"ref,omitempty"`
	Level int    `json:"level,omitempty"`
}

// PromptsResponse is the body of GET /v1/characters/{id}/prompts.
type PromptsResponse struct {
	Seq int `json:"seq"`

	// Complete reports that nothing required is outstanding. It is separate
	// from the list being empty, because a finished character still has the
	// optional prompt that offers them a level.
	Complete bool     `json:"complete"`
	Prompts  []Prompt `json:"prompts"`
}

// Prompts handles GET /v1/characters/{id}/prompts.
//
// This is the endpoint the build flow is driven by, for creation and for
// level-up alike: a finished character's remaining prompt is "which class do
// you gain a level in?", and answering it opens that level's own prompts.
func (h *Handler) Prompts(c *gin.Context) {
	ctx := c.Request.Context()
	id := idOf(c)
	locale := helpers.Locale(c)

	owner := h.owner(c)

	character, err := h.service.Get(ctx, owner, id)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	prompts, err := h.service.Prompts(ctx, owner, id, locale)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	cat, err := h.service.Catalog(ctx, locale)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}

	conv := catalogapi.NewConverter(cat)
	out := make([]Prompt, 0, len(prompts))
	for _, p := range prompts {
		out = append(out, Prompt{
			Choice:   conv.ChoiceValue(p.Choice),
			Source:   refString(p.Source),
			Group:    p.Group.String(),
			Level:    p.Level,
			Optional: p.Optional,
			Advances: p.Advances,
			Event: PromptEvent{
				Type:  p.Event.Type.String(),
				Ref:   refString(p.Event.Ref),
				Level: p.Event.Level,
			},
			Held:     slugStrings(p.Held),
			HeldOnly: p.HeldOnly,
		})
	}

	c.JSON(http.StatusOK, PromptsResponse{
		Seq:      character.Log.LastSeq(),
		Complete: domain.Complete(prompts),
		Prompts:  out,
	})
}
