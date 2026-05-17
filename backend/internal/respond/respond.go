// Package respond is the HTTP response helper used by handlers.
package respond

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type Error struct {
	Status int
	Msg    string
	Err    error
}

func (e *Error) Error() string {
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Err }

func New(status int, msg string, err error) *Error {
	return &Error{Status: status, Msg: msg, Err: err}
}

func NotFound(err error) *Error {
	return New(http.StatusNotFound, "not found", err)
}

func BadRequest(msg string, cause ...error) *Error {
	return New(http.StatusBadRequest, msg, firstOrNil(cause))
}

func Unauthorized(msg string, cause ...error) *Error {
	return New(http.StatusUnauthorized, msg, firstOrNil(cause))
}

func Forbidden(msg string, cause ...error) *Error {
	return New(http.StatusForbidden, msg, firstOrNil(cause))
}

func Conflict(msg string, cause ...error) *Error {
	return New(http.StatusConflict, msg, firstOrNil(cause))
}

func Internal(err error) *Error {
	return New(http.StatusInternalServerError, "internal error", err)
}

func firstOrNil(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errs[0]
}

func Handle(fn func(http.ResponseWriter, *http.Request) *Error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appErr := fn(w, r)
		if appErr == nil {
			return
		}
		if appErr.Err != nil {
			slog.ErrorContext(r.Context(), "handler error",
				"status", appErr.Status,
				"err", appErr.Err.Error(),
			)
		}
		Plain(w, appErr.Status, appErr.Msg)
	}
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

func Plain(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}
