package system

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AnalyticsConfigResponse is the explicit allowlist of public analytics settings.
type AnalyticsConfigResponse struct {
	Environment string `json:"environment"`
	Token       string `json:"token"`
	Host        string `json:"host"`
}

func (h *Handler) AnalyticsConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.analytics)
}
