package system

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthResponse is the body of GET /v1/health.
type HealthResponse struct {
	Status string `json:"status"`
}

// Health handles GET /v1/health.
//
// Liveness only: it reports that the process is up and serving. When a real
// datastore arrives, add a separate readiness endpoint that checks it rather
// than making this one depend on anything -- the deploy health gate must not
// start failing because a downstream is briefly unavailable.
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}
