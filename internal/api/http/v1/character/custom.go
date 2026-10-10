package character

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

type CustomOption struct {
	Reference   string `json:"ref,omitempty"`
	ID          string `json:"id,omitempty"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Parent      string `json:"parent,omitempty"`
	Ability     string `json:"ability,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Placement   string `json:"placement,omitempty"`
	Level       *int   `json:"level,omitempty"`
	HitDie      *int   `json:"hitDie,omitempty"`
	Speed       *int   `json:"speed,omitempty"`
	Count       int    `json:"count,omitempty"`
	Selected    bool   `json:"selected"`
}

func customOf(c domain.CustomOption) CustomOption {
	return CustomOption{Reference: c.Reference, ID: c.ID, Kind: c.Kind, Name: c.Name, Description: c.Description, Source: c.Source, Parent: c.Parent, Ability: c.Ability, Mode: c.Mode, Placement: c.Placement, Level: c.Level, HitDie: c.HitDie, Speed: c.Speed, Count: c.Count, Selected: c.Selected}
}
func (c CustomOption) domain() domain.CustomOption {
	return domain.CustomOption{Reference: c.Reference, ID: c.ID, Kind: c.Kind, Name: c.Name, Description: c.Description, Source: c.Source, Parent: c.Parent, Ability: c.Ability, Mode: c.Mode, Placement: c.Placement, Level: c.Level, HitDie: c.HitDie, Speed: c.Speed, Count: c.Count, Selected: c.Selected}
}
func (h *Handler) CustomOptions(c *gin.Context) {
	character, err := h.service.Get(c.Request.Context(), h.owner(c), idOf(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	options := []CustomOption{}
	for _, v := range domain.CustomOptions(character.Log) {
		options = append(options, customOf(v))
	}
	c.JSON(http.StatusOK, gin.H{"revision": character.Revision, "options": options})
}
func (h *Handler) UpsertCustomOption(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	var p struct {
		Revision int          `json:"revision"`
		Option   CustomOption `json:"option"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		helpers.FormatError(c, err)
		return
	}
	result, err := h.service.UpsertCustomOption(charuc.WithRevision(c.Request.Context(), p.Revision), h.owner(c), idOf(c), helpers.Locale(c), p.Option.domain())
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, WriteResponse{Revision: result.Revision, Seq: result.Seq, Sheet: SheetOf(result.Sheet)})
}
