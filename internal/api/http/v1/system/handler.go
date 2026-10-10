// Package system holds liveness, build version, and public analytics configuration.
//
// Convention for this tree, inherited from the reference project: one exported
// handler per file, named after the action, with its request and response
// types beside it.
package system

// Handler serves the operational endpoints.
type Handler struct {
	version   string
	analytics AnalyticsConfigResponse
}

// New injects the build version and allowlisted public analytics settings.
func New(version string, analytics AnalyticsConfigResponse) *Handler {
	return &Handler{version: version, analytics: analytics}
}
