package character

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// CopyLink is a freshly minted copy link. The server returns a token and the
// client builds the URL, as with a group invite.
type CopyLink struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CopyLinkParams carries a copy-link token -- in the body, never the URL, for
// the reason group.InviteParams gives.
type CopyLinkParams struct {
	Token string `json:"token"`
}

// CopyLinkPreview is what the holder of a link is shown before taking it.
type CopyLinkPreview struct {
	Name    string       `json:"name"`
	Level   int          `json:"level"`
	Classes []ClassLevel `json:"classes,omitempty"`
}

// CreateCopyLink handles POST /v1/characters/{id}/copy-links. Owner only.
func (h *Handler) CreateCopyLink(c *gin.Context) {
	token, expires, err := h.service.CreateCopyLink(c.Request.Context(), h.owner(c), idOf(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusCreated, CopyLink{Token: token, ExpiresAt: expires})
}

// PreviewCopyLink handles POST /v1/copy-links/preview.
func (h *Handler) PreviewCopyLink(c *gin.Context) {
	var params CopyLinkParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}
	summary, err := h.service.PreviewCopyLink(c.Request.Context(), params.Token, helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, CopyLinkPreview{
		Name: summary.Name, Level: summary.Level, Classes: classLevels(summary.Classes),
	})
}

// AcceptCopyLink handles POST /v1/copy-links/accept. It answers as Copy does:
// the holder has a character that did not exist a moment ago.
func (h *Handler) AcceptCopyLink(c *gin.Context) {
	var params CopyLinkParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	ctx := c.Request.Context()
	locale := helpers.Locale(c)
	owner := h.owner(c)

	copied, err := h.service.AcceptCopyLink(ctx, owner, params.Token, locale)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	sheet, err := h.service.Sheet(ctx, owner, copied.ID, locale)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusCreated, CreateResponse{
		ID:    copied.ID.String(),
		Seq:   copied.Log.LastSeq(),
		Sheet: SheetOf(sheet),
	})
}
