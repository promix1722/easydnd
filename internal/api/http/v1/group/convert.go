package group

import (
	"time"

	domain "github.com/promix1722/easydnd/internal/domain/group"
	groupuc "github.com/promix1722/easydnd/internal/usecase/group"
)

// groupOf renders a group with the caller's own rank and the whole roster.
func groupOf(g domain.Group, role domain.Role, members []domain.Member) Group {
	out := Group{
		ID:        string(g.ID),
		Name:      g.Name,
		CreatedAt: g.CreatedAt.UTC().Format(time.RFC3339),
		Role:      string(role),
		Members:   make([]Member, 0, len(members)),
	}
	for _, m := range members {
		out.Members = append(out.Members, memberOf(m))
	}
	return out
}

// memberOf renders one seat.
func memberOf(m domain.Member) Member {
	return Member{
		UserID:      string(m.UserID),
		DisplayName: m.DisplayName,
		Role:        string(m.Role),
		JoinedAt:    m.JoinedAt.UTC().Format(time.RFC3339),
		Anonymous:   m.Anonymous(),
	}
}

// summaryOf renders one row of the group list.
func summaryOf(m domain.Membership) Summary {
	return Summary{
		ID:        string(m.Group.ID),
		Name:      m.Group.Name,
		CreatedAt: m.Group.CreatedAt.UTC().Format(time.RFC3339),
		Role:      string(m.Role),
	}
}

// inviteOf renders a freshly minted link.
func inviteOf(i groupuc.Invitation) Invite {
	return Invite{
		Token:     i.Token,
		Role:      string(i.Invite.Role),
		ExpiresAt: i.Invite.ExpiresAt.UTC().Format(time.RFC3339),
	}
}

// previewOf renders what the holder of a link is shown.
func previewOf(p groupuc.Preview) Preview {
	return Preview{
		GroupID:       string(p.Group.ID),
		GroupName:     p.Group.Name,
		Role:          string(p.Role),
		InvitedBy:     p.InvitedBy,
		ExpiresAt:     p.ExpiresAt.UTC().Format(time.RFC3339),
		AlreadyMember: p.AlreadyMember,
	}
}
