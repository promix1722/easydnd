package group

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"unicode/utf8"

	domain "github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/types"
)

// groupIDPrefix tags a group id so that one seen in a log or a link is
// recognisable on sight. It is outside the base64url alphabet's meaning but
// inside its character set, which costs nothing and reads better than sixteen
// bytes of noise.
const groupIDPrefix = "grp_"

// validateName trims and checks a group name.
//
// The bounds match the CHECK constraint on groups.name, so nothing this
// accepts can be refused by the database -- a validation error belongs in a
// 400 with a field attached, and a constraint violation arrives as a 500 with
// nothing useful in it.
func validateName(name string) (string, error) {
	clean := strings.TrimSpace(name)
	switch {
	case clean == "":
		return "", types.NewFieldValidationError("a group needs a name",
			types.FieldError{Field: "name", Rule: "required"})
	case utf8.RuneCountInString(clean) > domain.MaxNameLen:
		return "", types.NewFieldValidationError("that group name is too long",
			types.FieldError{Field: "name", Rule: "max", Message: "at most 64 characters"})
	case !utf8.ValidString(clean):
		return "", types.NewFieldValidationError("that group name is not valid text",
			types.FieldError{Field: "name", Rule: "encoding"})
	}
	return clean, nil
}

// newGroupID mints an opaque group id.
//
// Unguessable rather than sequential, for the reason an account id is: a group
// id appears in invite links and in URLs people paste to each other, and a
// counter there would say how many groups exist and let a stranger address the
// next one.
func newGroupID() (domain.ID, error) {
	raw := make([]byte, groupIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", types.WrapServerError(err, "generate group id")
	}
	return domain.ID(groupIDPrefix + base64.RawURLEncoding.EncodeToString(raw)), nil
}
