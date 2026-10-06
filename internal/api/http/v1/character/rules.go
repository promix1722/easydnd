package character

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

type MigrateParams struct {
	ExpectedRevision int               `json:"expectedRevision"`
	Rules            helpers.RulesLock `json:"rules"`
	Entities         map[string]string `json:"entities,omitempty"`
	Prompts          map[string]string `json:"prompts,omitempty"`
	Options          map[string]string `json:"options,omitempty"`
	Paths            map[string]string `json:"paths,omitempty"`
}
type MigrationIssue struct {
	Seq     int    `json:"seq"`
	EventID string `json:"eventId"`
	Reason  string `json:"reason"`
}
type MigrationResponse struct {
	Revision int               `json:"revision"`
	Rules    helpers.RulesLock `json:"rules"`
	Before   Sheet             `json:"before"`
	After    Sheet             `json:"after"`
	Changed  []string          `json:"changed"`
	Issues   []MigrationIssue  `json:"issues,omitempty"`
}

func migrationOf(m charuc.Migration) MigrationResponse {
	out := MigrationResponse{Revision: m.Revision, Rules: helpers.RulesLockOf(m.Lock), Before: SheetOf(m.Before), After: SheetOf(m.After), Changed: m.Changed}
	for _, i := range m.Issues {
		out.Issues = append(out.Issues, MigrationIssue{Seq: i.Seq, EventID: i.EventID, Reason: i.Reason})
	}
	return out
}
func (h *Handler) MigrateRules(c *gin.Context) {
	var p MigrateParams
	if err := c.ShouldBindJSON(&p); err != nil {
		helpers.FormatError(c, err)
		return
	}
	dry, err := boolQuery(c, DryRunQueryParam)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	mappings := charuc.Mappings{Entities: map[rules.Ref]rules.Ref{}, Prompts: map[rules.Slug]rules.Slug{}, Options: map[rules.Slug]rules.Slug{}}
	for from, to := range p.Entities {
		a, ok := rules.ParseRef(from)
		b, ok2 := rules.ParseRef(to)
		if !ok || !ok2 {
			helpers.FormatError(c, types.NewValidationError("invalid migration reference"))
			return
		}
		mappings.Entities[a] = b
	}
	for a, b := range p.Prompts {
		mappings.Prompts[rules.Slug(a)] = rules.Slug(b)
	}
	for a, b := range p.Options {
		mappings.Options[rules.Slug(a)] = rules.Slug(b)
	}
	mappings.Paths = map[domain.Path]domain.Path{}
	for a, b := range p.Paths {
		mappings.Paths[domain.Path(a)] = domain.Path(b)
	}
	result, err := h.service.Migrate(c.Request.Context(), h.owner(c), idOf(c), helpers.Locale(c), p.ExpectedRevision, p.Rules.Domain(), mappings, !dry)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, migrationOf(result))
}
func (h *Handler) RestoreRules(c *gin.Context) {
	var p struct {
		ExpectedRevision int `json:"expectedRevision"`
		Checkpoint       int `json:"checkpoint"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		helpers.FormatError(c, err)
		return
	}
	dry, err := boolQuery(c, DryRunQueryParam)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	result, err := h.service.RestoreCheckpoint(c.Request.Context(), h.owner(c), idOf(c), helpers.Locale(c), p.ExpectedRevision, p.Checkpoint, !dry)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, migrationOf(result))
}
func (h *Handler) Catalog(c *gin.Context) {
	cat, err := h.service.CharacterCatalog(c.Request.Context(), h.owner(c), idOf(c), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	catalogapi.ServeCollection(c, cat)
}

// SpellSearch handles POST /v1/characters/{id}/catalog/spells/search.
func (h *Handler) SpellSearch(c *gin.Context) {
	cat, err := h.service.CharacterCatalog(c.Request.Context(), h.owner(c), idOf(c), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	catalogapi.ServeSpellSearch(c, cat)
}
