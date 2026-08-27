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

// Args are the values a client interpolates into a translated message.
//
// Only things a person can act on belong here -- a length limit, a role name,
// a count. Never an opaque id: "character %q not found" with the id spliced in
// reads worse in every language than "that character is not there", and a
// visitor can do nothing with `chr_9f2a`. The id belongs in the log, next to
// the request that mentioned it.
type Args map[string]any

// FieldError describes one field that failed a validation rule.
//
// `Reason` is a message key, not a sentence: the words live in
// web/locales/*.json with every other caption. Where it is empty the client
// falls back to `Rule`, which is already a small closed vocabulary
// (required, max, oneof, ...) and is enough to say something useful.
type FieldError struct {
	Field  string
	Rule   string
	Reason string
	Args   Args
}

// base supplies Error and Unwrap to every sentinel type below.
//
// `Message` is for the log and for `Error()`. It never reaches a client --
// see helpers.FormatError, which serialises `Reason` instead. That split is
// the whole point: a Go error should say as much as it can to whoever is
// reading the logs, and a browser should be handed a key it can render in
// whatever language the person reading it speaks.
type base struct {
	Message string
	// Reason is a stable slug the client turns into a sentence, e.g.
	// "group.name.required". Empty means "no better answer than the class",
	// and the client falls back to a message keyed by the error code.
	Reason string
	Args   Args
	Cause  error
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

/*
Because attaches the slug a client renders, and optional values to interpolate.

It is a separate call rather than another constructor parameter, and that is
what keeps this change small. Of the couple of hundred places that raise an
error here, most are saying something only a developer wants: a sequence
number that did not line up, a value kind the wire should never have carried,
a ceremony envelope that would not decode. Those keep their English message,
which goes to the log, and answer the client with the generic sentence for
their code -- which is all a person could have done with them anyway.

The ones a person is genuinely meant to read say so explicitly:

	types.NewFieldValidationError("a group needs a name",
		types.FieldError{Field: "name", Rule: "required"}).
		Because("group.name.required")

So the vocabulary a translator has to cover is the set somebody actually reads,
and adding a message to it later is one call at the raise site.
*/
func (e *FieldValidationError) Because(reason string, args ...Args) *FieldValidationError {
	e.Reason, e.Args = reason, firstArgs(args)
	return e
}

// Because attaches the slug a client renders. See FieldValidationError.Because.
func (e *ValidationError) Because(reason string, args ...Args) *ValidationError {
	e.Reason, e.Args = reason, firstArgs(args)
	return e
}

// Because attaches the slug a client renders. See FieldValidationError.Because.
func (e *UnauthenticatedError) Because(reason string, args ...Args) *UnauthenticatedError {
	e.Reason, e.Args = reason, firstArgs(args)
	return e
}

// Because attaches the slug a client renders. See FieldValidationError.Because.
func (e *AccessDeniedError) Because(reason string, args ...Args) *AccessDeniedError {
	e.Reason, e.Args = reason, firstArgs(args)
	return e
}

// Because attaches the slug a client renders. See FieldValidationError.Because.
func (e *NotFoundError) Because(reason string, args ...Args) *NotFoundError {
	e.Reason, e.Args = reason, firstArgs(args)
	return e
}

// Because attaches the slug a client renders. See FieldValidationError.Because.
func (e *NotImplementedError) Because(reason string, args ...Args) *NotImplementedError {
	e.Reason, e.Args = reason, firstArgs(args)
	return e
}

// firstArgs takes the optional Args, so `Because("x")` and
// `Because("x", Args{...})` are both legal and neither needs a nil literal.
func firstArgs(args []Args) Args {
	if len(args) == 0 {
		return nil
	}
	return args[0]
}

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
