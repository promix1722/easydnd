// Package logging builds the application's structured logger and carries a
// request-scoped logger on context.
//
// There is deliberately no package-level logger. Every consumer either
// receives a *slog.Logger through its constructor or pulls the request-scoped
// one off the context.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/promix1722/easydnd/internal/config"
)

// New builds a logger writing to w. The caller owns the writer.
func New(level slog.Level, format string, w io.Writer) (*slog.Logger, error) {
	opts := &slog.HandlerOptions{
		Level: level,
		// Source locations are noise at info level and invaluable at debug.
		AddSource: level <= slog.LevelDebug,
	}

	switch strings.ToLower(format) {
	case config.FormatJSON:
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	case config.FormatText:
		return slog.New(slog.NewTextHandler(w, opts)), nil
	default:
		return nil, fmt.Errorf("unsupported log format %q", format)
	}
}

type ctxKey struct{}

// IntoContext returns a copy of ctx carrying log. The request-ID middleware
// uses this so that deeper layers can log with request_id attached without
// importing anything HTTP-shaped.
func IntoContext(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, log)
}

// FromContext returns the request-scoped logger. It never returns nil and
// never falls back to a global: outside a request it discards, so a missing
// logger degrades to silence rather than to a panic or to stray output.
func FromContext(ctx context.Context) *slog.Logger {
	if log, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && log != nil {
		return log
	}
	return slog.New(slog.DiscardHandler)
}
