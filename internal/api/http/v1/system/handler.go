// Package system holds the operational endpoints: liveness and build version.
//
// Convention for this tree, inherited from the reference project: one exported
// handler per file, named after the action, with its request and response
// types beside it.
package system

// Handler serves the operational endpoints.
type Handler struct {
	version string
}

// New builds the handler over the injected build version. The version is
// passed in rather than read from internal/buildinfo here, so the handler
// stays testable without linker flags.
func New(version string) *Handler {
	return &Handler{version: version}
}
