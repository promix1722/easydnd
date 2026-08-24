package webauthn

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

func newTestCeremony(t *testing.T) *Ceremony {
	t.Helper()
	c, err := New(Config{
		RPID:            "localhost",
		RPDisplayName:   "easydnd",
		RPOrigins:       []string{"http://localhost:5173"},
		CeremonyTimeout: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// The whole sign-in design rests on the credential being discoverable: with no
// username to offer, a non-resident key could never be found again. Both
// spellings must go out -- the modern one and the one older authenticators
// read.
func TestRegistrationOptionsDemandADiscoverableCredential(t *testing.T) {
	options, state, err := newTestCeremony(t).BeginRegistration(
		user.User{ID: "abc", DisplayName: "Alice"})
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	if len(state) == 0 {
		t.Fatal("BeginRegistration produced no ceremony state")
	}

	var body struct {
		PublicKey struct {
			Challenge              string `json:"challenge"`
			Attestation            string `json:"attestation"`
			AuthenticatorSelection struct {
				ResidentKey        string `json:"residentKey"`
				RequireResidentKey *bool  `json:"requireResidentKey"`
			} `json:"authenticatorSelection"`
			User struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
			} `json:"user"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(options, &body); err != nil {
		t.Fatalf("decode options %s: %v", options, err)
	}

	if body.PublicKey.AuthenticatorSelection.ResidentKey != "required" {
		t.Errorf("residentKey = %q, want required", body.PublicKey.AuthenticatorSelection.ResidentKey)
	}
	if body.PublicKey.AuthenticatorSelection.RequireResidentKey == nil ||
		!*body.PublicKey.AuthenticatorSelection.RequireResidentKey {
		t.Error("requireResidentKey is not set; a pre-residentKey authenticator would make a non-discoverable credential")
	}
	// We have no use for a device identifier, so we do not ask for one.
	if body.PublicKey.Attestation != "none" {
		t.Errorf("attestation = %q, want none", body.PublicKey.Attestation)
	}
	if body.PublicKey.Challenge == "" {
		t.Error("options carry no challenge")
	}
	if body.PublicKey.User.DisplayName != "Alice" {
		t.Errorf("displayName = %q, want Alice", body.PublicKey.User.DisplayName)
	}
}

// A sign-in that named an account would leak which accounts exist, and would
// need a username to name one with. It does neither.
func TestLoginOptionsNameNoAccount(t *testing.T) {
	options, state, err := newTestCeremony(t).BeginLogin()
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	if len(state) == 0 {
		t.Fatal("BeginLogin produced no ceremony state")
	}

	var body struct {
		PublicKey struct {
			Challenge        string `json:"challenge"`
			AllowCredentials []any  `json:"allowCredentials"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(options, &body); err != nil {
		t.Fatalf("decode options %s: %v", options, err)
	}
	if len(body.PublicKey.AllowCredentials) != 0 {
		t.Errorf("allowCredentials has %d entries; sign-in must not enumerate credentials",
			len(body.PublicKey.AllowCredentials))
	}
	if body.PublicKey.Challenge == "" {
		t.Error("options carry no challenge")
	}
}

// Registering a second passkey on an authenticator that already holds one
// should be refused by the browser with a clear message, not quietly duplicated.
func TestRegistrationExcludesExistingCredentials(t *testing.T) {
	options, _, err := newTestCeremony(t).BeginRegistration(user.User{
		ID:          "abc",
		DisplayName: "Alice",
		Credentials: []user.Credential{{ID: []byte("already-registered")}},
	})
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}

	var body struct {
		PublicKey struct {
			ExcludeCredentials []struct {
				ID string `json:"id"`
			} `json:"excludeCredentials"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(options, &body); err != nil {
		t.Fatalf("decode options: %v", err)
	}
	if len(body.PublicKey.ExcludeCredentials) != 1 {
		t.Fatalf("excludeCredentials has %d entries, want 1", len(body.PublicKey.ExcludeCredentials))
	}
}

// Garbage from a client is ordinary input, not a server fault: it must
// classify as a 400, and it must never panic.
func TestFinishRejectsMalformedInputWithoutPanicking(t *testing.T) {
	c := newTestCeremony(t)
	_, state, err := c.BeginRegistration(user.User{ID: "abc", DisplayName: "Alice"})
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}

	for _, body := range [][]byte{
		[]byte(""),
		[]byte("not json"),
		[]byte(`{"id":"truncated"}`),
		[]byte(`{"id":"x","response":{"attestationObject":"!!!not-base64!!!"}}`),
	} {
		_, err := c.FinishRegistration(user.User{ID: "abc"}, state, body)
		if err == nil {
			t.Errorf("FinishRegistration(%q) succeeded", body)
			continue
		}
		var validation *types.ValidationError
		if !errors.As(err, &validation) {
			t.Errorf("FinishRegistration(%q) err = %v, want *types.ValidationError", body, err)
		}
	}
}

// Ceremony state travels through a sealed cookie as opaque bytes, so it has to
// survive an encode/decode round trip unchanged.
func TestCeremonyStateSurvivesARoundTrip(t *testing.T) {
	c := newTestCeremony(t)
	_, state, err := c.BeginLogin()
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}

	session, err := unmarshalState(state)
	if err != nil {
		t.Fatalf("unmarshalState: %v", err)
	}
	if len(session.Challenge) == 0 {
		t.Fatal("the decoded state carries no challenge")
	}

	again, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	round, err := unmarshalState(again)
	if err != nil {
		t.Fatalf("unmarshalState (second): %v", err)
	}
	if string(round.Challenge) != string(session.Challenge) {
		t.Error("the challenge changed across a round trip")
	}
}

func TestUnmarshalStateRejectsGarbage(t *testing.T) {
	if _, err := unmarshalState([]byte("not json")); err == nil {
		t.Fatal("unmarshalState accepted garbage")
	}
}

// New must reject a relying party it cannot verify against, rather than
// failing later inside every ceremony.
func TestNewRejectsAnEmptyRelyingParty(t *testing.T) {
	if _, err := New(Config{RPDisplayName: "easydnd", CeremonyTimeout: time.Minute}); err == nil {
		t.Fatal("New accepted a configuration with no RP id")
	}
}
