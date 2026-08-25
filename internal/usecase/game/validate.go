package game

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"unicode/utf8"

	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/types"
)

// gameIDBytes is the entropy behind a game id, and gameIDPrefix tags it so
// that one seen in a log or a URL is recognisable on sight -- the same
// arrangement group ids use, and for the same reason: possession of the
// identifier is one half of reaching the thing.
const (
	gameIDBytes  = 16
	gameIDPrefix = "gam_"
)

// validateName trims and checks a game name.
//
// The bounds match the group's rather than a database constraint, because
// there is no table behind this one. They are still checked here so that the
// day a table arrives, nothing this accepts can be refused by it.
func validateName(name string) (string, error) {
	clean := strings.TrimSpace(name)
	switch {
	case clean == "":
		return "", types.NewFieldValidationError("a game needs a name",
			types.FieldError{Field: "name", Rule: "required", Message: "a game needs a name"})
	case utf8.RuneCountInString(clean) > domain.MaxNameLen:
		return "", types.NewFieldValidationError("that game name is too long",
			types.FieldError{Field: "name", Rule: "max", Message: "at most 64 characters"})
	case !utf8.ValidString(clean):
		return "", types.NewFieldValidationError("that game name is not valid text",
			types.FieldError{Field: "name", Rule: "encoding"})
	}
	return clean, nil
}

// newGameID mints an opaque game id.
//
// Unguessable rather than sequential, for the reason a group id is: it appears
// in URLs people paste to each other, and a counter there would say how many
// games exist and let a stranger address the next one. Note this is the
// usecase's job and not the store's -- the character store mints its own ids
// as a counter, which is exactly what must not happen to something reachable
// by link.
func newGameID() (domain.ID, error) {
	raw := make([]byte, gameIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", types.WrapServerError(err, "generate game id")
	}
	return domain.ID(gameIDPrefix + base64.RawURLEncoding.EncodeToString(raw)), nil
}
