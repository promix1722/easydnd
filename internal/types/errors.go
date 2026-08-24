// Package types defines the application's transport-agnostic error vocabulary.
//
// It deliberately contains no HTTP status codes: the error-to-status mapping
// lives exactly once, in internal/api/http/helpers. That is what lets the
// domain and usecase layers import this package without dragging in net/http.
package types

import (
	"errors"
	"fmt"
)

// FieldError describes one field that failed a validation rule.
type FieldError struct {
	Field   string
	Rule    string
	Message string
}

// base supplies Error and Unwrap to every sentinel type below.
type base struct {
	Message string
	Cause   error
}

func (b base) Error() string {
	if b.Cause != nil {
		return b.Message + ": " + b.Cause.Error()
	}
	return b.Message
}

// Unwrap keeps errors.Is and errors.As working through a wrapped cause.
func (b base) Unwrap() error { return b.Cause }

// FieldValidationError reports one or more fields that failed validation.
type FieldValidationError struct {
	base
	Fields []FieldError
}

// ValidationError reports a request that is malformed or invalid as a whole.
type ValidationError struct{ base }

// UnauthenticatedError reports a caller we could not identify at all: no
// session cookie, or one that is expired, forged or points at an account that
// no longer exists.
//
// Distinct from AccessDeniedError because the two ask the client for different
// things. "Sign in" is actionable; "you may not" is not, and a browser told
// 403 when it should have been told 401 will keep showing a signed-in shell to
// somebody who is signed out.
type UnauthenticatedError struct{ base }

// AccessDeniedError reports a caller who is known but not permitted.
type AccessDeniedError struct{ base }

// NotFoundError reports an addressed resource that does not exist.
type NotFoundError struct{ base }

// NotImplementedError reports a route that exists but has no behaviour yet.
// Every usecase in this skeleton returns one.
type NotImplementedError struct{ base }

// ServerError reports an unexpected internal failure. Its message is never
// sent to a client; helpers.FormatError substitutes a generic body.
type ServerError struct{ base }

// NewFieldValidationError builds a FieldValidationError over the given fields.
func NewFieldValidationError(msg string, fields ...FieldError) *FieldValidationError {
	return &FieldValidationError{base: base{Message: msg}, Fields: fields}
}

// NewValidationError builds a ValidationError from a format string.
func NewValidationError(format string, a ...any) *ValidationError {
	return &ValidationError{base{Message: fmt.Sprintf(format, a...)}}
}

// NewUnauthenticatedError builds an UnauthenticatedError from a format string.
func NewUnauthenticatedError(format string, a ...any) *UnauthenticatedError {
	return &UnauthenticatedError{base{Message: fmt.Sprintf(format, a...)}}
}

// NewAccessDeniedError builds an AccessDeniedError from a format string.
func NewAccessDeniedError(format string, a ...any) *AccessDeniedError {
	return &AccessDeniedError{base{Message: fmt.Sprintf(format, a...)}}
}

// NewNotFoundError builds a NotFoundError from a format string.
func NewNotFoundError(format string, a ...any) *NotFoundError {
	return &NotFoundError{base{Message: fmt.Sprintf(format, a...)}}
}

// NewNotImplementedError builds a NotImplementedError from a format string.
func NewNotImplementedError(format string, a ...any) *NotImplementedError {
	return &NotImplementedError{base{Message: fmt.Sprintf(format, a...)}}
}

// NewServerError builds a ServerError from a format string.
func NewServerError(format string, a ...any) *ServerError {
	return &ServerError{base{Message: fmt.Sprintf(format, a...)}}
}

// WrapServerError attaches a cause, so errors.As still finds *ServerError and
// errors.Is still finds the underlying sentinel.
func WrapServerError(cause error, format string, a ...any) *ServerError {
	return &ServerError{base{Message: fmt.Sprintf(format, a...), Cause: cause}}
}

// IsUnauthenticated reports whether err is, or wraps, an
// *UnauthenticatedError.
func IsUnauthenticated(err error) bool {
	var target *UnauthenticatedError
	return errors.As(err, &target)
}

// IsNotFound reports whether err is, or wraps, a *NotFoundError.
func IsNotFound(err error) bool {
	var target *NotFoundError
	return errors.As(err, &target)
}

// IsNotImplemented reports whether err is, or wraps, a *NotImplementedError.
func IsNotImplemented(err error) bool {
	var target *NotImplementedError
	return errors.As(err, &target)
}
