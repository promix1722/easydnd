package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	groupapi "github.com/promix1722/easydnd/internal/api/http/v1/group"
)

// createGroup makes a group and returns it, failing the test on anything else.
func createGroup(t *testing.T, r *gin.Engine, session *http.Cookie, name string) groupapi.Group {
	t.Helper()
	rec := send(t, r, session, http.MethodPost, "/v1/groups", map[string]any{"name": name})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/groups = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	return decode[groupapi.Group](t, rec)
}

// inviteToken mints a link for a group at the given role.
func inviteToken(
	t *testing.T, r *gin.Engine, session *http.Cookie, id, role string,
) string {
	t.Helper()
	rec := send(t, r, session, http.MethodPost, "/v1/groups/"+id+"/invites",
		map[string]any{"role": role})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST invites = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	return decode[groupapi.Invite](t, rec).Token
}

// The whole feature, end to end over HTTP: one person makes a table, another
// joins it by link, the owner hands it on, and the ex-owner walks away.
func TestAGroupFromCreationToHandover(t *testing.T) {
	r, owner, ceremony := newFullRouterWithCeremony(t)
	created := createGroup(t, r, owner, "Wednesday Night")

	if created.Role != "owner" {
		t.Errorf("creator's role = %q, want %q", created.Role, "owner")
	}
	if len(created.Members) != 1 {
		t.Fatalf("a new group has %d members, want 1", len(created.Members))
	}
	if created.Members[0].DisplayName == "" {
		t.Error("the roster does not name its only member")
	}

	// A second account joins with the link.
	ceremony.credentialID = "second-credential"
	joiner := register(t, r, helpers.CookieOptions{Secure: false})
	token := inviteToken(t, r, owner, created.ID, "player")

	// They can read the invitation before committing to it.
	rec := send(t, r, joiner, http.MethodPost, "/v1/invites/preview", map[string]any{"token": token})
	if rec.Code != http.StatusOK {
		t.Fatalf("preview = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	preview := decode[groupapi.Preview](t, rec)
	if preview.GroupName != "Wednesday Night" || preview.Role != "player" {
		t.Errorf("preview = %+v, want the group name and the player role", preview)
	}
	if preview.AlreadyMember {
		t.Error("a stranger previews as already a member")
	}

	rec = send(t, r, joiner, http.MethodPost, "/v1/invites/accept", map[string]any{"token": token})
	if rec.Code != http.StatusOK {
		t.Fatalf("accept = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	joined := decode[groupapi.Group](t, rec)
	if joined.Role != "player" || len(joined.Members) != 2 {
		t.Fatalf("after joining: role %q, %d members; want player and 2",
			joined.Role, len(joined.Members))
	}

	// The link is reusable, so redeeming it twice is a success and a no-op.
	rec = send(t, r, joiner, http.MethodPost, "/v1/invites/accept", map[string]any{"token": token})
	if rec.Code != http.StatusOK {
		t.Errorf("accepting twice = %d, want 200", rec.Code)
	}

	// A player may not rename, invite or remove.
	rec = send(t, r, joiner, http.MethodPatch, "/v1/groups/"+created.ID,
		map[string]any{"name": "Mine now"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("a player renaming = %d, want 403", rec.Code)
	}

	// The owner promotes them, hands the group over, and leaves.
	target := "/v1/groups/" + created.ID + "/members?user=" + joined.Members[1].UserID
	rec = send(t, r, owner, http.MethodPatch, target, map[string]any{"role": "owner"})
	if rec.Code != http.StatusOK {
		t.Fatalf("transfer = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	after := decode[groupapi.Group](t, rec)
	if after.Role != "dm" {
		t.Errorf("the outgoing owner is now %q, want %q", after.Role, "dm")
	}

	self := "/v1/groups/" + created.ID + "/members?user=" + created.Members[0].UserID
	if rec := send(t, r, owner, http.MethodDelete, self, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("the ex-owner could not leave: %d (%s)", rec.Code, rec.Body.String())
	}

	// And the group is gone from their list, and unreachable to them.
	listed := decode[groupapi.ListResponse](t,
		send(t, r, owner, http.MethodGet, "/v1/groups", nil))
	if len(listed.Groups) != 0 {
		t.Errorf("the ex-owner still lists %d groups, want 0", len(listed.Groups))
	}
	rec = send(t, r, owner, http.MethodGet, "/v1/groups/"+created.ID, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("reading a group after leaving = %d, want 404", rec.Code)
	}
}

// An owner cannot walk away from their own group. The refusal is a 403 and not
// a 404: they are standing in the group, so there is nothing to hide, and the
// message has to be able to tell them what to do instead.
func TestTheOwnerCannotLeaveOverHTTP(t *testing.T) {
	r, owner := newFullRouter(t)
	created := createGroup(t, r, owner, "Wednesday Night")

	self := "/v1/groups/" + created.ID + "/members?user=" + created.Members[0].UserID
	rec := send(t, r, owner, http.MethodDelete, self, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("the owner leaving = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "access_denied" {
		t.Errorf("error code = %q, want %q", code, "access_denied")
	}

	// Deleting is the other way out, so nobody is ever trapped.
	if rec := send(t, r, owner, http.MethodDelete, "/v1/groups/"+created.ID, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("Delete = %d, want 204", rec.Code)
	}
}

// A group is several people's. Somebody who is not in it must not be able to
// read it, change it or learn that it exists -- which is why every answer here
// is 404 and never 403, exactly as for a character.
func TestAnotherAccountCannotReachTheGroup(t *testing.T) {
	r, owner, ceremony := newFullRouterWithCeremony(t)
	created := createGroup(t, r, owner, "Wednesday Night")

	ceremony.credentialID = "second-credential"
	intruder := register(t, r, helpers.CookieOptions{Secure: false})
	member := created.Members[0].UserID

	for _, route := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/v1/groups/" + created.ID, nil},
		{http.MethodPatch, "/v1/groups/" + created.ID, map[string]any{"name": "Theirs"}},
		{http.MethodDelete, "/v1/groups/" + created.ID, nil},
		{http.MethodPost, "/v1/groups/" + created.ID + "/invites", map[string]any{"role": "dm"}},
		{
			http.MethodPatch, "/v1/groups/" + created.ID + "/members?user=" + member,
			map[string]any{"role": "player"},
		},
		{http.MethodDelete, "/v1/groups/" + created.ID + "/members?user=" + member, nil},
	} {
		rec := send(t, r, intruder, route.method, route.path, route.body)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s as a non-member = %d, want 404",
				route.method, route.path, rec.Code)
		}
	}

	// Their own list is empty and the group survived untouched.
	listed := decode[groupapi.ListResponse](t,
		send(t, r, intruder, http.MethodGet, "/v1/groups", nil))
	if len(listed.Groups) != 0 {
		t.Errorf("a non-member lists %d groups, want 0", len(listed.Groups))
	}
	still := decode[groupapi.Group](t,
		send(t, r, owner, http.MethodGet, "/v1/groups/"+created.ID, nil))
	if still.Name != "Wednesday Night" || len(still.Members) != 1 {
		t.Errorf("the group changed under a non-member's requests: %+v", still)
	}
}

// A stale or forged link must not sign anybody out.
//
// Every token failure is an UnauthenticatedError inside the adapter, and a 401
// is what tells this application's client that the session has gone. If that
// leaked out here, clicking yesterday's invitation would drop the perfectly
// signed-in person who clicked it back onto the landing page.
func TestABadInviteLinkDoesNotSignYouOut(t *testing.T) {
	r, session := newFullRouter(t)

	for _, path := range []string{"/v1/invites/preview", "/v1/invites/accept"} {
		rec := send(t, r, session, http.MethodPost, path,
			map[string]any{"token": "not-a-real-token"})
		if rec.Code == http.StatusUnauthorized {
			t.Fatalf("%s with a bad token = 401, which signs the caller out", path)
		}
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s with a bad token = %d, want 400 (%s)", path, rec.Code, rec.Body.String())
		}
	}
}

// A guest is a full participant: they can make a table and be named in
// somebody else's roster, which is the only reason a users row is written for
// them at all.
func TestAGuestCanKeepAGroup(t *testing.T) {
	r, _ := newFullRouter(t)
	session := guest(t, r, helpers.CookieOptions{Secure: false})

	created := createGroup(t, r, session, "Guest's table")
	if created.Role != "owner" {
		t.Errorf("guest's role = %q, want %q", created.Role, "owner")
	}
	if len(created.Members) != 1 {
		t.Fatalf("roster has %d members, want 1", len(created.Members))
	}
	if !created.Members[0].Anonymous {
		t.Error("the guest is not marked anonymous in their own roster")
	}
	// The name is the one the session carries, not the bare "Guest" constant
	// -- three guests in one roster have to be tellable apart.
	if created.Members[0].DisplayName == "" {
		t.Error("the guest has no display name in the roster")
	}
}
