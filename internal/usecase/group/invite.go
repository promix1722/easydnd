package group

import (
	"context"
	"time"

	domain "github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// Invitation is a freshly minted invite and the token that carries it. The
// handler turns the token into a link; nothing else in the system ever sees it
// again, because nothing else stores it.
type Invitation struct {
	Invite domain.Invite
	Token  string
}

// Preview is what somebody holding a link is told before they accept it.
//
// It names the group and the person who invited them, because "join this
// group" with no idea whose it is or who is asking is not a decision anyone
// can make. It deliberately does not include the roster: the holder of a link
// is not yet a member, and the roster is the members' to see.
type Preview struct {
	Group     domain.Group
	Role      domain.Role
	ExpiresAt time.Time

	// InvitedBy is the inviter's display name, or empty if that account has
	// since gone. An invite outlives the person who made it by up to a day.
	InvitedBy string

	// AlreadyMember reports that the holder is in this group already, so the
	// screen can say "you are already here" rather than offering a button
	// that does nothing visible.
	AlreadyMember bool
}

// Invite mints a link that seats its holder at role.
//
// Owner and DMs may invite, and they may invite at either rank below owner.
// The link is reusable and cannot be withdrawn -- see domain.Invite, where
// that choice and its consequences are written down.
func (s *Service) Invite(
	ctx context.Context, actor user.ID, id domain.ID, role domain.Role,
) (Invitation, error) {
	_, actorRole, err := s.member(ctx, actor, id)
	if err != nil {
		return Invitation{}, err
	}
	if !actorRole.AtLeast(domain.RoleDM) {
		return Invitation{}, types.NewAccessDeniedError("only the owner or a DM may invite people to a group")
	}
	// RoleOwner is refused here, in the signer, and again on the way back out
	// of the signer. A link that made its holder the owner would be a way to
	// give somebody else's table away.
	if role != domain.RoleDM && role != domain.RolePlayer {
		return Invitation{}, types.NewFieldValidationError("that is not a role you can invite somebody as",
			types.FieldError{Field: "role", Rule: "oneof", Message: "dm or player"})
	}

	now := s.now()
	invite := domain.Invite{
		Group:     id,
		Role:      role,
		InvitedBy: actor,
		IssuedAt:  now,
		ExpiresAt: now.Add(domain.InviteTTL),
	}
	token, err := s.invites.SignInvite(invite)
	if err != nil {
		return Invitation{}, err
	}
	s.log.Info("group invite issued",
		"group_id", string(id), "actor_id", string(actor), "role", string(role))
	return Invitation{Invite: invite, Token: token}, nil
}

// PreviewInvite decodes a link without acting on it.
func (s *Service) PreviewInvite(ctx context.Context, actor user.ID, token string) (Preview, error) {
	invite, g, err := s.openInvite(ctx, token)
	if err != nil {
		return Preview{}, err
	}

	preview := Preview{Group: g, Role: invite.Role, ExpiresAt: invite.ExpiresAt}
	if _, err := s.repo.MemberRole(ctx, g.ID, actor); err == nil {
		preview.AlreadyMember = true
	} else if !types.IsNotFound(err) {
		return Preview{}, err
	}

	// A missing inviter is not an error. The invite is still good: it was
	// signed by this server, for this group, and has not expired.
	if inviter, err := s.users.ByID(ctx, invite.InvitedBy); err == nil {
		preview.InvitedBy = inviter.DisplayName
	} else if !types.IsNotFound(err) {
		return Preview{}, err
	}
	return preview, nil
}

// Accept seats the holder of a link.
//
// Accepting twice succeeds and changes nothing. The link is reusable by
// design, so a second click, a refresh, or two tabs must not produce an error
// -- and must not quietly change the rank of somebody who is already seated,
// which is why an existing membership returns before AddMember rather than
// through it.
func (s *Service) Accept(ctx context.Context, actor user.User, token string) (domain.Group, error) {
	invite, g, err := s.openInvite(ctx, token)
	if err != nil {
		return domain.Group{}, err
	}

	if _, err := s.repo.MemberRole(ctx, g.ID, actor.ID); err == nil {
		return g, nil
	} else if !types.IsNotFound(err) {
		return domain.Group{}, err
	}

	if err := s.ensureStored(ctx, actor); err != nil {
		return domain.Group{}, err
	}
	if err := s.repo.AddMember(ctx, g.ID, actor.ID, invite.Role, s.now()); err != nil {
		return domain.Group{}, err
	}
	s.log.Info("group invite accepted",
		"group_id", string(g.ID), "actor_id", string(actor.ID), "role", string(invite.Role))
	return g, nil
}

// openInvite verifies a token and loads the group it names.
//
// A group deleted after its link was made reports the deletion rather than a
// bare "not found", because the holder of a valid link has done nothing wrong
// and "that group is gone" is the only useful thing to tell them.
func (s *Service) openInvite(
	ctx context.Context, token string,
) (domain.Invite, domain.Group, error) {
	invite, err := s.invites.VerifyInvite(token, s.now())
	if err != nil {
		// A bad link is a bad *argument*, not a failure to authenticate, and
		// the difference is not pedantry. Every port that checks a token
		// reports UnauthenticatedError, helpers.FormatError renders that as
		// 401, and a 401 is precisely what tells this application's client
		// that the session is gone and to show the landing page. Passed
		// through, a link that expired yesterday would sign out the
		// perfectly-signed-in person who clicked it.
		//
		// This is the same translation Service.asUnauthenticated performs in
		// the auth usecase, in the other direction.
		if types.IsUnauthenticated(err) {
			return domain.Invite{}, domain.Group{},
				types.NewValidationError("this invitation link is not valid, or it has expired")
		}
		return domain.Invite{}, domain.Group{}, err
	}
	g, err := s.repo.ByID(ctx, invite.Group)
	if err != nil {
		if types.IsNotFound(err) {
			return domain.Invite{}, domain.Group{},
				types.NewNotFoundError("that invitation is for a group that no longer exists")
		}
		return domain.Invite{}, domain.Group{}, err
	}
	return invite, g, nil
}
