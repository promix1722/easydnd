// Package spellicon serves the development spell-icon generator: a queue the
// spells screen's dev tooling starts and polls. It exists only on the
// development route table -- the router never registers it in production --
// and every method sits behind the session guard and the same-origin check
// the /v1 group already carries.
package spellicon

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/api/http/middleware"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	spelliconuc "github.com/promix1722/easydnd/internal/usecase/spellicon"
)

// The request is a scope string and the same filter object the spells
// screen's search box produces, so nothing in it can grow large. The cap is
// far above what a real filter needs; it exists so the endpoint does not have
// to trust a body it never measured.
const maxRequestBytes = 256 << 10

// scopeCatalogPath is the one scoped catalogue URL the client may name. The
// backend allowlists it rather than proxying an arbitrary path: anything else
// is rejected before it can reach the resolver.
const scopeCatalogPath = "/packs/catalog"

// maxScopePacks bounds one scoped selection. Nobody selects more than a
// handful of packs in the browser; the bound is to stop a body, not to
// constrain a user.
const maxScopePacks = 64

// maxSlugs bounds one explicit resume list. The whole SRD is under 400
// spells and the 256KiB body cap already limits the strings, so 2000 leaves
// generous headroom without trusting an unbounded array.
const maxSlugs = 2000

// A present resume list must never turn into an unrestricted paid batch.
type explicitSlugs []string

func (s *explicitSlugs) UnmarshalJSON(data []byte) error {
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	if len(values) == 0 {
		return invalidRequest("slugs must name at least one spell")
	}
	*s = values
	return nil
}

// slugSegment mirrors the pack codec's local-id rule
// (internal/adapter/catalog/file) plus a segment cap: a slug names a file
// once the store picks it up, so it is validated here rather than trusted to
// stay inside the catalogue's own alphabet.
var slugSegment = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,119}$`)

// Handler is the transport half of the icon queue: it decodes the screen's
// request into a Selection and hands the paid work to the service. All of the
// interesting behaviour -- authorization, catalogue access, serialization of
// the run -- lives one layer in.
type Handler struct {
	service  *spelliconuc.Service
	selector *spelliconuc.Selector
}

func New(service *spelliconuc.Service, selector *spelliconuc.Selector) *Handler {
	return &Handler{service: service, selector: selector}
}

// The screen's search box, transported in the request body. These are exactly
// the parameters catalogapi.ParseSpellSearch validates on the catalogue
// routes; POST reuses that validation by re-laying them out as a query.
type searchBody struct {
	Pack          string `json:"pack"`
	Source        string `json:"source"`
	Versions      string `json:"versions"`
	Query         string `json:"q"`
	Level         *int   `json:"level"`
	School        string `json:"school"`
	Class         string `json:"class"`
	CastingTime   string `json:"castingTime"`
	Concentration *bool  `json:"concentration"`
	Ritual        *bool  `json:"ritual"`
	Material      *bool  `json:"material"`
	// Limit and Offset are accepted and validated like on the catalogue
	// route, then deliberately dropped: the run always covers every match.
	Limit  *int `json:"limit"`
	Offset *int `json:"offset"`
}

// query lays the decoded filter back out as query parameters so the catalogue
// package's own parser -- and its field-level error reasons -- applies
// unchanged. Only the fields that were sent travel, because ParseSpellSearch
// distinguishes "absent" from "present and odd".
func (s searchBody) query() url.Values {
	q := url.Values{}
	set := func(key, value string) {
		if value != "" {
			q.Set(key, value)
		}
	}
	set("pack", s.Pack)
	set("source", s.Source)
	set(catalogapi.ParamQuery, s.Query)
	set(catalogapi.ParamSchool, s.School)
	set(catalogapi.ParamClass, s.Class)
	set(catalogapi.ParamCastingTime, s.CastingTime)
	if s.Level != nil {
		q.Set(catalogapi.ParamLevel, strconv.Itoa(*s.Level))
	}
	put := func(key string, value *bool) {
		if value != nil {
			q.Set(key, strconv.FormatBool(*value))
		}
	}
	put(catalogapi.ParamConcentration, s.Concentration)
	put(catalogapi.ParamRitual, s.Ritual)
	put(catalogapi.ParamMaterial, s.Material)
	if s.Limit != nil {
		q.Set(catalogapi.ParamLimit, strconv.Itoa(*s.Limit))
	}
	if s.Offset != nil {
		q.Set(catalogapi.ParamOffset, strconv.Itoa(*s.Offset))
	}
	return q
}

// parseFilter runs the body's search object through the catalogue's own spell
// search parser. The parser reads the query string off a request, so the
// decoded fields are laid onto a bare probe request rather than
// re-implemented field by field -- duplicating its limits here would only
// fork the rules it enforces.
func parseFilter(search searchBody) (catalog.SpellFilter, error) {
	probe := &gin.Context{Request: &http.Request{
		Method: http.MethodGet,
		URL:    &url.URL{Path: "/v1/packs/spells", RawQuery: search.query().Encode()},
		Header: http.Header{},
	}}
	filter, _, _, err := catalogapi.ParseSpellSearch(probe)
	return filter, err
}

// actor resolves the session RequireSession parked. The route cannot be
// reached without it, so a miss means wiring, not a credential problem -- but
// refuse rather than trust it.
func actor(c *gin.Context) (user.User, bool) {
	return middleware.UserFrom(c)
}

// Get reports the caller's queue state. Per-owner by design: the items map
// can carry private pack names, so nobody reads another account's run.
func (h *Handler) Get(c *gin.Context) {
	who, ok := actor(c)
	if !ok {
		helpers.FormatError(c, types.NewUnauthenticatedError("no session").Because("auth.noSession"))
		return
	}
	c.JSON(http.StatusOK, h.service.Status(who.ID))
}

// Post decodes the screen's scope and filters into a Selection, resolves the
// spells it names, and starts the run. The response is the queue as accepted;
// the artwork itself is still being produced when the answer arrives.
func (h *Handler) Post(c *gin.Context) {
	who, ok := actor(c)
	if !ok {
		helpers.FormatError(c, types.NewUnauthenticatedError("no session").Because("auth.noSession"))
		return
	}
	var body struct {
		Scope   *string       `json:"scope"`
		Search  *searchBody   `json:"search"`
		Slug    *string       `json:"slug"`
		Slugs   explicitSlugs `json:"slugs"`
		Replace *bool         `json:"replace"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBytes)
	if err := c.ShouldBindJSON(&body); err != nil {
		if errors.Is(err, io.EOF) {
			helpers.FormatError(c, invalidRequest("empty request body"))
			return
		}
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			helpers.FormatError(c, invalidRequest("request body too large"))
			return
		}
		helpers.FormatError(c, err)
		return
	}
	if body.Scope == nil || body.Search == nil || body.Replace == nil {
		helpers.FormatError(c, invalidRequest("scope, search and replace are required"))
		return
	}
	slug := ""
	if body.Slug != nil {
		slug = *body.Slug
	}
	var slugs []string
	if body.Slugs != nil {
		switch {
		case body.Slug != nil:
			helpers.FormatError(c, invalidRequest("slug and slugs cannot be sent together"))
		case len(body.Slugs) > maxSlugs:
			helpers.FormatError(c, invalidRequest("too many slugs"))
		default:
			slugs = body.Slugs
		}
		if slugs == nil {
			return
		}
	}
	sel, err := h.selection(c, *body.Scope, *body.Search, slug, slugs)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	spells, err := h.selector.Select(c.Request.Context(), who.ID, sel)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	state, err := h.service.Start(c.Request.Context(), who.ID, spells, *body.Replace)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, state)
}

// fail writes the queue's refusals.
//
// Busy and not-configured are ValidationErrors underneath -- the usecase has
// no status vocabulary -- but the client's contract asks for a 409 and a 503,
// and a generic 400 would collapse "the paid run is already going" into the
// same shape as "your scope is malformed". The envelope keeps the shared code
// and reason so the error reads exactly like every other refusal; only the
// status differs.
func fail(c *gin.Context, err error) {
	var ve *types.ValidationError
	status := 0
	if errors.As(err, &ve) {
		switch ve.Reason {
		case spelliconuc.ReasonBusy:
			status = http.StatusConflict
		case spelliconuc.ReasonNotConfigured:
			status = http.StatusServiceUnavailable
		}
	}
	if status == 0 {
		helpers.FormatError(c, err)
		return
	}
	c.AbortWithStatusJSON(status, helpers.ErrorResponse{Error: helpers.ErrorBody{
		Code:      "validation_error",
		Reason:    ve.Reason,
		Args:      ve.Args,
		RequestID: c.GetString(helpers.ContextKeyRequestID),
	}})
}

// selection maps the wire scope to the usecase's Selection.
//
// Three shapes are legal, the same three the spells screen issues:
// "browse" for the every-pack view, "/packs/catalog?packs=id@version,…" for a
// pinned set of releases, and "" for the aggregate SRD compendium. Anything
// else -- another path, an extra parameter, a malformed selector -- is a
// validation failure rather than something to be lenient about: a silently
// broadened scope would spend money generating icons nobody asked for.
func (h *Handler) selection(c *gin.Context, scope string, search searchBody, slug string, slugs []string) (spelliconuc.Selection, error) {
	sel := spelliconuc.Selection{Locale: helpers.Locale(c).String()}
	filter, err := parseFilter(search)
	if err != nil {
		return sel, err
	}
	sel.Filter = filter
	if slug != "" {
		if !validSlug(slug) {
			return sel, types.NewValidationError("invalid spell slug").Because(spelliconuc.ReasonInvalidRequest)
		}
		sel.Slug = slug
	}
	if slugs != nil {
		// Every entry follows the same alphabet as the singular slug: the
		// usecase resolves them one catalogue at a time, but a value that
		// can never spell a catalogue key is refused here rather than sent
		// down to fail lookup.
		for _, item := range slugs {
			if !validSlug(item) {
				return sel, types.NewValidationError("invalid spell slug").Because(spelliconuc.ReasonInvalidRequest)
			}
		}
		sel.Slugs = slugs
	}
	switch {
	case scope == "browse":
		sel.Browse = true
		versions, err := parseVersions(search.Versions)
		if err != nil {
			return sel, err
		}
		sel.Versions = versions
	case strings.HasPrefix(scope, scopeCatalogPath+"?") || scope == scopeCatalogPath:
		u, err := url.Parse(scope)
		if err != nil {
			return sel, invalidScope()
		}
		query := u.Query()
		raw := query["packs"]
		if len(raw) != 1 || len(query) != 1 {
			return sel, invalidScope()
		}
		packs := strings.Split(raw[0], ",")
		if len(packs) > maxScopePacks {
			return sel, invalidScope()
		}
		for _, item := range packs {
			id, version, ok := strings.Cut(item, "@")
			if !ok || id == "" || version == "" {
				return sel, invalidScope()
			}
			sel.Packs = append(sel.Packs, domain.Release{ID: id, Version: version})
		}
	case scope == "":
		// The aggregate compendium: no pack list, everything the source loads.
	default:
		return sel, invalidScope()
	}
	return sel, nil
}

func invalidRequest(message string) error {
	return types.NewValidationError("%s", message).Because(spelliconuc.ReasonInvalidRequest)
}

func invalidScope() error {
	return types.NewValidationError("invalid scope").Because(spelliconuc.ReasonInvalidRequest)
}

// validSlug admits the qualified or bare slug of one catalogue spell. Slug
// segments follow the pack codec's own id alphabet, which contains neither
// "." nor "/", so traversal cannot be spelled.
func validSlug(slug string) bool {
	segments := strings.Split(slug, "/")
	if len(segments) < 1 || len(segments) > 4 {
		return false
	}
	for _, segment := range segments {
		if !slugSegment.MatchString(segment) {
			return false
		}
	}
	return true
}

// parseVersions reads the browse screen's release pinning: "id@version"
// pairs joined by commas, the same format the packs catalogue route takes as
// its ?versions= parameter. The catalogue handler's own rejection of a
// malformed pair and a duplicated pack is mirrored so the two endpoints agree
// on what the parameter means.
func parseVersions(raw string) (map[string]string, error) {
	overrides := map[string]string{}
	if raw == "" {
		return overrides, nil
	}
	for _, part := range strings.Split(raw, ",") {
		id, version, ok := strings.Cut(part, "@")
		if !ok || overrides[id] != "" {
			return nil, types.NewValidationError("invalid version selection").Because("pack.invalid")
		}
		overrides[id] = version
	}
	return overrides, nil
}
