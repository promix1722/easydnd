package types

// Limits is how many of each thing one owner, group, game or character may
// hold. Creating past a number is refused with LimitReached.
//
// It is a struct in the binary and not a block of YAML on purpose: an unknown
// config key is fatal at startup, so a key is a two-release dance, and none of
// these numbers has needed to differ between environments yet. The services
// copy DefaultLimits when they are built; when a number has to vary, it
// becomes a constructor argument then.
type Limits struct {
	// Characters and Folders are per owner; Groups is groups owned.
	Characters int
	Folders    int
	Groups     int

	// Per group.
	GroupMembers    int
	GroupCharacters int
	GroupGames      int
	GroupPacks      int

	// GameEntries is one game's roster: seated characters and monsters.
	GameEntries int

	// Packs is an owner's unarchived packs; PackReleases is per pack.
	Packs        int
	PackReleases int

	// Per character. CharacterItems is stacks, not pieces, and must stay
	// above the 300 items the AI Wizard may set in one call.
	// CharacterCustomOptions counts every kind, notes included.
	// CharacterEvents is the whole log, which bounds every list this struct
	// does not name: traits, proficiencies, conditions, rests.
	CharacterItems         int
	CharacterNotes         int
	CharacterCustomOptions int
	CharacterEvents        int

	// WizardRunsPerDay is AI Wizard chats an owner started in the last 24h.
	WizardRunsPerDay int
}

// DefaultLimits is every limit the service enforces.
var DefaultLimits = Limits{
	Characters:             100,
	Folders:                50,
	Groups:                 20,
	GroupMembers:           50,
	GroupCharacters:        200,
	GroupGames:             50,
	GroupPacks:             20,
	GameEntries:            100,
	Packs:                  30,
	PackReleases:           100,
	CharacterItems:         500,
	CharacterNotes:         100,
	CharacterCustomOptions: 300,
	CharacterEvents:        20000,
	WizardRunsPerDay:       20,
}

// LimitReached is the one refusal every limit answers with. what is the
// Limits field in lower camel case, and the client's key is "limit."+what.
func LimitReached(what string, limit int) error {
	return NewValidationError("%s limit of %d reached", what, limit).
		Because("limit."+what, Args{"max": limit})
}
