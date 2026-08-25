package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
	folderapi "github.com/promix1722/easydnd/internal/api/http/v1/folder"
)

// newCharacter is the body every test here creates a character with. The
// numbers are the standard array; nothing in this file depends on them.
func newCharacter(name, folder string) map[string]any {
	body := map[string]any{
		"name":   name,
		"method": "standard_array",
		"abilities": map[string]int{
			"str": 10, "dex": 15, "con": 13, "int": 12, "wis": 14, "cha": 8,
		},
	}
	if folder != "" {
		body["folder"] = folder
	}
	return body
}

func listFolders(t *testing.T, r *gin.Engine, session *http.Cookie) []folderapi.Folder {
	t.Helper()
	rec := send(t, r, session, http.MethodGet, "/v1/folders", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/folders = %d: %s", rec.Code, rec.Body)
	}
	return decode[folderapi.ListResponse](t, rec).Folders
}

// The promise the feature rests on: an account that has done nothing already
// has somewhere to put a character.
func TestANewAccountAlreadyHasADefaultFolder(t *testing.T) {
	r, session := newFullRouter(t)

	folders := listFolders(t, r, session)
	if len(folders) != 1 {
		t.Fatalf("folders = %+v, want exactly one", folders)
	}
	if !folders[0].Default {
		t.Errorf("the only folder is not the default: %+v", folders[0])
	}
	if folders[0].Name == "" {
		t.Error("the default folder has no name")
	}
}

func TestACharacterLandsInTheDefaultFolder(t *testing.T) {
	r, session := newFullRouter(t)

	def := listFolders(t, r, session)[0]

	rec := send(t, r, session, http.MethodPost, "/v1/characters", newCharacter("Ada", ""))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body)
	}

	listed := decode[characterapi.ListResponse](t,
		send(t, r, session, http.MethodGet, "/v1/characters", nil)).Characters
	if len(listed) != 1 {
		t.Fatalf("characters = %+v, want one", listed)
	}
	if listed[0].Folder != def.ID {
		t.Errorf("character folder = %q, want the default %q", listed[0].Folder, def.ID)
	}
}

func TestCreateFilterMoveAndCopyAcrossFolders(t *testing.T) {
	r, session := newFullRouter(t)

	def := listFolders(t, r, session)[0]

	rec := send(t, r, session, http.MethodPost, "/v1/folders", map[string]any{"name": "Campaign"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create folder = %d: %s", rec.Code, rec.Body)
	}
	campaign := decode[folderapi.Folder](t, rec)
	if campaign.Default {
		t.Error("a created folder claims to be the default")
	}

	// One character in each folder.
	inDefault := decode[characterapi.CreateResponse](t,
		send(t, r, session, http.MethodPost, "/v1/characters", newCharacter("Ada", "")))
	inCampaign := decode[characterapi.CreateResponse](t,
		send(t, r, session, http.MethodPost, "/v1/characters", newCharacter("Bram", campaign.ID)))

	// The filter narrows.
	only := decode[characterapi.ListResponse](t,
		send(t, r, session, http.MethodGet, "/v1/characters?folder="+campaign.ID, nil)).Characters
	if len(only) != 1 || only[0].ID != inCampaign.ID {
		t.Fatalf("?folder=campaign = %+v, want just %q", only, inCampaign.ID)
	}

	// Moving.
	move := send(t, r, session, http.MethodPut, "/v1/characters/"+inDefault.ID+"/folder",
		map[string]any{"folder": campaign.ID})
	if move.Code != http.StatusNoContent {
		t.Fatalf("move = %d: %s", move.Code, move.Body)
	}
	both := decode[characterapi.ListResponse](t,
		send(t, r, session, http.MethodGet, "/v1/characters?folder="+campaign.ID, nil)).Characters
	if len(both) != 2 {
		t.Fatalf("after the move ?folder=campaign = %+v, want two", both)
	}
	if left := decode[characterapi.ListResponse](t,
		send(t, r, session, http.MethodGet, "/v1/characters?folder="+def.ID, nil)).Characters; len(left) != 0 {
		t.Errorf("the default folder still holds %+v", left)
	}

	// Copying, into a named folder.
	copyRec := send(t, r, session, http.MethodPost, "/v1/characters/"+inCampaign.ID+"/copy",
		map[string]any{"folder": def.ID})
	if copyRec.Code != http.StatusCreated {
		t.Fatalf("copy = %d: %s", copyRec.Code, copyRec.Body)
	}
	copied := decode[characterapi.CreateResponse](t, copyRec)
	if copied.ID == inCampaign.ID {
		t.Fatal("copy returned the original's id")
	}
	if got := copied.Sheet.Identity.Name; got != "Bram (copy)" {
		t.Errorf("copy name = %q, want \"Bram (copy)\"", got)
	}

	inDefaultNow := decode[characterapi.ListResponse](t,
		send(t, r, session, http.MethodGet, "/v1/characters?folder="+def.ID, nil)).Characters
	if len(inDefaultNow) != 1 || inDefaultNow[0].ID != copied.ID {
		t.Errorf("the copy did not land in the default folder: %+v", inDefaultNow)
	}

	// The original is untouched: copying must not rename what it copied.
	source := decode[characterapi.Sheet](t,
		send(t, r, session, http.MethodGet, "/v1/characters/"+inCampaign.ID+"/sheet", nil))
	if source.Identity.Name != "Bram" {
		t.Errorf("source name = %q after being copied, want Bram", source.Identity.Name)
	}
}

// Copying with no body at all: the common case is a Copy button on a row, and
// it should not have to send `{}`.
func TestCopyWithNoBodyLandsBesideTheOriginal(t *testing.T) {
	r, session := newFullRouter(t)

	folder := decode[folderapi.Folder](t,
		send(t, r, session, http.MethodPost, "/v1/folders", map[string]any{"name": "Campaign"}))
	created := decode[characterapi.CreateResponse](t,
		send(t, r, session, http.MethodPost, "/v1/characters", newCharacter("Ada", folder.ID)))

	rec := send(t, r, session, http.MethodPost, "/v1/characters/"+created.ID+"/copy", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("copy = %d: %s", rec.Code, rec.Body)
	}

	listed := decode[characterapi.ListResponse](t,
		send(t, r, session, http.MethodGet, "/v1/characters?folder="+folder.ID, nil)).Characters
	if len(listed) != 2 {
		t.Fatalf("folder holds %+v, want the original and its copy", listed)
	}
}

func TestRenamingAFolderIncludingTheDefault(t *testing.T) {
	r, session := newFullRouter(t)

	def := listFolders(t, r, session)[0]

	rec := send(t, r, session, http.MethodPatch, "/v1/folders/"+def.ID,
		map[string]any{"name": "Active"})
	if rec.Code != http.StatusOK {
		t.Fatalf("rename = %d: %s", rec.Code, rec.Body)
	}
	renamed := decode[folderapi.Folder](t, rec)
	if renamed.Name != "Active" {
		t.Errorf("renamed name = %q, want Active", renamed.Name)
	}
	// Renaming must not cost the account the folder it can never delete.
	if !renamed.Default {
		t.Error("renaming the default folder cleared its default flag")
	}
}

func TestCreatingAFolderWithNoNameIsAFieldError(t *testing.T) {
	r, session := newFullRouter(t)

	rec := send(t, r, session, http.MethodPost, "/v1/folders", map[string]any{"name": "  "})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body)
	}
	if got := errorCode(t, rec); got != "field_validation_error" {
		t.Errorf("error code = %q, want field_validation_error", got)
	}
	if !strings.Contains(rec.Body.String(), `"name"`) {
		t.Errorf("the error does not name the field: %s", rec.Body)
	}
}

// The destructive one, end to end. A deleted folder takes its characters with
// it, and nothing gives them back.
func TestDeletingAFolderDeletesItsCharacters(t *testing.T) {
	r, session := newFullRouter(t)

	folder := decode[folderapi.Folder](t,
		send(t, r, session, http.MethodPost, "/v1/folders", map[string]any{"name": "Campaign"}))
	doomed := decode[characterapi.CreateResponse](t,
		send(t, r, session, http.MethodPost, "/v1/characters", newCharacter("Ada", folder.ID)))
	kept := decode[characterapi.CreateResponse](t,
		send(t, r, session, http.MethodPost, "/v1/characters", newCharacter("Bram", "")))

	rec := send(t, r, session, http.MethodDelete, "/v1/folders/"+folder.ID, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete folder = %d: %s", rec.Code, rec.Body)
	}

	if got := send(t, r, session, http.MethodGet, "/v1/characters/"+doomed.ID, nil); got.Code != http.StatusNotFound {
		t.Errorf("the deleted folder's character = %d, want 404", got.Code)
	}
	if got := send(t, r, session, http.MethodGet, "/v1/characters/"+kept.ID, nil); got.Code != http.StatusOK {
		t.Errorf("a character in another folder = %d, want 200", got.Code)
	}
	if folders := listFolders(t, r, session); len(folders) != 1 {
		t.Errorf("folders = %+v, want just the default", folders)
	}
}

// A 400, not a 404: the folder exists and the caller owns it. The honest answer
// is that this particular folder cannot go.
func TestTheDefaultFolderCannotBeDeleted(t *testing.T) {
	r, session := newFullRouter(t)

	def := listFolders(t, r, session)[0]

	rec := send(t, r, session, http.MethodDelete, "/v1/folders/"+def.ID, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("delete default = %d: %s", rec.Code, rec.Body)
	}
	if got := errorCode(t, rec); got != "validation_error" {
		t.Errorf("error code = %q, want validation_error", got)
	}
	if folders := listFolders(t, r, session); len(folders) != 1 {
		t.Errorf("folders = %+v, want the default still there", folders)
	}
}

// Every folder route is behind RequireSession, exactly as the character routes
// are. This is the test that notices one declared a line above the guarded
// group.
func TestFolderRoutesRequireASession(t *testing.T) {
	r, _ := newFullRouter(t)

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/folders"},
		{http.MethodPost, "/v1/folders"},
		{http.MethodPatch, "/v1/folders/fld_000001"},
		{http.MethodDelete, "/v1/folders/fld_000001"},
		{http.MethodPut, "/v1/characters/chr_000001/folder"},
		{http.MethodPost, "/v1/characters/chr_000001/copy"},
	} {
		rec := send(t, r, nil, tc.method, tc.path, map[string]any{"name": "x"})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without a session = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}

// The whole authorization model, applied to the second aggregate: somebody
// else's folder is indistinguishable from one that never existed. A 403 would
// confirm the id, and folders are numbered from one.
func TestAnotherAccountCannotReachTheFolder(t *testing.T) {
	r, alice, _, federation := newFullRouterWithFederation(t)
	cookies := helpers.CookieOptions{Secure: false}

	folder := decode[folderapi.Folder](t,
		send(t, r, alice, http.MethodPost, "/v1/folders", map[string]any{"name": "Campaign"}))
	character := decode[characterapi.CreateResponse](t,
		send(t, r, alice, http.MethodPost, "/v1/characters", newCharacter("Ada", folder.ID)))

	bob := signInWithGoogle(t, r, cookies, federation, "google-bob")

	// Bob's own listing is his own default and nothing else.
	bobsFolders := listFolders(t, r, bob)
	if len(bobsFolders) != 1 || bobsFolders[0].ID == folder.ID {
		t.Fatalf("another account's folders = %+v, want only their own default", bobsFolders)
	}

	for _, tc := range []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
	}{
		{"list into it", http.MethodGet, "/v1/characters?folder=" + folder.ID, nil, http.StatusNotFound},
		{"rename it", http.MethodPatch, "/v1/folders/" + folder.ID, map[string]any{"name": "Mine"}, http.StatusNotFound},
		{"delete it", http.MethodDelete, "/v1/folders/" + folder.ID, nil, http.StatusNotFound},
		{"copy from it", http.MethodPost, "/v1/characters/" + character.ID + "/copy", nil, http.StatusNotFound},
	} {
		if rec := send(t, r, bob, tc.method, tc.path, tc.body); rec.Code != tc.wantStatus {
			t.Errorf("%s = %d, want %d: %s", tc.name, rec.Code, tc.wantStatus, rec.Body)
		}
	}

	// And the move that would otherwise make one of Bob's own characters
	// vanish into Alice's folder.
	bobs := decode[characterapi.CreateResponse](t,
		send(t, r, bob, http.MethodPost, "/v1/characters", newCharacter("Bram", "")))
	rec := send(t, r, bob, http.MethodPut, "/v1/characters/"+bobs.ID+"/folder",
		map[string]any{"folder": folder.ID})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("moving into another account's folder = %d, want 404: %s", rec.Code, rec.Body)
	}
	still := decode[characterapi.ListResponse](t,
		send(t, r, bob, http.MethodGet, "/v1/characters", nil)).Characters
	if len(still) != 1 || still[0].ID != bobs.ID {
		t.Errorf("the refused move lost the character: %+v", still)
	}
}
