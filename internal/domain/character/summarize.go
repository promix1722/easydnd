package character

import (
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// Summarize folds a log into the short form a listing needs.
//
// It exists because Summary carries a name, a level and a class line, none of
// which are stored: they are projections. Running a full Project for every
// character in a list would resolve the whole compendium per row to render
// three fields, so this reads only what a listing shows.
//
// It takes a catalogue for one reason -- a class line reads better with the
// subclass attached, and matching a subclass to its class needs the
// compendium. Everything else here comes from the log alone, which is why an
// unresolvable race or a missing class does not make a character unlistable
// the way it would make it unprojectable.
func Summarize(id ID, owner OwnerID, log Log, cat *catalog.Catalog) Summary {
	s := Summary{ID: id, Owner: owner}
	for _, e := range log.Events {
		switch e.Type {
		case EventInit, EventChange:
			for _, ch := range e.Changes {
				if ch.Path == "identity.name" && ch.Op == OpSet {
					if ch.Value.Kind == ValueString {
						s.Name = ch.Value.Str
					}
				}
			}
		case EventClass:
			s.Classes = takeClassLevel(s.Classes, e.Ref.Slug, max(e.Level, 1))
		case EventLevel:
			s.Classes = takeClassLevel(s.Classes, e.Ref.Slug, e.Level)
		case EventSubclass:
			s.Classes = attachSubclass(s.Classes, e.Ref.Slug, cat)
		case EventRace, EventSubrace, EventBackground, EventFeat, EventNote, EventNone:
		}
	}
	for _, c := range s.Classes {
		s.Level += c.Level
	}
	return s
}

// takeClassLevel records a level in a class, keeping the highest seen rather
// than accumulating, so replaying a log twice cannot inflate it.
func takeClassLevel(classes []ClassLevel, class rules.Slug, level int) []ClassLevel {
	if class.IsZero() || level < 1 {
		return classes
	}
	for i, c := range classes {
		if c.Class == class {
			if level > c.Level {
				classes[i].Level = level
			}
			return classes
		}
	}
	return append(classes, ClassLevel{Class: class, Level: level})
}

func attachSubclass(classes []ClassLevel, subclass rules.Slug, cat *catalog.Catalog) []ClassLevel {
	if cat == nil {
		return classes
	}
	entry, ok := cat.Subclasses.Get(subclass)
	if !ok {
		return classes
	}
	for i, c := range classes {
		if c.Class == entry.Class {
			classes[i].Subclass = subclass
			return classes
		}
	}
	return classes
}
