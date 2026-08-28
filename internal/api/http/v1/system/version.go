package system

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// VersionResponse is the body of GET /v1/version.
//
// DEPLOY CONTRACT -- do not change the shape of this without changing the
// pipeline. deploy/deploy.sh health-gates a release by matching the literal
// `"version":"<release>"` in this body, and .github/workflows/deploy.yml
// matches the public response the same way. Neither parses JSON, so the field
// name, the quoting and the absence of a space after the colon are all part of
// the contract. That is exactly what encoding/json emits, which is why this
// stays a plain struct with no custom marshaller and why the handler uses
// c.JSON rather than c.IndentedJSON.
//
// The match is anchored on the field rather than being a bare substring search
// of the body, and that is not fussiness: the release identifier is a tag now,
// and a loose grep for `v1.0.4` also matches `v1.0.40`.
type VersionResponse struct {
	Version string `json:"version"`
}

// Version handles GET /v1/version.
func (h *Handler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, VersionResponse{Version: h.version})
}
