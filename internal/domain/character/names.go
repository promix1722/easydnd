package character

// The wire names for the enums the projected sheet uses.
//
// They live in the domain for the same reason the catalogue's do: an enum
// that cannot name its own values forces every adapter to invent a spelling.
// Nothing outside the domain had to serialize a sheet until the HTTP layer
// did.

var speedKindNames = map[SpeedKind]string{
	SpeedNone: "none",
	Walking:   "walking",
	Flying:    "flying",
	Climbing:  "climbing",
	Swimming:  "swimming",
	Burrowing: "burrowing",
}

// String returns the speed's wire name, or "unknown" outside the enumeration.
func (k SpeedKind) String() string { return enumName(speedKindNames, k) }

var senseKindNames = map[SenseKind]string{
	SenseNone:   "none",
	Darkvision:  "darkvision",
	Blindsight:  "blindsight",
	Tremorsense: "tremorsense",
	Truesight:   "truesight",
}

// String returns the sense's wire name, or "unknown" outside the enumeration.
func (k SenseKind) String() string { return enumName(senseKindNames, k) }

var rechargeNames = map[Recharge]string{
	RechargeNone: "",
	OnShortRest:  "short-rest",
	OnLongRest:   "long-rest",
	OnDawn:       "dawn",
	AtWill:       "at-will",
}

// String returns the recharge's wire name, or "unknown" outside the
// enumeration. The zero value is empty, so a pool that never recharges
// serializes as an absent field rather than as "none".
func (r Recharge) String() string { return enumName(rechargeNames, r) }

var actionKindNames = map[ActionKind]string{
	ActionKindNone:  "none",
	MainAction:      "action",
	BonusAction:     "bonus-action",
	Reaction:        "reaction",
	FreeAction:      "free-action",
	LegendaryAction: "legendary-action",
}

// String returns the action kind's wire name, or "unknown" outside the
// enumeration.
func (k ActionKind) String() string { return enumName(actionKindNames, k) }

var actionSourceNames = map[ActionSource]string{
	ActionSourceNone: "none",
	Derived:          "derived",
	Manual:           "manual",
}

// String returns the provenance's wire name, or "unknown" outside the
// enumeration.
func (s ActionSource) String() string { return enumName(actionSourceNames, s) }

func enumName[T comparable](names map[T]string, value T) string {
	if got, ok := names[value]; ok {
		return got
	}
	return "unknown"
}
