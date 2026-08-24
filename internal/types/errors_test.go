package types_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/promix1722/easydnd/internal/types"
)

// TestErrorsAsSurvivesWrapping is the regression test for the classification
// bug this vocabulary exists to avoid: a concrete type switch on an error
// (`switch e.(type)`) stops matching as soon as a caller wraps the error with
// %w, silently degrading a 404 into a 500. helpers.FormatError uses errors.As
// instead, so every sentinel must remain discoverable through wrapping.
func TestErrorsAsSurvivesWrapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		as   func(error) bool
	}{
		{
			name: "not found",
			err:  types.NewNotFoundError("character %q not found", "thoradin"),
			as: func(err error) bool {
				var target *types.NotFoundError
				return errors.As(err, &target)
			},
		},
		{
			name: "not implemented",
			err:  types.NewNotImplementedError("character.Create"),
			as: func(err error) bool {
				var target *types.NotImplementedError
				return errors.As(err, &target)
			},
		},
		{
			name: "access denied",
			err:  types.NewAccessDeniedError("nope"),
			as: func(err error) bool {
				var target *types.AccessDeniedError
				return errors.As(err, &target)
			},
		},
		{
			name: "validation",
			err:  types.NewValidationError("bad body"),
			as: func(err error) bool {
				var target *types.ValidationError
				return errors.As(err, &target)
			},
		},
		{
			name: "field validation",
			err:  types.NewFieldValidationError("invalid", types.FieldError{Field: "name", Rule: "required"}),
			as: func(err error) bool {
				var target *types.FieldValidationError
				return errors.As(err, &target)
			},
		},
		{
			name: "server",
			err:  types.NewServerError("boom"),
			as: func(err error) bool {
				var target *types.ServerError
				return errors.As(err, &target)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.as(tt.err) {
				t.Fatalf("errors.As failed on the bare error")
			}
			wrapped := fmt.Errorf("usecase: %w", tt.err)
			if !tt.as(wrapped) {
				t.Errorf("errors.As failed after one level of wrapping")
			}
			if !tt.as(fmt.Errorf("handler: %w", wrapped)) {
				t.Errorf("errors.As failed after two levels of wrapping")
			}
		})
	}
}

// TestWrapServerErrorPreservesCause checks that a wrapped cause stays
// reachable by errors.Is while the ServerError itself stays reachable by
// errors.As.
func TestWrapServerErrorPreservesCause(t *testing.T) {
	cause := errors.New("disk on fire")
	err := types.WrapServerError(cause, "persisting character")

	if !errors.Is(err, cause) {
		t.Errorf("errors.Is could not find the cause")
	}
	var target *types.ServerError
	if !errors.As(err, &target) {
		t.Errorf("errors.As could not find *ServerError")
	}
	if got, want := err.Error(), "persisting character: disk on fire"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// TestPredicates covers the convenience helpers.
func TestPredicates(t *testing.T) {
	if !types.IsNotFound(fmt.Errorf("wrapped: %w", types.NewNotFoundError("x"))) {
		t.Errorf("IsNotFound missed a wrapped *NotFoundError")
	}
	if types.IsNotFound(types.NewServerError("x")) {
		t.Errorf("IsNotFound matched a *ServerError")
	}
	if !types.IsNotImplemented(types.NewNotImplementedError("x")) {
		t.Errorf("IsNotImplemented missed a *NotImplementedError")
	}
}
