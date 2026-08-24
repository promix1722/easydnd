package httpapi

import (
	"net/http"

	"github.com/promix1722/easydnd/internal/config"
)

// NewServer wraps a handler in an http.Server with every timeout populated.
// Go's zero values here all mean "no timeout", which is an open invitation to
// slow-loris and to leaked idle connections.
func NewServer(cfg config.HTTPConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr(),
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}
}
