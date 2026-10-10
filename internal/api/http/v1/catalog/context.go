package catalog

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

type fixedSource struct{ cat *catalog.Catalog }

func (s fixedSource) Load(context.Context, rules.Locale) (*catalog.Catalog, error) { return s.cat, nil }
func (s fixedSource) Locales(context.Context) ([]rules.Locale, error) {
	return []rules.Locale{s.cat.Locale()}, nil
}

// ServeCollection is called only after the character authorization boundary.
// Its request-local handler cannot leak another character's pinned context.
func ServeCollection(c *gin.Context, cat *catalog.Catalog) {
	New(fixedSource{cat}, slog.Default()).Collection(c)
}

// ServeManifest renders an already authorized selection.
func ServeManifest(c *gin.Context, cat *catalog.Catalog, log *slog.Logger) {
	New(fixedSource{cat}, log).Manifest(c)
}
