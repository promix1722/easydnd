package character

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/types"
)

// The query parameters of DELETE /v1/characters/{id}/events.
//
// Query parameters rather than a body: a DELETE with a body is legal but
// unevenly supported by proxies and client libraries, and these two numbers
// are small enough that a URL is a fair place for them.
const (
	AfterQueryParam       = "after"
	ExpectedSeqQueryParam = "expectedSeq"
)

// TruncateEvents handles DELETE /v1/characters/{id}/events?after=N&expectedSeq=M.
//
// This is the suffix-dropping primitive: un-taking a level, and everything
// after it. The log's invariant is not "append-only" -- that would make going
// back impossible -- but "append, drop a suffix, or replace one entry and
// revalidate what follows".
//
// It is not what changing a pick needs, and the reason is not the one an
// earlier version of this comment gave. Answers do fold last-write-wins, but a
// prompt that has been answered stops being emitted, so re-answering it is
// rejected as a prompt the character does not have open; changing a pick is
// ReplaceEvent. Nothing in the web client calls this route any more. It stays
// because it is working, tested API, and removing it would be a breaking
// change made as a side effect of a UI decision.
func (h *Handler) TruncateEvents(c *gin.Context) {
	after, err := intQuery(c, AfterQueryParam)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	expected, err := intQuery(c, ExpectedSeqQueryParam)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}

	ctx := c.Request.Context()
	id := idOf(c)
	locale := helpers.Locale(c)

	if err := h.service.Truncate(ctx, h.owner(c), id, expected, after); err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.writeResponse(c, id, locale, after)
}

func intQuery(c *gin.Context, name string) (int, error) {
	raw := c.Query(name)
	if raw == "" {
		return 0, types.NewFieldValidationError("a required parameter is missing", types.FieldError{
			Field: name, Rule: "required", Message: "this parameter is required",
		})
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, types.NewFieldValidationError("a parameter is not a number", types.FieldError{
			Field: name, Rule: "format", Message: "this parameter must be a whole number",
		})
	}
	return value, nil
}
