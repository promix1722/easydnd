package webauthn

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"

	"github.com/promix1722/easydnd/internal/domain/user"
)

// This file drives the real ceremony end to end against a software
// authenticator: real ES256 keys, real signatures, real CBOR, verified by the
// real library. Everything else in the suite stubs one side or the other, so
// this is the only test that would notice if the adapter agreed with itself
// but not with the specification.

const (
	testRPID           = "localhost"
	testOrigin         = "http://localhost:5173"
	flagUserPresent    = 0x01
	flagUserVerified   = 0x04
	flagBackupEligible = 0x08
	flagBackupState    = 0x10
	flagAttestedData   = 0x40
)

// authenticator is a minimal CTAP2 authenticator: one key pair, one credential
// id, a counter it actually advances.
type authenticator struct {
	key       *ecdsa.PrivateKey
	credID    []byte
	aaguid    []byte
	signCount uint32
}

func newAuthenticator(t *testing.T) *authenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	credID := make([]byte, 32)
	if _, err := rand.Read(credID); err != nil {
		t.Fatalf("generate credential id: %v", err)
	}
	return &authenticator{key: key, credID: credID, aaguid: make([]byte, 16)}
}

// coseKey renders the public key the way the specification requires: a COSE
// EC2 key, which is what the relying party stores and verifies against later.
func (a *authenticator) coseKey(t *testing.T) []byte {
	t.Helper()
	x := make([]byte, 32)
	y := make([]byte, 32)
	a.key.PublicKey.X.FillBytes(x)
	a.key.PublicKey.Y.FillBytes(y)

	encoded, err := cbor.Marshal(map[int]any{
		1:  2,  // kty: EC2
		3:  -7, // alg: ES256
		-1: 1,  // crv: P-256
		-2: x,
		-3: y,
	})
	if err != nil {
		t.Fatalf("encode COSE key: %v", err)
	}
	return encoded
}

func (a *authenticator) authData(t *testing.T, flags byte, attested bool) []byte {
	t.Helper()
	rpIDHash := sha256.Sum256([]byte(testRPID))

	data := append([]byte{}, rpIDHash[:]...)
	data = append(data, flags)
	data = binary.BigEndian.AppendUint32(data, a.signCount)

	if attested {
		data = append(data, a.aaguid...)
		data = binary.BigEndian.AppendUint16(data, uint16(len(a.credID)))
		data = append(data, a.credID...)
		data = append(data, a.coseKey(t)...)
	}
	return data
}

func clientData(t *testing.T, ceremonyType, challenge string) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"type":        ceremonyType,
		"challenge":   challenge,
		"origin":      testOrigin,
		"crossOrigin": false,
	})
	if err != nil {
		t.Fatalf("encode client data: %v", err)
	}
	return encoded
}

// register produces the attestation response a browser would post.
func (a *authenticator) register(t *testing.T, options []byte) []byte {
	t.Helper()
	challenge := challengeFrom(t, options)

	client := clientData(t, "webauthn.create", challenge)
	// Backup flags set: this stands in for a passkey that syncs, which is what
	// the UI reads to decide how hard to nag about a second one.
	authData := a.authData(t,
		flagUserPresent|flagUserVerified|flagBackupEligible|flagBackupState|flagAttestedData, true)

	attestation, err := cbor.Marshal(map[string]any{
		"fmt":      "none",
		"attStmt":  map[string]any{},
		"authData": authData,
	})
	if err != nil {
		t.Fatalf("encode attestation object: %v", err)
	}

	return mustJSON(t, map[string]any{
		"id":                      base64.RawURLEncoding.EncodeToString(a.credID),
		"rawId":                   base64.RawURLEncoding.EncodeToString(a.credID),
		"type":                    "public-key",
		"clientExtensionResults":  map[string]any{},
		"authenticatorAttachment": "platform",
		"response": map[string]any{
			"clientDataJSON":    base64.RawURLEncoding.EncodeToString(client),
			"attestationObject": base64.RawURLEncoding.EncodeToString(attestation),
			"transports":        []string{"internal", "hybrid"},
		},
	})
}

// assert produces the assertion response a browser would post at sign-in.
func (a *authenticator) assert(t *testing.T, options []byte, handle user.ID) []byte {
	t.Helper()
	challenge := challengeFrom(t, options)

	// A real authenticator advances its counter; most platform ones do not.
	// Advancing here is what lets the sign-count write-back be observed.
	a.signCount++

	client := clientData(t, "webauthn.get", challenge)
	authData := a.authData(t,
		flagUserPresent|flagUserVerified|flagBackupEligible|flagBackupState, false)

	clientHash := sha256.Sum256(client)
	signed := append(append([]byte{}, authData...), clientHash[:]...)
	digest := sha256.Sum256(signed)

	signature, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		t.Fatalf("sign assertion: %v", err)
	}

	return mustJSON(t, map[string]any{
		"id":                     base64.RawURLEncoding.EncodeToString(a.credID),
		"rawId":                  base64.RawURLEncoding.EncodeToString(a.credID),
		"type":                   "public-key",
		"clientExtensionResults": map[string]any{},
		"response": map[string]any{
			"clientDataJSON":    base64.RawURLEncoding.EncodeToString(client),
			"authenticatorData": base64.RawURLEncoding.EncodeToString(authData),
			"signature":         base64.RawURLEncoding.EncodeToString(signature),
			// The handle is what makes a usernameless sign-in resolvable: the
			// authenticator returns it, nobody types it.
			"userHandle": base64.RawURLEncoding.EncodeToString([]byte(handle)),
		},
	})
}

func challengeFrom(t *testing.T, options []byte) string {
	t.Helper()
	var body struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(options, &body); err != nil {
		t.Fatalf("decode options: %v", err)
	}
	if body.PublicKey.Challenge == "" {
		t.Fatal("options carry no challenge")
	}
	return body.PublicKey.Challenge
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return encoded
}

// The whole point of the feature, proved once with real cryptography.
func TestRegisterThenSignInWithRealSignatures(t *testing.T) {
	c := newTestCeremony(t)
	auth := newAuthenticator(t)
	account := user.User{ID: "abc123", DisplayName: "Alice", CreatedAt: time.Now()}

	options, state, err := c.BeginRegistration(account)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}

	credential, err := c.FinishRegistration(account, state, auth.register(t, options))
	if err != nil {
		t.Fatalf("FinishRegistration: %v", err)
	}
	if string(credential.ID) != string(auth.credID) {
		t.Error("the stored credential id is not the one the authenticator made")
	}
	if len(credential.PublicKey) == 0 {
		t.Fatal("no public key was stored; every future assertion would fail")
	}
	// Read straight off the authenticator data, and the UI's only signal for
	// whether an account is one dropped phone from oblivion.
	if !credential.BackupEligible || !credential.BackupState {
		t.Error("the backup flags did not survive; the UI could not warn about a device-bound passkey")
	}

	account.Credentials = []user.Credential{credential}

	// Sign in. Note that nothing below names the account: the assertion is
	// resolved entirely from what the authenticator returns.
	loginOptions, loginState, err := c.BeginLogin()
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}

	var lookedUpCredential, lookedUpHandle []byte
	lookup := func(rawCredentialID, userHandle []byte) (user.User, error) {
		lookedUpCredential = rawCredentialID
		lookedUpHandle = userHandle
		return account, nil
	}

	id, used, err := c.FinishLogin(loginState, auth.assert(t, loginOptions, account.ID), lookup)
	if err != nil {
		t.Fatalf("FinishLogin: %v", err)
	}
	if id != account.ID {
		t.Errorf("signed in as %q, want %q", id, account.ID)
	}
	if string(lookedUpCredential) != string(auth.credID) {
		t.Error("the lookup was not given the asserted credential id")
	}
	if string(lookedUpHandle) != string(account.ID) {
		t.Error("the lookup was not given the asserted user handle")
	}
	if used.SignCount != 1 {
		t.Errorf("SignCount = %d, want the authenticator's advanced counter", used.SignCount)
	}
}

// A challenge belongs to the ceremony it was minted for. Replaying an
// assertion against a different one must fail, or the sealed-cookie design
// would be worthless.
func TestAssertionFromAnotherCeremonyIsRejected(t *testing.T) {
	c := newTestCeremony(t)
	auth := newAuthenticator(t)
	account := user.User{ID: "abc123", DisplayName: "Alice"}

	options, state, err := c.BeginRegistration(account)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	credential, err := c.FinishRegistration(account, state, auth.register(t, options))
	if err != nil {
		t.Fatalf("FinishRegistration: %v", err)
	}
	account.Credentials = []user.Credential{credential}

	// Two ceremonies; answer the first while presenting the second's state.
	firstOptions, _, err := c.BeginLogin()
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	_, secondState, err := c.BeginLogin()
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}

	lookup := func([]byte, []byte) (user.User, error) { return account, nil }
	if _, _, err := c.FinishLogin(secondState, auth.assert(t, firstOptions, account.ID), lookup); err == nil {
		t.Fatal("an assertion for another challenge was accepted")
	}
}

// The signature is the whole proof. A credential signed by a different key
// must not verify, however well-formed everything else is.
func TestAssertionFromTheWrongKeyIsRejected(t *testing.T) {
	c := newTestCeremony(t)
	enrolled := newAuthenticator(t)
	account := user.User{ID: "abc123", DisplayName: "Alice"}

	options, state, err := c.BeginRegistration(account)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	credential, err := c.FinishRegistration(account, state, enrolled.register(t, options))
	if err != nil {
		t.Fatalf("FinishRegistration: %v", err)
	}
	account.Credentials = []user.Credential{credential}

	// An impostor holding the credential id but not the private key.
	impostor := newAuthenticator(t)
	impostor.credID = enrolled.credID

	loginOptions, loginState, err := c.BeginLogin()
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	lookup := func([]byte, []byte) (user.User, error) { return account, nil }

	if _, _, err := c.FinishLogin(loginState, impostor.assert(t, loginOptions, account.ID), lookup); err == nil {
		t.Fatal("an assertion signed by the wrong key was accepted")
	}
}

// The origin is bound inside the signed client data, which is what makes a
// passkey unphishable: a lookalike site cannot produce this.
func TestAssertionFromAnotherOriginIsRejected(t *testing.T) {
	c, err := New(Config{
		RPID:            testRPID,
		RPDisplayName:   "easydnd",
		RPOrigins:       []string{"https://easydnd.org"},
		CeremonyTimeout: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	auth := newAuthenticator(t)
	account := user.User{ID: "abc123", DisplayName: "Alice"}

	options, state, err := c.BeginRegistration(account)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	// The authenticator signs testOrigin, which this relying party does not
	// accept.
	if _, err := c.FinishRegistration(account, state, auth.register(t, options)); err == nil {
		t.Fatal("a ceremony from an unlisted origin was accepted")
	}
}
