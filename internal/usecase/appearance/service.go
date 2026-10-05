// Package appearance manages the current account's appearance resource.
package appearance

import (
	"context"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// Service reads and replaces an account's appearance.
type Service struct{ repo user.Repository }

func NewService(repo user.Repository) *Service { return &Service{repo: repo} }

// Get returns the current persisted resource, including defaults for new accounts.
func (s *Service) Get(ctx context.Context, account user.User) (user.Appearance, error) {
	if err := requireAccount(account); err != nil {
		return user.Appearance{}, err
	}
	current, err := s.repo.ByID(ctx, account.ID)
	if err != nil {
		return user.Appearance{}, err
	}
	return current.Appearance.WithDefaults(), nil
}

func requireAccount(account user.User) error {
	if account.Anonymous || strings.HasPrefix(string(account.ID), user.AnonymousIDPrefix) {
		return types.NewAccessDeniedError("guests use browser appearance settings")
	}
	return nil
}

// Put replaces a preference for the authenticated account only.
func (s *Service) Put(ctx context.Context, account user.User, a user.Appearance) (user.Appearance, error) {
	if err := requireAccount(account); err != nil {
		return user.Appearance{}, err
	}
	if !a.Valid() {
		return user.Appearance{}, types.NewValidationError("unsupported appearance")
	}
	if err := s.repo.SetAppearance(ctx, account.ID, a); err != nil {
		return user.Appearance{}, err
	}
	return a, nil
}
