// Package profile changes what an account shows of itself: its portrait.
package profile

import (
	"context"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	"github.com/promix1722/easydnd/internal/usecase/portrait"
)

type Service struct{ repo user.Repository }

func NewService(repo user.Repository) *Service { return &Service{repo: repo} }

// PutImage changes only the authenticated account's portrait. Empty removes it.
func (s *Service) PutImage(ctx context.Context, account user.User, image string) error {
	if account.Anonymous || strings.HasPrefix(string(account.ID), user.AnonymousIDPrefix) {
		return types.NewAccessDeniedError("a profile requires an account")
	}
	if !portrait.Valid(image) {
		return types.NewFieldValidationError("invalid portrait", portrait.FieldError())
	}
	return s.repo.SetImage(ctx, account.ID, image)
}
