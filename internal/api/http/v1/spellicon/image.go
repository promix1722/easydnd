package spellicon

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/domain/imageasset"
	"github.com/promix1722/easydnd/internal/types"
)

// ImageReader is the one usecase port the artwork endpoint needs: the image
// seeded under slug, or a *types.NotFoundError when nothing is. SeedStore
// satisfies it -- serving resolves the slug with the same validation and
// legacy-name fallback the seed pack applies, so a catalog slug and the file
// it grew from answer at the same address.
type ImageReader interface {
	Image(ctx context.Context, slug string) (imageasset.Image, error)
}

// ImageHandler serves seeded spell artwork to anyone. It is deliberately a
// different handler from the generation one: generating costs money and is a
// development route behind a session, while these bytes are public seed data
// the browser asks for from bare <img> tags.
type ImageHandler struct {
	images ImageReader
}

// NewImages builds the public artwork handler over the seed store.
func NewImages(images ImageReader) *ImageHandler {
	return &ImageHandler{images: images}
}

// Get serves GET /v1/spell-icons/*slug, where the wildcard is the seed slug
// plus its .webp suffix: /v1/spell-icons/dnd-2014/acid-splash.webp asks for
// "dnd-2014/acid-splash".
//
// The ETag is the seed revision -- the SHA256 the store derives from the
// bytes -- so a regenerated icon always carries a different one. Cache-Control
// is no-cache, not immutable and not no-store: no-cache lets the browser hold
// the bytes and revalidate them (304 costs a header, not the body), which is
// what makes a replaced icon appear on refresh without re-downloading every
// unchanged one.
func (h *ImageHandler) Get(c *gin.Context) {
	slug, hasSuffix := strings.CutSuffix(strings.TrimPrefix(c.Param("slug"), "/"), ".webp")
	// An addressable icon always ends in .webp. A slug that does not -- or
	// that resolves to nothing -- is a 404 through the API error envelope,
	// never the SPA fallback a static file would have hit.
	if slug == "" || !hasSuffix {
		helpers.FormatError(c, types.NewNotFoundError("spell icon not found"))
		return
	}
	img, err := h.images.Image(c.Request.Context(), slug)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	etag := `"` + img.Revision + `"`
	c.Header("ETag", etag)
	c.Header("Cache-Control", "no-cache")
	if ifNoneMatch(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return
	}
	contentType := img.ContentType
	if contentType == "" {
		contentType = "image/webp"
	}
	c.Data(http.StatusOK, contentType, img.Data)
}

// ifNoneMatch reports whether the header's entity tags match etag. It honours
// the list form (clients may keep several) and the wildcard, and folds weak
// validators: this endpoint only ever sends strong tags, but a cache
// comparing weakly is still telling the truth about the bytes.
func ifNoneMatch(header, etag string) bool {
	for tag := range strings.SplitSeq(header, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "*" || strings.TrimPrefix(tag, "W/") == etag {
			return true
		}
	}
	return false
}
