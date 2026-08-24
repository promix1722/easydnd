package auth

import (
	"crypto/rand"
	"math/big"

	"github.com/promix1722/easydnd/internal/types"
)

// Naming an account nobody named.
//
// Neither passkey ceremony asks for anything: sign-in is discoverable, and
// sign-up has no reason to be different. But an account still needs a label,
// because the display name is what the operating system's passkey prompt --
// and the credential manager it is later listed in -- shows beside "easydnd".
// So the server picks one, exactly as the SSO path already does when a
// provider tells us nothing useful (see displayNameFor).
//
// Two words rather than one, and words rather than an id: this string sits in
// a list somebody scrolls through months later looking for their own passkey.
// "Brave Owl" is findable there in a way that "0hXcVv1QT..." is not.
//
// Collisions are fine and are not checked for. The account id is the key;
// users.display_name is neither unique nor indexed, and two people called
// "Brave Owl" is no more of a problem here than it is in a tavern.

// The words are deliberately ordinary English -- animals, metals, weather and
// medieval trades. None of them are drawn from the SRD text vendored under
// docs/reference_srd_5.1/, and none are Product Identity creatures, so this
// list is original work under the project's own licence and raises no
// attribution question. See docs/licensing.md.
//
// Both lists are sorted, so a duplicate is visible on review -- and a test
// fails if one slips through anyway, because a duplicate silently doubles that
// word's share of the draw.

var displayNameAdjectives = []string{
	"Amber", "Ashen", "Bold", "Brave", "Bronze", "Cinder",
	"Copper", "Crimson", "Dappled", "Dusky", "Eager", "Ember",
	"Fabled", "Gallant", "Gilded", "Grim", "Hollow", "Humble",
	"Iron", "Ivory", "Jolly", "Keen", "Lofty", "Lucky",
	"Merry", "Mossy", "Nimble", "Noble", "Patient", "Quiet",
	"Restless", "Russet", "Sable", "Silver", "Sly", "Solemn",
	"Stout", "Sunlit", "Swift", "Tawny", "Umber", "Valiant",
	"Velvet", "Wandering", "Wary", "Weathered", "Wintry", "Wistful",
}

// Nouns that read as somebody rather than as something. A credential manager
// entry is a person's account, so "Quiet Warden" belongs there and the name of
// a monster does not.
var displayNameNouns = []string{
	"Badger", "Bard", "Beacon", "Boar", "Cooper", "Crow",
	"Drake", "Falcon", "Ferret", "Finch", "Fox", "Griffon",
	"Harper", "Hawk", "Herald", "Heron", "Hound", "Ibex",
	"Jackdaw", "Kestrel", "Knight", "Lantern", "Lark", "Lynx",
	"Magpie", "Marten", "Mason", "Minstrel", "Moth", "Otter",
	"Owl", "Pilgrim", "Piper", "Ranger", "Raven", "Scout",
	"Sentinel", "Shrike", "Sparrow", "Squire", "Stag", "Starling",
	"Swallow", "Thrush", "Tinker", "Vixen", "Warden", "Weaver",
	"Wolf", "Wren",
}

// newDisplayName mints the label a new account is known by.
//
// It runs the result through normalizeDisplayName even though every pair in
// the lists above is short, trimmed ASCII by construction. The call is free,
// and it means a word added carelessly one day surfaces as a validation error
// here rather than as a constraint violation from users_display_name_len at
// the far end of a ceremony that has already prompted the authenticator.
// TestGeneratedDisplayNamesAreValid proves that branch is unreachable today.
func newDisplayName() (string, error) {
	adjective, err := pick(displayNameAdjectives)
	if err != nil {
		return "", err
	}
	noun, err := pick(displayNameNouns)
	if err != nil {
		return "", err
	}
	return normalizeDisplayName(adjective + " " + noun)
}

// pick draws one word uniformly.
//
// rand.Int rather than rand.Read and a modulo: the modulo is unbiased only
// when the list length divides 256, which is true of neither list and would
// stop being true silently the day somebody adds a word.
func pick(words []string) (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
	if err != nil {
		return "", types.WrapServerError(err, "generate display name")
	}
	return words[n.Int64()], nil
}
