package apierrors

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/lib/pq"
)

type MultiErr interface {
	error
	Unwrap() []error
}

type APIErr struct {
	StatusCode int
	error
}

func (e *APIErr) Unwrap() error {
	return e.error
}

func New(code int, msg string) error {
	return APIErr{
		StatusCode: code,
		error:      errors.New(msg),
	}
}

func FromErr(code int, err error) error {
	return APIErr{
		StatusCode: code,
		error:      err,
	}
}

func FromDBErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return FromErr(http.StatusNotFound, err)
	}
	return FromErr(http.StatusInternalServerError, err)
}

func IsDBRetryable(err error) bool {
	if pqErr, ok := errors.AsType[*pq.Error](err); ok {
		switch pqErr.Code {
		case "40P01", "40001":
			return true
		}
	}
	return false
}
