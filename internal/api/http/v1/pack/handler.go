package pack

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/api/http/middleware"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	packuc "github.com/promix1722/easydnd/internal/usecase/pack"
)

type Handler struct {
	service *packuc.Service
	source  catalog.Source
}

func New(s *packuc.Service, source catalog.Source) *Handler { return &Handler{s, source} }
func actor(c *gin.Context) user.User                        { u, _ := middleware.UserFrom(c); return u }

type Release struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

func releaseOf(r domain.Release) Release { return Release{r.ID, r.Version, r.Digest} }

type Record struct {
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	Owned    bool            `json:"owned"`
	Builtin  bool            `json:"builtin"`
	Archived bool            `json:"archived"`
	Revision int             `json:"revision"`
	Draft    json.RawMessage `json:"draft,omitempty"`
	Releases []Release       `json:"releases"`
}

func recordOf(r domain.Record, u user.ID, detail bool) Record {
	out := Record{ID: r.ID, Title: r.Title, Owned: r.Owner == u, Builtin: r.Owner == "", Archived: r.Archived, Revision: r.Revision, Releases: []Release{}}
	if detail {
		out.Draft = r.Draft
	}
	for _, d := range r.Releases {
		out.Releases = append(out.Releases, releaseOf(d.Release))
	}
	return out
}
func respond(c *gin.Context, v any, err error) {
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}
func (h *Handler) List(c *gin.Context) {
	rows, err := h.service.List(c.Request.Context(), actor(c).ID)
	out := []Record{}
	for _, r := range rows {
		out = append(out, recordOf(r, actor(c).ID, false))
	}
	respond(c, gin.H{"packs": out, "defaultRules": helpers.RulesLockOf(h.service.Default())}, err)
}
func (h *Handler) Get(c *gin.Context) {
	r, err := h.service.Get(c.Request.Context(), actor(c).ID, c.Param("id"))
	respond(c, recordOf(r, actor(c).ID, true), err)
}
func (h *Handler) Schema(c *gin.Context) { c.Data(200, gin.MIMEJSON, h.service.Schema()) }

type draftParams struct {
	Title            string            `json:"title"`
	Document         json.RawMessage   `json:"document"`
	ExpectedRevision int               `json:"expectedRevision"`
	Mappings         map[string]string `json:"mappings"`
}

func bind(c *gin.Context, p any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<20)
	if err := c.ShouldBindJSON(p); err != nil {
		helpers.FormatError(c, err)
		return false
	}
	return true
}
func (h *Handler) Create(c *gin.Context) {
	var p draftParams
	if !bind(c, &p) {
		return
	}
	r, err := h.service.Create(c.Request.Context(), actor(c), p.Title, p.Document, p.Mappings)
	respond(c, recordOf(r, actor(c).ID, true), err)
}
func (h *Handler) Import(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<20)
	b, err := io.ReadAll(c.Request.Body)
	if err != nil {
		helpers.FormatError(c, types.NewValidationError("invalid upload").Because("pack.invalid"))
		return
	}
	title := c.Query("title")
	r, err := h.service.Create(c.Request.Context(), actor(c), title, b, nil)
	respond(c, recordOf(r, actor(c).ID, true), err)
}
func (h *Handler) Save(c *gin.Context) {
	var p draftParams
	if !bind(c, &p) {
		return
	}
	r, err := h.service.Save(c.Request.Context(), actor(c).ID, c.Param("id"), p.Title, p.ExpectedRevision, p.Document, p.Mappings)
	respond(c, recordOf(r, actor(c).ID, true), err)
}
func (h *Handler) Validate(c *gin.Context) {
	_, l, err := h.service.Validate(c.Request.Context(), actor(c).ID, c.Param("id"))
	if err != nil {
		if types.IsNotFound(err) {
			helpers.FormatError(c, err)
			return
		}
		d := packuc.Diagnose(err)
		c.JSON(200, gin.H{"valid": false, "diagnostics": []gin.H{{"reason": d.Reason, "path": d.Path, "detail": d.Detail, "args": d.Args}}})
		return
	}
	c.JSON(200, gin.H{"valid": true, "rules": helpers.RulesLockOf(l), "diagnostics": []any{}})
}
func (h *Handler) Publish(c *gin.Context) {
	var p draftParams
	if !bind(c, &p) {
		return
	}
	r, err := h.service.Publish(c.Request.Context(), actor(c).ID, c.Param("id"), p.ExpectedRevision)
	respond(c, recordOf(r, actor(c).ID, true), err)
}
func (h *Handler) Archive(c *gin.Context) {
	var p draftParams
	if !bind(c, &p) {
		return
	}
	r, err := h.service.Archive(c.Request.Context(), actor(c).ID, c.Param("id"), p.ExpectedRevision)
	respond(c, recordOf(r, actor(c).ID, true), err)
}
func (h *Handler) Export(c *gin.Context) {
	b, err := h.service.Export(c.Request.Context(), actor(c).ID, c.Param("id"), c.Query("version"))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="pack.json"`)
	c.Data(200, gin.MIMEJSON, b)
}
func rootsOf(in []Release) []domain.Release {
	out := []domain.Release{}
	for _, p := range in {
		out = append(out, domain.Release{ID: p.ID, Version: p.Version, Digest: p.Digest})
	}
	return out
}
func (h *Handler) Resolve(c *gin.Context) {
	var p struct {
		Packs []Release `json:"packs"`
	}
	if !bind(c, &p) {
		return
	}
	l, err := h.service.Resolve(c.Request.Context(), actor(c).ID, rootsOf(p.Packs))
	respond(c, helpers.RulesLockOf(l), err)
}

// Catalogue requests resolve and authorize on every call, including cache hits.
func (h *Handler) Catalog(c *gin.Context) {
	roots := []domain.Release{}
	for _, item := range strings.Split(c.Query("packs"), ",") {
		id, version, ok := strings.Cut(item, "@")
		if !ok {
			helpers.FormatError(c, types.NewValidationError("invalid selection").Because("pack.invalid"))
			return
		}
		roots = append(roots, domain.Release{ID: id, Version: version})
	}
	l, err := h.service.Resolve(c.Request.Context(), actor(c).ID, roots)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	cat, err := catalog.LoadLocked(c.Request.Context(), h.source, helpers.Locale(c), l)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	if c.Param("collection") == "" {
		catalogapi.ServeManifest(c, cat, slog.Default())
		return
	}
	catalogapi.ServeCollection(c, cat)
}
func (h *Handler) GroupList(c *gin.Context) {
	rows, err := h.service.GroupShares(c.Request.Context(), actor(c).ID, c.Param("id"))
	out := []gin.H{}
	for _, r := range rows {
		out = append(out, gin.H{"pack": r.Pack, "contributor": r.Contributor, "rules": helpers.RulesLockOf(r.Lock)})
	}
	respond(c, out, err)
}
func (h *Handler) Share(c *gin.Context) {
	var p struct {
		Pack    string `json:"pack"`
		Version string `json:"version"`
	}
	if !bind(c, &p) {
		return
	}
	respond(c, gin.H{}, h.service.Share(c.Request.Context(), actor(c).ID, c.Param("id"), p.Pack, p.Version))
}
func (h *Handler) Unshare(c *gin.Context) {
	respond(c, gin.H{}, h.service.Unshare(c.Request.Context(), actor(c).ID, c.Param("id"), c.Query("pack")))
}
