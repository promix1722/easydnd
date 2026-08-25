package game

import (
	"time"

	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
	"github.com/promix1722/easydnd/internal/domain/character"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	gameuc "github.com/promix1722/easydnd/internal/usecase/game"
)

// gameOfDomain renders a game with the caller's rank and its roster.
func gameOfDomain(g domain.Game, role group.Role, roster []character.Summary) Game {
	out := Game{
		ID:         string(g.ID),
		GroupID:    string(g.Group),
		Name:       g.Name,
		CreatedAt:  g.CreatedAt.UTC().Format(time.RFC3339),
		Role:       string(role),
		Characters: charactersOf(roster),
	}
	return out
}

// summaryOf renders one row of the games list, with the table it sits at.
func summaryOf(a gameuc.At) Summary {
	return Summary{
		ID:        string(a.Game.ID),
		GroupID:   string(a.Game.Group),
		GroupName: a.GroupName,
		Name:      a.Game.Name,
		CreatedAt: a.Game.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// charactersOf renders a set of character summaries.
func charactersOf(in []character.Summary) []Character {
	out := make([]Character, 0, len(in))
	for _, c := range in {
		out = append(out, characterOf(c))
	}
	return out
}

// characterOf renders one character summary.
func characterOf(c character.Summary) Character {
	classes := make([]characterapi.ClassLevel, 0, len(c.Classes))
	for _, cl := range c.Classes {
		classes = append(classes, characterapi.ClassLevel{
			Class: cl.Class.String(), Subclass: cl.Subclass.String(), Level: cl.Level,
		})
	}
	return Character{
		ID:      string(c.ID),
		OwnerID: string(c.Owner),
		Name:    c.Name,
		Level:   c.Level,
		Classes: classes,
	}
}
