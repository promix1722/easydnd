package pack

import (
	"regexp"
	"strings"

	"github.com/promix1722/easydnd/internal/types"
)

// Diagnostic supplies a stable translation key plus a document location. Details
// remain available to authors inspecting compiler output, never as HTTP errors.
type Diagnostic struct {
	Reason, Path, Detail string
	Args                 types.Args
}

var defaultName = regexp.MustCompile(`missing default name for ([^/]+)/(.+)$`)

func Diagnose(err error) Diagnostic {
	d := Diagnostic{Reason: "pack.invalid", Path: "/mechanics", Detail: err.Error()}
	text := err.Error()
	switch {
	case strings.HasPrefix(text, "no installed release satisfies "):
		parts := strings.SplitN(strings.TrimPrefix(text, "no installed release satisfies "), " ", 2)
		d.Reason = "pack.missingDependency"
		d.Path = "/manifest/dependencies"
		d.Args = types.Args{"pack": parts[0]}
		if len(parts) > 1 {
			d.Args["version"] = parts[1]
		}
	case strings.Contains(text, "multiple core rule providers"):
		d.Reason = "pack.coreConflict"
		d.Path = "/mechanics/core"
	case strings.Contains(text, "no core policy"):
		d.Reason = "pack.coreMissing"
		d.Path = "/manifest/dependencies"
	case strings.Contains(text, "incompatible rules editions"):
		d.Reason = "pack.edition"
		d.Path = "/manifest/edition"
	case strings.Contains(text, "pack version"):
		d.Reason = "pack.version"
		d.Path = "/manifest/version"
	case strings.Contains(text, "dependency cycle"):
		d.Reason = "pack.cycle"
		d.Path = "/manifest/dependencies"
	case strings.Contains(text, "missing default locale"):
		d.Reason = "pack.locale"
		d.Path = "/locales"
	case defaultName.MatchString(text):
		parts := defaultName.FindStringSubmatch(text)
		d.Reason = "pack.nameMissing"
		d.Path = "/locales"
		d.Args = types.Args{"entry": parts[1] + "/" + parts[2]}
	}
	return d
}
