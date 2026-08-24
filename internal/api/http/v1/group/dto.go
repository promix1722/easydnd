package group

// Group is one group and its whole roster.
type Group struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`

	// Role is the *caller's* rank in this group, not anybody else's.
	//
	// It duplicates one row of Members on purpose. Every screen has to decide
	// which controls to draw, and without this each of them would re-derive
	// the answer by searching the roster for itself -- the same decision made
	// in three places, which is two places for it to be made differently.
	Role string `json:"role"`

	Members []Member `json:"members"`
}

// Summary is one row of the group list. It carries no roster: the list draws
// names and ranks, and loading every roster to render them would be one query
// per row.
type Summary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	Role      string `json:"role"`
}

// Member is one person's seat at the table.
type Member struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	JoinedAt    string `json:"joined_at"`

	// Anonymous marks a guest. The roster shows it because a guest's session
	// expires and cannot be recovered: the rest of the table should know that
	// this person will not be back under the same name.
	Anonymous bool `json:"anonymous"`
}

// Invite is a freshly minted link.
//
// The token is returned rather than a whole URL because the client knows its
// own origin and this server does not: it sits behind a proxy, and guessing
// the public address is how a link ends up pointing at 127.0.0.1.
type Invite struct {
	Token     string `json:"token"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expires_at"`
}

// Preview is what the holder of a link is told before they accept it.
//
// It names the group and whoever is asking, and stops there: the holder is not
// a member yet, and the roster belongs to the members.
type Preview struct {
	GroupID   string `json:"group_id"`
	GroupName string `json:"group_name"`
	Role      string `json:"role"`
	InvitedBy string `json:"invited_by,omitempty"`
	ExpiresAt string `json:"expires_at"`

	// AlreadyMember lets the screen say "you are already here" rather than
	// offering a button whose effect is invisible.
	AlreadyMember bool `json:"already_member"`
}
