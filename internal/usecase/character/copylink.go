package character

import (
	"context"
	"time"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// SetCopyLinks installs the port copy links are signed with.
func (s *Service) SetCopyLinks(links domain.CopyLinks) { s.copyLinks = links }

// CreateCopyLink mints a link whose holder may take a copy of a character.
// Only its owner may; see domain.CopyLink for what the link is and is not.
func (s *Service) CreateCopyLink(
	ctx context.Context, owner domain.OwnerID, id domain.ID,
) (string, time.Time, error) {
	if _, err := s.owned(ctx, owner, id); err != nil {
		return "", time.Time{}, err
	}
	now := s.Now()
	link := domain.CopyLink{Character: id, From: owner, IssuedAt: now, ExpiresAt: now.Add(domain.CopyLinkTTL)}
	token, err := s.copyLinks.SignCopyLink(link)
	if err != nil {
		return "", time.Time{}, err
	}
	s.log.Info("character copy link issued", "character_id", string(id), "actor_id", string(owner))
	return token, link.ExpiresAt, nil
}

// PreviewCopyLink says what a link offers without taking it: the character's
// name and class line, and nothing else of a sheet that is not the holder's.
func (s *Service) PreviewCopyLink(
	ctx context.Context, token string, locale rules.Locale,
) (domain.Summary, error) {
	source, cat, err := s.openCopyLink(ctx, token, locale)
	if err != nil {
		return domain.Summary{}, err
	}
	return domain.Summarize(source.ID, source.Owner, source.Folder, source.Log, cat), nil
}

// AcceptCopyLink files a copy of the linked character in actor's default
// folder. Accepting twice makes two copies: the link is reusable, and a copy
// is not a membership that a second click could find already there.
func (s *Service) AcceptCopyLink(
	ctx context.Context, actor domain.OwnerID, token string, locale rules.Locale,
) (domain.Character, error) {
	source, cat, err := s.openCopyLink(ctx, token, locale)
	if err != nil {
		return domain.Character{}, err
	}
	copied, err := s.copyTo(ctx, source, cat, actor, "", "")
	if err != nil {
		return domain.Character{}, err
	}
	s.log.Info("character copy link accepted",
		"character_id", string(source.ID), "copy_id", string(copied.ID), "actor_id", string(actor))
	return copied, nil
}

// openCopyLink verifies a token and loads the character it names, as the
// owner who made the link -- so a link stops working the moment that owner no
// longer has the character.
func (s *Service) openCopyLink(
	ctx context.Context, token string, locale rules.Locale,
) (domain.Character, *catalog.Catalog, error) {
	link, err := s.copyLinks.VerifyCopyLink(token, s.Now())
	if err != nil {
		// A bad link is a bad argument, not a lost session: passed through
		// as a 401 it would sign out whoever clicked it. See openInvite in
		// the group usecase.
		if types.IsUnauthenticated(err) {
			return domain.Character{}, nil,
				types.NewValidationError("this link is not valid, or it has expired").
					Because("invite.invalid")
		}
		return domain.Character{}, nil, err
	}
	return s.load(ctx, link.From, link.Character, locale)
}
