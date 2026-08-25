package game

import (
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
)

// Game is one game and its whole roster.
type Game struct {
	ID        string `json:"id"`
	GroupID   string `json:"group_id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`

	// Role is the *caller's* rank in the group this game sits at, not
	// anybody else's. It is here for the reason the group DTO carries one:
	// every screen has to decide which controls to draw, and without it each
	// of them would fetch the group separately to find out.
	Role string `json:"role"`

	Characters []Character `json:"characters"`
}

// Summary is one row of the games list. It carries no roster: the list draws
// names, and loading every roster to render them would be a fold per row.
//
// It does carry the group's name. The list spans every table the caller sits
// at, so "Thursday night" on its own would not say which of them it belongs
// to -- and the alternative, a request per row to find out, is the thing a
// summary exists to avoid.
type Summary struct {
	ID        string `json:"id"`
	GroupID   string `json:"group_id"`
	GroupName string `json:"group_name"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// Character is one character on a table or a roster, in the short form a
// listing needs.
//
// It mirrors the character resource's own summary rather than inventing a
// second shape, so that a client already rendering a character list can render
// this one with the same code. The folder is deliberately absent: it is the
// owner's private filing and says nothing to the rest of the table.
type Character struct {
	ID      string `json:"id"`
	OwnerID string `json:"owner_id"`
	Name    string `json:"name"`
	Level   int    `json:"level"`

	// Classes uses the character resource's own ClassLevel rather than a
	// second shape saying the same thing, so a client already rendering a
	// class line renders this one with the same code.
	Classes []characterapi.ClassLevel `json:"classes,omitempty"`
}

// ListResponse is the body of GET /v1/groups/:id/games.
type ListResponse struct {
	Games []Summary `json:"games"`
}

// TableResponse is the body of GET /v1/groups/:id/characters.
type TableResponse struct {
	Characters []Character `json:"characters"`
}
