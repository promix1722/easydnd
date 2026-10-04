package character

import (
	"encoding/json"
	"fmt"
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
	s, err := h.agent.Create(c.Request.Context(), h.owner(c), folderOf(c), helpers.Locale(c), files, c.PostForm("instructions"), selected.Domain())
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
	s, err := h.agent.Get(h.owner(c), c.Param("id"))
	h.agentResult(c, s, err)
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

// Every event has a durable-within-process cursor. Reconnect starts with a
// coherent snapshot; no missed interval exists between snapshot and polling.
func (h *Handler) AgentEvents(c *gin.Context) {
	if !h.agentAvailable(c) {
		return
	}
	s, err := h.agent.Get(h.owner(c), c.Param("id"))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-store")
	c.Header("X-Accel-Buffering", "no")
	cursor, _ := strconv.Atoi(c.GetHeader("Last-Event-ID"))
	if cursor < 0 || cursor > len(s.Events) {
		cursor = 0
	}
	revision := -1
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()
	send := func(event string, id int, data any) bool {
		b, _ := json.Marshal(data)
		_, err := fmt.Fprintf(c.Writer, "id: %d\nevent: %s\ndata: %s\n\n", id, event, b)
		c.Writer.Flush()
		return err == nil
	}
	for {
		if revision != s.Revision {
			if !send("snapshot", len(s.Events), gin.H{"session": s}) {
				return
			}
			cursor = len(s.Events)
			revision = s.Revision
		}
		for _, e := range s.Events {
			if e.ID > cursor {
				if !send("update", e.ID, e) {
					return
				}
				cursor = e.ID
			}
		}
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(c.Writer, ": heartbeat\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		case <-ticker.C:
		}
		s, err = h.agent.Get(h.owner(c), c.Param("id"))
		if err != nil {
			return
		}
	}
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
