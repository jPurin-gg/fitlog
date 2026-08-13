package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

const (
	CodeValidation       = "VALIDATION_ERROR"
	CodeUnauthenticated  = "UNAUTHENTICATED"
	CodeForbidden        = "FORBIDDEN"
	CodeNotFound         = "NOT_FOUND"
	CodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
	CodeConflict         = "CONFLICT"
	CodeRateLimited      = "RATE_LIMITED"
	CodeAIUnavailable    = "AI_UNAVAILABLE"
	CodeInternal         = "INTERNAL_ERROR"
)

type Error struct {
	Status int
	Code   string
	Detail string
	Fields map[string]string
	Cause  error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Cause)
	}
	return e.Code
}

func (e *Error) Unwrap() error { return e.Cause }

func New(status int, code, detail string) *Error {
	return &Error{Status: status, Code: code, Detail: detail}
}

func Wrap(err error, status int, code, detail string) *Error {
	return &Error{Status: status, Code: code, Detail: detail, Cause: err}
}

func Validation(detail string, fields map[string]string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: CodeValidation, Detail: detail, Fields: fields}
}

func Unauthenticated(detail string) *Error {
	return New(http.StatusUnauthorized, CodeUnauthenticated, detail)
}

func Forbidden(detail string) *Error {
	return New(http.StatusForbidden, CodeForbidden, detail)
}

func NotFound(detail string) *Error {
	return New(http.StatusNotFound, CodeNotFound, detail)
}

func MethodNotAllowed(detail string) *Error {
	return New(http.StatusMethodNotAllowed, CodeMethodNotAllowed, detail)
}

func Conflict(detail string) *Error {
	return New(http.StatusConflict, CodeConflict, detail)
}

func Internal(err error) *Error {
	return Wrap(err, http.StatusInternalServerError, CodeInternal, "処理中にエラーが発生しました。")
}

func As(err error) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal(err)
}
