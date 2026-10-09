// Package apperr defines transport-agnostic application errors.
//
// Services return *Error values describing *what* went wrong (not found,
// conflict, invalid input, ...). The HTTP layer maps the Kind to a status code,
// so business logic never has to know about HTTP.
package apperr

import (
	"errors"
	"fmt"
)

type Kind uint8

const (
	KindInternal Kind = iota
	KindInvalid
	KindNotFound
	KindConflict
	KindForbidden
	KindUnauthorized
)

func (k Kind) String() string {
	switch k {
	case KindInvalid:
		return "invalid"
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindForbidden:
		return "forbidden"
	case KindUnauthorized:
		return "unauthorized"
	default:
		return "internal"
	}
}

type Error struct {
	Kind    Kind
	Message string
	// Fields carries per-field (or per-path) validation messages.
	Fields map[string]string
	// Err is the underlying cause; never exposed to clients.
	Err error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Kind, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// Is lets errors.Is match sentinel *Error values by kind and message.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Kind == t.Kind && e.Message == t.Message
}

func NotFound(message string) *Error {
	return &Error{Kind: KindNotFound, Message: message}
}

func Conflict(message string) *Error {
	return &Error{Kind: KindConflict, Message: message}
}

func Forbidden(message string) *Error {
	return &Error{Kind: KindForbidden, Message: message}
}

func Invalid(message string) *Error {
	return &Error{Kind: KindInvalid, Message: message}
}

// Validation builds an invalid-input error with field-level details.
func Validation(fields map[string]string) *Error {
	return &Error{Kind: KindInvalid, Message: "Validation failed", Fields: fields}
}

// ValidationWithMessage is Validation with a custom top-level message.
func ValidationWithMessage(message string, fields map[string]string) *Error {
	return &Error{Kind: KindInvalid, Message: message, Fields: fields}
}

// Internal wraps an unexpected failure. The message shown to clients is generic.
func Internal(err error, context string) *Error {
	return &Error{Kind: KindInternal, Message: context, Err: err}
}

// KindOf reports the Kind of err, defaulting to KindInternal for foreign errors.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindInternal
}

// As extracts an *Error from err's chain.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}
