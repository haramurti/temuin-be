// Package apperror defines transport-agnostic application errors.
// The HTTP layer maps Kind to a status code.
package apperror

import "fmt"

type Kind int

const (
	KindInternal Kind = iota
	KindValidation
	KindBadRequest
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindUnprocessable
	KindTooManyRequests
)

type Error struct {
	Kind    Kind
	Code    string
	Message string
	Details map[string]string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

func New(kind Kind, code, msg string) *Error { return &Error{Kind: kind, Code: code, Message: msg} }

func Validation(details map[string]string) *Error {
	return &Error{Kind: KindValidation, Code: "VALIDATION_ERROR", Message: "Input tidak valid", Details: details}
}
func BadRequest(code, msg string) *Error    { return New(KindBadRequest, code, msg) }
func Unauthorized(code, msg string) *Error  { return New(KindUnauthorized, code, msg) }
func Forbidden(code, msg string) *Error     { return New(KindForbidden, code, msg) }
func NotFound(code, msg string) *Error      { return New(KindNotFound, code, msg) }
func Conflict(code, msg string) *Error      { return New(KindConflict, code, msg) }
func Unprocessable(code, msg string) *Error { return New(KindUnprocessable, code, msg) }
func Internal(err error) *Error {
	return &Error{Kind: KindInternal, Code: "INTERNAL_ERROR", Message: "Terjadi kesalahan pada server", Err: err}
}
