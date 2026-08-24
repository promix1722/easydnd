// Package helpers holds the HTTP layer's response plumbing.
//
// This file owns the one and only error-to-HTTP-status mapping in the
// codebase. Handlers never build an error body themselves; they call
// FormatError and return.
package helpers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/logging"
	"github.com/promix1722/easydnd/internal/types"
)

// ContextKeyRequestID is the gin context key under which the request-ID
// middleware parks the correlation id.
const ContextKeyRequestID = "request_id"

// FieldError is the wire form of a field-level validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule,omitempty"`
	Message string `json:"message,omitempty"`
}

// ErrorBody is the error payload.
type ErrorBody struct {
	Code      string       `json:"code"`
	Message   string       `json:"message"`
	Fields    []FieldError `json:"fields,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
}

// ErrorResponse is the envelope every failed request returns.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// FormatError is the single exit point for a failed request. It classifies
// err, logs it when the fault is ours, and aborts the handler chain with a
// JSON envelope carrying the request id.
func FormatError(c *gin.Context, err error) {
	status, body := classify(err)

	if status >= http.StatusInternalServerError {
		// Log the real error; the client only ever sees the generic message
		// substituted by classify.
		logging.FromContext(c.Request.Context()).Error("request failed",
			"error", err.Error(),
			"status", status,
			"route", c.FullPath(),
		)
	}

	body.RequestID = c.GetString(ContextKeyRequestID)
	c.AbortWithStatusJSON(status, ErrorResponse{Error: body})
}

// classify maps an error onto a status and a body.
//
// Every branch uses errors.As rather than a concrete type switch, so an error
// wrapped with fmt.Errorf("%w") still classifies correctly. A type switch
// silently degrades every wrapped error to a 500, which is exactly the bug
// this package exists to avoid.
func classify(err error) (int, ErrorBody) {
	var fieldErr *types.FieldValidationError
	if errors.As(err, &fieldErr) {
		fields := make([]FieldError, 0, len(fieldErr.Fields))
		for _, f := range fieldErr.Fields {
			fields = append(fields, FieldError{Field: f.Field, Rule: f.Rule, Message: f.Message})
		}
		return http.StatusBadRequest, ErrorBody{
			Code:    "field_validation_error",
			Message: fieldErr.Message,
			Fields:  fields,
		}
	}

	var validationErr *types.ValidationError
	if errors.As(err, &validationErr) {
		return http.StatusBadRequest, ErrorBody{
			Code:    "validation_error",
			Message: validationErr.Message,
		}
	}

	// Before access-denied, and both before not-found: an unidentified caller
	// must be told to sign in rather than told no.
	var unauthErr *types.UnauthenticatedError
	if errors.As(err, &unauthErr) {
		return http.StatusUnauthorized, ErrorBody{
			Code:    "unauthenticated",
			Message: unauthErr.Message,
		}
	}

	var accessErr *types.AccessDeniedError
	if errors.As(err, &accessErr) {
		return http.StatusForbidden, ErrorBody{
			Code:    "access_denied",
			Message: accessErr.Message,
		}
	}

	var notFoundErr *types.NotFoundError
	if errors.As(err, &notFoundErr) {
		return http.StatusNotFound, ErrorBody{
			Code:    "not_found",
			Message: notFoundErr.Message,
		}
	}

	var notImplErr *types.NotImplementedError
	if errors.As(err, &notImplErr) {
		return http.StatusNotImplemented, ErrorBody{
			Code:    "not_implemented",
			Message: notImplErr.Message,
		}
	}

	// Malformed request bodies surface as stdlib decode errors.
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &syntaxErr) || errors.As(err, &typeErr) {
		return http.StatusBadRequest, ErrorBody{
			Code:    "validation_error",
			Message: "request body is not valid JSON",
		}
	}

	// Default. *types.ServerError deliberately has no branch of its own: it
	// lands here, so its message is logged but never sent to the client.
	return http.StatusInternalServerError, ErrorBody{
		Code:    "server_error",
		Message: "internal server error",
	}
}
