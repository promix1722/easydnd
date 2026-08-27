package group

import (
	"context"

	domain "github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// SetRole changes what an existing member's rank is, and is also how
// ownership moves.
//
// Owner only, in every direction. A DM may seat and unseat people -- see
// RemoveMember -- but deciding who runs the table is the one thing that stays
// with the person whose table it is.
//
// Asking for RoleOwner means "hand the group to this person", which is why it
// is this method and not a separate one: from the caller's side both are "set
// their role", and splitting them would only invite a client to promote
// somebody and then demote themselves as two requests, with the group briefly
// having two owners or none.
func (s *Service) SetRole(
	ctx context.Context, actor user.ID, id domain.ID, target user.ID, role domain.Role,
) error {
	_, actorRole, err := s.member(ctx, actor, id)
	if err != nil {
		return err
	}
	if actorRole != domain.RoleOwner {
		return types.NewAccessDeniedError("only the owner may change a member's role").
			Because("group.ownerOnly")
	}
	if !role.Valid() {
		return types.NewFieldValidationError("that is not a role",
			types.FieldError{Field: "role", Rule: "oneof", Reason: "field.role.any"})
	}

	targetRole, err := s.repo.MemberRole(ctx, id, target)
	if err != nil {
		if types.IsNotFound(err) {
			return types.NewNotFoundError("that person is not in this group").Because("group.notAMember")
		}
		return err
	}

	if role == domain.RoleOwner {
		return s.transfer(ctx, id, actor, target, targetRole)
	}

	// The owner cannot be demoted, only replaced. Otherwise a slip of the
	// finger leaves a group whose owner is a player and which nobody can
	// delete, rename or hand on.
	if targetRole == domain.RoleOwner {
		return types.NewAccessDeniedError(
			"the owner's role cannot be changed; transfer the group instead")
	}
	if targetRole == role {
		return nil
	}
	if err := s.repo.SetRole(ctx, id, target, role); err != nil {
		return err
	}
	s.log.Info("group role changed",
		"group_id", string(id), "actor_id", string(actor),
		"target_id", string(target), "role", string(role))
	return nil
}

// transfer hands the group to target and demotes the outgoing owner to DM.
//
// DM rather than player because the outgoing owner was, a moment ago, running
// the table, and because it is what makes "transfer, then leave" work as one
// obvious sequence: a DM may leave and an owner may not.
func (s *Service) transfer(
	ctx context.Context, id domain.ID, from, to user.ID, toRole domain.Role,
) error {
	if from == to || toRole == domain.RoleOwner {
		return types.NewValidationError("that person already owns this group").Because("group.alreadyOwner")
	}
	if err := s.repo.TransferOwnership(ctx, id, from, to); err != nil {
		return err
	}
	s.log.Info("group ownership transferred",
		"group_id", string(id), "from_id", string(from), "to_id", string(to))
	return nil
}

// RemoveMember unseats somebody, and is also how a member leaves: leaving is
// removing yourself, and making it one operation means the rule that stops an
// owner walking away from their group cannot be sidestepped by picking the
// other endpoint.
func (s *Service) RemoveMember(
	ctx context.Context, actor user.ID, id domain.ID, target user.ID,
) error {
	_, actorRole, err := s.member(ctx, actor, id)
	if err != nil {
		return err
	}

	if target == actor {
		// The one rule the user asked for by name: nobody leaves while they
		// are the last -- and, since there is only ever one, the only --
		// owner. Two ways out exist, so this traps nobody: hand the group to
		// somebody else and then leave, or delete it.
		if actorRole == domain.RoleOwner {
			return types.NewAccessDeniedError(
				"an owner cannot leave their own group; transfer it to somebody else first, or delete it")
		}
		return s.unseat(ctx, id, actor, target)
	}

	if !actorRole.AtLeast(domain.RoleDM) {
		return types.NewAccessDeniedError("only the owner or a DM may remove someone from a group")
	}

	targetRole, err := s.repo.MemberRole(ctx, id, target)
	if err != nil {
		if types.IsNotFound(err) {
			return types.NewNotFoundError("that person is not in this group").Because("group.notAMember")
		}
		return err
	}
	// A DM may remove players and other DMs -- but the owner is nobody's to
	// remove, including their own DMs'.
	if targetRole == domain.RoleOwner {
		return types.NewAccessDeniedError("the owner cannot be removed from their own group")
	}
	return s.unseat(ctx, id, actor, target)
}

// unseat performs the removal and records who did it.
func (s *Service) unseat(ctx context.Context, id domain.ID, actor, target user.ID) error {
	if err := s.repo.RemoveMember(ctx, id, target); err != nil {
		return err
	}
	s.log.Info("group member removed",
		"group_id", string(id), "actor_id", string(actor), "target_id", string(target))
	return nil
}
