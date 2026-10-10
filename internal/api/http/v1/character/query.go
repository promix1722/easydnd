package character

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// ExpectedSeqQueryParam names the sequence a DELETE states in its query.
//
// A query parameter rather than a body: a DELETE with a body is legal but
// unevenly supported by proxies and client libraries, and one small number is
// a fair thing to put in a URL.
const ExpectedSeqQueryParam = "expectedSeq"

func intQuery(c *gin.Context, name string) (int, error) {
	raw := c.Query(name)
	if raw == "" {
		return 0, types.NewFieldValidationError("a required parameter is missing", types.FieldError{
			Field: name, Rule: "required", Reason: "field.param.required",
		})
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, types.NewFieldValidationError("a parameter is not a number", types.FieldError{
			Field: name, Rule: "format", Reason: "field.param.integer",
		})
	}
	return value, nil
}

func guardRevision(c *gin.Context, fallback int) error {
	revision := fallback
	if c.Query("expectedRevision") != "" {
		v, err := intQuery(c, "expectedRevision")
		if err != nil {
			return err
		}
		revision = v
	}
	c.Request = c.Request.WithContext(charuc.WithRevision(c.Request.Context(), revision))
	return nil
}
