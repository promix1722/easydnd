package character

import (
	"context"
	"path/filepath"
	"testing"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// answersAnOpenPrompt is the predicate the whole feature rests on -- it is
// both the gate on an append and the source of an entry's group -- so it is
// tested directly rather than only through what it lets through.
//
// The catalogue is the committed one. A stub would prove only that the code
// agrees with itself, and every case below is about a shape the SRD actually
// has: a race chosen from a collection, a subrace listed inline, a prompt
// that hangs off an entry already held.

// A second Source, because this file is the internal test package and
// service_test.go's is the external one -- same binary, different packages, so
// the var cannot be shared. Two loads of the compendium rather than one is
// still two rather than the fifty this package used to do.
var internalCatalogSource = catalogfile.NewSource(filepath.Join("..", "..", "..", "data", "srd_5.1"))

func loadCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := internalCatalogSource.Load(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatalf("loading the compendium: %v", err)
	}
	return cat
}

// logFrom builds a log the domain will accept, without going near a service.
func logFrom(t *testing.T, events ...domain.Event) domain.Log {
	t.Helper()
	seeded := append([]domain.Event{{
		Type: domain.EventInit,
		Changes: []domain.Change{
			{Path: "identity.name", Op: domain.OpSet, Value: domain.StringValue("Rurik")},
		},
	}}, events...)
	log, err := domain.Rebuild(seeded)
	if err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	return log
}

func TestAnswersAnOpenPrompt(t *testing.T) {
	cat := loadCatalog(t)

	race := func(slug rules.Slug) domain.Event {
		return domain.Event{Type: domain.EventRace, Ref: rules.NewRef(rules.RefRace, slug)}
	}

	tests := []struct {
		name  string
		log   domain.Log
		event domain.Event
		want  bool
		group domain.PromptGroup
	}{
		{
			name:  "a race, which the character is being asked for",
			log:   logFrom(t),
			event: race("half-elf"),
			want:  true, group: domain.GroupRace,
		},
		{
			name: "a subrace the chosen race lists",
			log:  logFrom(t, race("dwarf")),
			event: domain.Event{
				Type: domain.EventSubrace, Ref: rules.NewRef(rules.RefSubrace, "hill-dwarf"),
			},
			want: true, group: domain.GroupRace,
		},
		{
			// The bug. The reference resolves; nothing asked for it.
			name: "a subrace belonging to another race",
			log:  logFrom(t, race("half-elf")),
			event: domain.Event{
				Type: domain.EventSubrace, Ref: rules.NewRef(rules.RefSubrace, "hill-dwarf"),
			},
			want: false,
		},
		{
			name:  "a second race, when one is already chosen",
			log:   logFrom(t, race("half-elf")),
			event: race("elf"),
			want:  false,
		},
		{
			// The shape that keeps a race's follow-up entries alive: the
			// prompt hangs off the race, and states the ref to post it with.
			name:  "a follow-up entry naming the race it hangs off",
			log:   logFrom(t, race("half-elf")),
			event: race("half-elf"),
			want:  true, group: domain.GroupRace,
		},
		{
			name: "the first class",
			log:  logFrom(t, race("half-elf")),
			event: domain.Event{
				Type: domain.EventClass, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 1,
			},
			want: true, group: domain.GroupClass,
		},
		{
			name: "a level in a class the character does not have",
			log:  logFrom(t, race("half-elf")),
			event: domain.Event{
				Type: domain.EventLevel, Ref: rules.NewRef(rules.RefClass, "fighter"), Level: 3,
			},
			want: false,
		},
		{
			name: "a background, drawn from its collection",
			log:  logFrom(t),
			event: domain.Event{
				Type: domain.EventBackground, Ref: rules.NewRef(rules.RefBackground, "acolyte"),
			},
			want: true, group: domain.GroupBackground,
		},
		{
			// A feat event is legal in the model and offered by nothing: the
			// Ability Score Improvement's feat branch is answered as a level
			// event, so there is no prompt that says "post a feat event".
			name:  "a feat, which no prompt offers",
			log:   logFrom(t, race("half-elf")),
			event: domain.Event{Type: domain.EventFeat, Ref: rules.NewRef(rules.RefFeat, "grappler")},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			open, err := domain.Prompts(tt.log, cat)
			if err != nil {
				t.Fatalf("Prompts() error = %v", err)
			}
			prompt, ok := answersAnOpenPrompt(open, tt.event)
			if ok != tt.want {
				t.Fatalf("answersAnOpenPrompt() = %v, want %v", ok, tt.want)
			}
			if ok && prompt.Group != tt.group {
				t.Errorf("group = %s, want %s", prompt.Group, tt.group)
			}
		})
	}
}

// offers is where the kind is checked as well as the slug, so a prompt
// offering races cannot be answered with a class that happens to share one.
func TestOffersChecksTheKindAndNotOnlyTheSlug(t *testing.T) {
	inline := rules.OptionSet{Kind: rules.OptionsExplicit, Options: []rules.Option{
		rules.RefOption{Ref: rules.NewRef(rules.RefSubrace, "hill-dwarf"), Count: 1},
	}}
	if !offers(inline, rules.NewRef(rules.RefSubrace, "hill-dwarf")) {
		t.Error("offers() missed an option it lists")
	}
	if offers(inline, rules.NewRef(rules.RefRace, "hill-dwarf")) {
		t.Error("offers() accepted the right slug under the wrong kind")
	}

	collection := rules.OptionSet{Kind: rules.OptionsFromCollection, Collection: rules.RefRace}
	if !offers(collection, rules.NewRef(rules.RefRace, "elf")) {
		t.Error("offers() rejected an entry of the collection it draws from")
	}
	if offers(collection, rules.NewRef(rules.RefClass, "rogue")) {
		t.Error("offers() accepted a class from a set of races")
	}
}
