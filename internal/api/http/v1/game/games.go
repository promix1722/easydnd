package game

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/domain/character"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/user"
)

// CreateParams is the body of POST /v1/games.
//
// The group is named in the body rather than in the path because a game is a
// top-level thing here: it is reached, listed and opened from its own section,
// and only its creation has to say which table it belongs to.
type CreateParams struct {
	GroupID string `json:"group_id"`
	Name    string `json:"name"`
}

// RenameParams is the body of PATCH /v1/games/:id.
type RenameParams struct {
	Name string `json:"name"`
}

// AddParams is the body of POST /v1/games/:id/characters.
//
// A list, always, and never a flag beside a single id. "Seat everyone at the
// table" is then the client posting the list already on its screen: one
// request shape whether it is one character or nine, and no second code path
// to keep in step with the first. An empty or absent list means everyone the
// group shares, which is the one thing the client cannot compute without
// racing whoever is sharing at that moment.
type AddParams struct {
	CharacterIDs []string `json:"character_ids"`
}

// Mine handles GET /v1/games: every game at every table the caller sits at.
//
// No group in the path and none required as a filter. A player at three tables
// wants one list, and making them name a table first would mean remembering
// where a game lives in order to find it.
func (h *Handler) Mine(c *gin.Context) {
	games, err := h.service.Mine(c.Request.Context(), h.actor(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	out := make([]Summary, 0, len(games))
	for _, g := range games {
		out = append(out, summaryOf(g))
	}
	c.JSON(http.StatusOK, ListResponse{Games: out})
}

// Create handles POST /v1/games. Owner or DM of the group it names.
func (h *Handler) Create(c *gin.Context) {
	var params CreateParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	ctx := c.Request.Context()
	actor := h.actor(c)
	created, err := h.service.Create(ctx, actor, group.ID(params.GroupID), params.Name)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	// Read it back rather than assembling an empty roster here, so the
	// response shape is produced by exactly one code path and cannot drift
	// between "just created" and "opened a moment later".
	h.detail(c, ctx, actor, created.ID, http.StatusCreated)
}

// Get handles GET /v1/games/:id. Any member of the group it sits at.
func (h *Handler) Get(c *gin.Context) {
	h.detail(c, c.Request.Context(), h.actor(c), gameOf(c), http.StatusOK)
}

// Rename handles PATCH /v1/games/:id. Owner or DM.
func (h *Handler) Rename(c *gin.Context) {
	var params RenameParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	ctx := c.Request.Context()
	actor := h.actor(c)
	id := gameOf(c)
	if _, err := h.service.Rename(ctx, actor, id, params.Name); err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.detail(c, ctx, actor, id, http.StatusOK)
}

// Delete handles DELETE /v1/games/:id. Owner or DM.
//
// It takes nothing with it. The characters were never the game's -- they
// belong to the players and stay on the group's table, which is the difference
// between ending a game and deleting a folder.
func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), h.actor(c), gameOf(c)); err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AddCharacters handles POST /v1/games/:id/characters. Owner or DM.
func (h *Handler) AddCharacters(c *gin.Context) {
	var params AddParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	ids := make([]character.ID, 0, len(params.CharacterIDs))
	for _, raw := range params.CharacterIDs {
		ids = append(ids, character.ID(raw))
	}

	ctx := c.Request.Context()
	actor := h.actor(c)
	id := gameOf(c)
	if err := h.service.AddCharacters(ctx, actor, id, ids); err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.detail(c, ctx, actor, id, http.StatusOK)
}

// RemoveCharacter handles DELETE /v1/games/:id/characters?character=.
//
// The character leaves this game and stays on the group's table: a seat is not
// a share, and giving one up is not taking the character back.
func (h *Handler) RemoveCharacter(c *gin.Context) {
	ctx := c.Request.Context()
	actor := h.actor(c)
	id := gameOf(c)
	if err := h.service.RemoveCharacter(ctx, actor, id, targetOf(c)); err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.detail(c, ctx, actor, id, http.StatusOK)
}

// detail writes a game and its roster, and is how every write here answers.
//
// One function rather than each handler assembling its own response: the
// shape a client gets back from creating, renaming and seating a character is
// then produced by one code path, which is what stops the three from drifting.
func (h *Handler) detail(
	c *gin.Context, ctx context.Context, actor user.ID, id domain.ID, status int,
) {
	g, role, roster, err := h.service.Get(ctx, actor, id, helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(status, gameOfDomain(g, role, roster))
}
