package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/promix1722/easydnd/internal/config"
)

// The stub route is the only one in the table that is not always there, so it
// is tested from both sides: that it builds a character in development, and
// that it does not exist at all in production. The second is the one that
// matters -- a development convenience reachable on easydnd.org would be a
// route nobody meant to ship.

func TestStubBuildsAFullCharacter(t *testing.T) {
	r, session := newFullRouter(t)

	created := post(t, r, "/v1/characters/stub", "", session)
	if created.Code != http.StatusCreated {
		t.Fatalf("stub status = %d: %s", created.Code, created.Body)
	}

	body := decode[struct {
		ID    string `json:"id"`
		Seq   int    `json:"seq"`
		Sheet struct {
			Identity struct {
				Name    string `json:"name"`
				Level   int    `json:"level"`
				Race    string `json:"race"`
				Classes []struct {
					Class    string `json:"class"`
					Subclass string `json:"subclass"`
					Level    int    `json:"level"`
				} `json:"classes"`
			} `json:"identity"`
			Status struct {
				ArmorClass       int `json:"armorClass"`
				ProficiencyBonus int `json:"proficiencyBonus"`
			} `json:"status"`
		} `json:"sheet"`
	}](t, created)

	if body.ID == "" {
		t.Error("stub returned no id")
	}
	// The opening entry plus the eight selections that build the character.
	if got, want := body.Seq, 9; got != want {
		t.Errorf("seq = %d, want %d", got, want)
	}
	if got, want := body.Sheet.Identity.Name, "Сахарок"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
	if got, want := body.Sheet.Identity.Level, 3; got != want {
		t.Errorf("level = %d, want %d", got, want)
	}
	if got, want := body.Sheet.Identity.Race, "half-elf"; got != want {
		t.Errorf("race = %q, want %q", got, want)
	}
	if len(body.Sheet.Identity.Classes) != 1 ||
		body.Sheet.Identity.Classes[0].Class != "rogue" ||
		body.Sheet.Identity.Classes[0].Subclass != "thief" {
		t.Errorf("classes = %+v, want one rogue/thief", body.Sheet.Identity.Classes)
	}
	if got, want := body.Sheet.Status.ArmorClass, 14; got != want {
		t.Errorf("armor class = %d, want %d", got, want)
	}
	if got, want := body.Sheet.Status.ProficiencyBonus, 2; got != want {
		t.Errorf("proficiency bonus = %d, want %d", got, want)
	}
}

// The whole of the gate: in production the endpoint does not exist, rather
// than existing and declining.
//
// The status is 405 rather than the 404 an unrouted path would give, and the
// reason is worth stating so that a future reader does not "fix" it. The path
// shape is already claimed by GET and DELETE /v1/characters/:id -- to gin,
// "stub" is a character id there -- so with the POST unregistered it reports
// the method as not allowed rather than the path as unknown. What matters is
// that no handler runs and no character is made; which of the two refusals
// gin picks is a routing detail.
func TestStubIsNotRegisteredInProduction(t *testing.T) {
	r, session, _, _ := newFullRouterInEnv(t, config.EnvProduction)

	got := post(t, r, "/v1/characters/stub", "", session)
	if got.Code != http.StatusMethodNotAllowed {
		t.Errorf("stub status in production = %d, want %d: %s",
			got.Code, http.StatusMethodNotAllowed, got.Body)
	}

	// Nothing was created: the refusal above is the route being absent, not a
	// handler running and failing quietly.
	listed := decode[struct {
		Characters []struct {
			ID string `json:"id"`
		} `json:"characters"`
	}](t, getWith(t, r, "/v1/characters", session))
	if len(listed.Characters) != 0 {
		t.Errorf("characters after a refused stub = %d, want 0", len(listed.Characters))
	}

	// The neighbouring route is unconditional, so this is the control: the
	// 404 above is the stub being absent, not the whole table failing to build.
	alive := post(t, r, "/v1/characters", `{"name":"Rurik"}`, session)
	if alive.Code != http.StatusCreated {
		t.Fatalf("create status in production = %d: %s", alive.Code, alive.Body)
	}
}

// The party list may be filtered when the button is pressed, and a stub that
// ignored the filter would land somewhere the player is not looking.
func TestStubFilesIntoTheNamedFolder(t *testing.T) {
	r, session := newFullRouter(t)

	folder := decode[struct {
		ID string `json:"id"`
	}](t, post(t, r, "/v1/folders", `{"name":"Stubs"}`, session))

	created := post(t, r, "/v1/characters/stub?folder="+folder.ID, "", session)
	if created.Code != http.StatusCreated {
		t.Fatalf("stub status = %d: %s", created.Code, created.Body)
	}

	listed := decode[struct {
		Characters []struct {
			ID     string `json:"id"`
			Folder string `json:"folder"`
		} `json:"characters"`
	}](t, getWith(t, r, "/v1/characters?folder="+folder.ID, session))

	if len(listed.Characters) != 1 {
		t.Fatalf("characters in folder = %d, want 1", len(listed.Characters))
	}
	if got, want := listed.Characters[0].Folder, folder.ID; got != want {
		t.Errorf("folder = %q, want %q", got, want)
	}
}
