package character

import (
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
)

// Upgrade the metadata of older AI imports on read without renumbering their
// log or turning ordinary answers into unrestricted observations.
func normalizeImportLog(log domain.Log) domain.Log {
	if !slices.ContainsFunc(log.Events, func(e domain.Event) bool { return strings.HasPrefix(e.Note, "import.session:") }) {
		return log
	}
	out := log.Clone()
	for i := range out.Events {
		e := &out.Events[i]
		if e.Source == domain.PromptGroupNone && len(e.Choices) == 0 && requiredRef(*e) {
			e.Observed = true
			e.Source = observedGroup(e.Type, "")
		}
		if e.Type == domain.EventNote && e.Custom == nil && strings.HasPrefix(e.Note, "import.manual:") {
			lines := strings.SplitN(e.Note, "\n", 3)
			if len(lines) >= 2 {
				kind, name, ok := strings.Cut(lines[1], ": ")
				id := strings.TrimPrefix(lines[0], "import.manual:")
				if ok && customID.MatchString(id) {
					switch kind {
					case "class", "race", "subrace", "subclass", "background", "spell", "cantrip", "item", "feat", "feature", "trait", "note":
						description := ""
						if len(lines) == 3 {
							description = lines[2]
						}
						e.Custom = &domain.CustomOption{ID: id, Kind: kind, Name: name, Description: description, Selected: false}
					}
				}
			}
		}
		if e.Type == domain.EventInit {
			e.Source = domain.GroupIdentity
		}
	}
	return out
}
func observedAssociation(log domain.Log, cat *catalog.Catalog, e domain.Event) bool {
	if e.Type != domain.EventSubclass && e.Type != domain.EventSubrace {
		return true
	}
	state, err := domain.Project(log, cat)
	if err != nil {
		return false
	}
	if e.Type == domain.EventSubrace {
		sub, ok := cat.Subraces.Get(e.Ref.Slug)
		return ok && sub.Race == state.Identity.Race
	}
	sub, ok := cat.Subclasses.Get(e.Ref.Slug)
	if !ok {
		return false
	}
	return slices.ContainsFunc(state.Identity.Classes, func(c domain.ClassLevel) bool { return c.Class == sub.Class })
}
