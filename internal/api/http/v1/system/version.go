package system

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// VersionResponse is the body of GET /v1/version.
//
// DEPLOY CONTRACT -- do not change the shape of this without changing the
// pipeline. deploy/deploy.sh health-gates a release with
// `curl .../v1/version | grep -q "$SHA"`, and .github/workflows/deploy.yml
// substring-matches the public response the same way. Neither parses JSON, so
// the raw commit SHA must appear verbatim in this body: never truncate it to a
// short SHA and never prefix it.
type VersionResponse struct {
	Version string `json:"version"`
}

// Version handles GET /v1/version.
func (h *Handler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, VersionResponse{Version: h.version})
}
