package character

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

func (h *Handler) AgentCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"enabled": h.agent != nil && h.agent.Enabled()})
}
func (h *Handler) agentAvailable(c *gin.Context) bool {
	if h.agent == nil || !h.agent.Enabled() {
		helpers.FormatError(c, types.NewNotImplementedError("agent disabled").Because("agent.disabled"))
		return false
	}
	return true
}

// agentPollWait is how long AgentGet holds an up-to-date reader. One second is
// far inside every timeout between the browser and this handler, so a held
// poll needs no proxy configuration, heartbeat or deadline override.
const agentPollWait = time.Second

func (h *Handler) agentResult(c *gin.Context, s charuc.AgentSession, err error) {
	c.Header("Cache-Control", "no-store")
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"session": s})
}
func (h *Handler) AgentCreate(c *gin.Context) {
	if !h.agentAvailable(c) {
		return
	}
	files, ok := readAgentFiles(c)
	if !ok {
		return
	}
	var selected helpers.RulesLock
	if err := json.Unmarshal([]byte(c.PostForm("rules")), &selected); err != nil || selected.Domain().Validate() != nil {
		helpers.FormatError(c, types.NewValidationError("select rules before starting").Because("agent.rulesRequired"))
		return
	}
	create := h.agent.Create
	if c.PostForm("unattended") == "true" {
		create = h.agent.CreateUnattended
	}
	s, err := create(c.Request.Context(), h.owner(c), folderOf(c), helpers.Locale(c), files, c.PostForm("instructions"), selected.Domain())
	h.agentResult(c, s, err)
}
func (h *Handler) AgentList(c *gin.Context) {
	if !h.agentAvailable(c) {
		return
	}
	c.JSON(http.StatusOK, h.agent.List(h.owner(c)))
}
func (h *Handler) AgentGet(c *gin.Context) {
	if !h.agentAvailable(c) {
		return
	}
	raw, polling := c.GetQuery("revision")
	if !polling {
		s, err := h.agent.Get(h.owner(c), c.Param("id"))
		h.agentResult(c, s, err)
		return
	}
	// Long polling: the reader says what it holds and is answered when that is
	// out of date, or with 204 after agentPollWait. See docs/long-polling.md.
	revision, _ := strconv.Atoi(raw)
	after, _ := strconv.Atoi(c.Query("after"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), agentPollWait)
	defer cancel()
	s, full, changed, err := h.agent.Wait(ctx, h.owner(c), c.Param("id"), revision, after)
	switch {
	case err != nil || full:
		h.agentResult(c, s, err)
	case changed:
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"events": s.Events})
	default:
		c.Header("Cache-Control", "no-store")
		c.Status(http.StatusNoContent)
	}
}
func (h *Handler) AgentControl(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	if !h.agentAvailable(c) {
		return
	}
	var p struct {
		Action   string `json:"action"`
		Text     string `json:"text"`
		Revision int    `json:"revision"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		helpers.FormatError(c, types.NewValidationError("invalid control"))
		return
	}
	s, err := h.agent.Control(h.owner(c), c.Param("id"), p.Action, p.Text, p.Revision)
	h.agentResult(c, s, err)
}

func readAgentFiles(c *gin.Context) ([]charuc.AgentFile, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 21<<20)
	if err := c.Request.ParseMultipartForm(21 << 20); err != nil {
		helpers.FormatError(c, types.NewValidationError("invalid attachments").Because("agent.files"))
		return nil, false
	}
	defer c.Request.MultipartForm.RemoveAll()
	files := []charuc.AgentFile{}
	for _, part := range c.Request.MultipartForm.File["files"] {
		f, err := part.Open()
		if err != nil {
			helpers.FormatError(c, err)
			return nil, false
		}
		b, err := io.ReadAll(io.LimitReader(f, 20<<20+1))
		_ = f.Close()
		if err != nil {
			helpers.FormatError(c, err)
			return nil, false
		}
		mime := http.DetectContentType(b)
		switch {
		case strings.HasPrefix(mime, "text/plain"):
			if !utf8.Valid(b) {
				helpers.FormatError(c, types.NewValidationError("invalid text").Because("agent.files"))
				return nil, false
			}
			mime = "text/plain"
			if json.Valid(b) {
				mime = "application/json"
			}
		case mime == "application/pdf", mime == "image/png", mime == "image/jpeg", mime == "image/webp":
		default:
			helpers.FormatError(c, types.NewValidationError("unsupported attachment").Because("agent.files"))
			return nil, false
		}
		if (mime == "text/plain" || mime == "application/json") && len(b) > 256<<10 {
			helpers.FormatError(c, types.NewValidationError("text attachment too large").Because("agent.files"))
			return nil, false
		}
		files = append(files, charuc.AgentFile{Name: part.Filename, MIME: mime, Data: b})
	}

	return files, true
}

func (h *Handler) AgentFiles(c *gin.Context) {
	if !h.agentAvailable(c) {
		return
	}
	// Authorize before buffering attachment bodies.
	if _, err := h.agent.Get(h.owner(c), c.Param("id")); err != nil {
		helpers.FormatError(c, err)
		return
	}
	files, ok := readAgentFiles(c)
	if !ok {
		return
	}
	revision, err := strconv.Atoi(c.PostForm("revision"))
	if err != nil {
		helpers.FormatError(c, types.NewValidationError("revision required"))
		return
	}
	s, err := h.agent.AddFiles(h.owner(c), c.Param("id"), revision, files, c.PostForm("instructions"))
	h.agentResult(c, s, err)
}
