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
	Blocked []string          `json:"blocked,omitempty"`
	Purpose string            `json:"purpose,omitempty"`
	UpTo    bool              `json:"upTo,omitempty"`
	Choice  catalogapi.Choice `json:"choice"`

	// Source names the catalogue entry posing this prompt, as "kind:slug".
	// Empty for a prompt the compendium does not pose.
	Source string `json:"source,omitempty"`

	Group string `json:"group"`
	Level int    `json:"level,omitempty"`

	// Optional reports that a character is complete without answering it.
	Optional bool `json:"optional"`

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
type SpellRule struct {
	ID            string   `json:"id"`
	Source        string   `json:"source"`
	Class         string   `json:"class,omitempty"`
	ClassLevel    int      `json:"classLevel,omitempty"`
	Count         int      `json:"count"`
	MinLevel      int      `json:"minLevel"`
	MaxLevel      int      `json:"maxLevel"`
	MaxLevelCount *int     `json:"maxLevelCount,omitempty"`
	Purpose       string   `json:"purpose"`
	Optional      bool     `json:"optional"`
	ListClasses   []string `json:"listClasses,omitempty"`
	Automatic     []string `json:"automatic,omitempty"`
}

type BuildPolicy struct {
	MinScore       int         `json:"minScore"`
	MaxScore       int         `json:"maxScore"`
	MaxLevel       int         `json:"maxLevel"`
	StandardArray  []int       `json:"standardArray"`
	PointBuyBudget int         `json:"pointBuyBudget"`
	PointCosts     map[int]int `json:"pointCosts"`
}
type PromptsResponse struct {
	BuildPolicy *BuildPolicy `json:"buildPolicy,omitempty"`
	SpellRules  []SpellRule  `json:"spellRules,omitempty"`
	Revision    int          `json:"revision"`
	Seq         int          `json:"seq"`

	// Complete reports that nothing required is outstanding. It is separate
	// from the list being empty, because a character with only optional
	// prompts left -- an unwritten personality trait -- is finished.
	Complete bool     `json:"complete"`
	Prompts  []Prompt `json:"prompts"`
}

// Prompts handles GET /v1/characters/{id}/prompts.
//
// This is the endpoint the build flow is driven by, for creation and for
// level-up alike: raising identity.desiredLevel raises the character's level,
// and the levels that adds pose their own questions here.
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
	var prompts []domain.Prompt
	if c.Request.URL.Query().Has("before") {
		var before int
		before, err = intQuery(c, "before")
		if err == nil {
			prompts, err = h.service.PromptsBefore(ctx, owner, id, locale, before)
		}
	} else {
		prompts, err = h.service.Prompts(ctx, owner, id, locale)
	}
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	cat, err := h.service.CharacterCatalog(ctx, owner, id, locale)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}

	rules, err := domain.SpellRules(character.Log, cat)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	spellRules := make([]SpellRule, 0, len(rules))
	for _, r := range rules {
		spellRules = append(spellRules, SpellRule{ID: r.ID.String(), Source: refString(r.Source), Class: r.Class.String(), ClassLevel: r.ClassLevel, Count: r.Count, MinLevel: r.MinLevel, MaxLevel: r.MaxLevel, MaxLevelCount: r.MaxLevelCount, Purpose: r.Purpose, Optional: r.Optional, ListClasses: slugStrings(r.ListClasses), Automatic: slugStrings(r.Automatic)})
	}
	conv := catalogapi.NewConverter(cat)
	out := make([]Prompt, 0, len(prompts))
	for _, p := range prompts {
		out = append(out, promptOf(p, conv))
	}

	var policy *BuildPolicy
	if core := cat.Mechanics.Core; core.MaxScore > 0 {
		policy = &BuildPolicy{core.MinScore, core.MaxScore, core.MaxLevel, core.StandardArray, core.PointBuyBudget, core.PointCosts}
	}
	c.JSON(http.StatusOK, PromptsResponse{
		BuildPolicy: policy,
		SpellRules:  spellRules,
		Seq:         character.Log.LastSeq(),
		Revision:    character.Revision,
		Complete:    domain.Complete(prompts),
		Prompts:     out,
	})
}

func promptOf(p domain.Prompt, conv catalogapi.Converter) Prompt {
	out := Prompt{
		Choice:  conv.ChoiceValue(p.Choice),
		Purpose: p.Purpose, UpTo: p.UpTo,
		Source:   refString(p.Source),
		Group:    p.Group.String(),
		Level:    p.Level,
		Optional: p.Optional,
		Event: PromptEvent{
			Type:  p.Event.Type.String(),
			Ref:   refString(p.Event.Ref),
			Level: p.Event.Level,
		},
		Held:     slugStrings(p.Held),
		Blocked:  slugStrings(p.Blocked),
		HeldOnly: p.HeldOnly,
	}
	return out
}
