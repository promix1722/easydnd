package character

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// ReviseEvents applies a complete spell draft without intermediate partial saves.
func (h *Handler) ReviseEvents(c *gin.Context) {
	var params struct {
		ExpectedSeq      int `json:"expectedSeq"`
		ExpectedRevision int `json:"expectedRevision"`
		Replacements     []struct {
			Seq   int   `json:"seq"`
			Event Event `json:"event"`
		} `json:"replacements"`
		Events []Event `json:"events"`
	}
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}
	dryRun, err := boolQuery(c, DryRunQueryParam)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	edits := make(map[int]*domain.Event)
	now := time.Now().UTC()
	for i, edit := range params.Replacements {
		if _, duplicate := edits[edit.Seq]; duplicate {
			helpers.FormatError(c, types.NewValidationError("duplicate replacement address"))
			return
		}
		event, fields := toEvent(edit.Event, i)
		if len(fields) > 0 {
			helpers.FormatError(c, types.NewFieldValidationError("invalid replacement", fields...))
			return
		}
		event.At = now
		edits[edit.Seq] = &event
	}
	added, err := toEvents(params.Events)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	for i := range added {
		added[i].At = now
	}
	ctx := charuc.WithRevision(c.Request.Context(), params.ExpectedRevision)
	result, err := h.service.ReviseBatch(ctx, h.owner(c), idOf(c), helpers.Locale(c), params.ExpectedSeq, edits, added, !dryRun)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, WriteResponse{Seq: result.Seq, Revision: result.Revision, Sheet: SheetOf(result.Sheet), Dropped: droppedOf(result.Dropped)})
}
