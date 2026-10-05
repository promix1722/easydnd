package portrait

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	_ "golang.org/x/image/webp"

	"github.com/promix1722/easydnd/internal/types"
)

// ponytail: inline portraits add up in rosters; use asset storage if payload size matters.
func Valid(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 256<<10 {
		return false
	}
	header, encoded, ok := strings.Cut(value, ",")
	if !ok {
		return false
	}
	formats := map[string]string{"data:image/jpeg;base64": "jpeg", "data:image/png;base64": "png", "data:image/webp;base64": "webp"}
	format, ok := formats[header]
	if !ok {
		return false
	}
	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return false
	}
	config, detected, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || detected != format || config.Width < 1 || config.Height < 1 || config.Width > 256 || config.Height > 256 {
		return false
	}
	_, _, err = image.Decode(bytes.NewReader(data))
	return err == nil
}

func FieldError() types.FieldError {
	return types.FieldError{Field: "image", Rule: "invalid", Reason: "field.character.image.invalid"}
}
