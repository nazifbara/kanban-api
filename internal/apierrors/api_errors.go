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

func (e APIErr) Unwrap() error {
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

	if pqErr, ok := errors.AsType[*pq.Error](err); ok {
		switch pqErr.Code {
		case "23505":
			return FromErr(http.StatusConflict, err)
		case "23503", "22P02", "22001", "22003", "23502", "23514":
			return FromErr(http.StatusBadRequest, err)
		case "40P01", "40001":
			return FromErr(http.StatusConflict, err)
		case "53300", "08000", "08003", "08006", "57P03":
			return FromErr(http.StatusServiceUnavailable, err)
		case "57014":
			return FromErr(http.StatusGatewayTimeout, err)
		}
	}

	return FromErr(http.StatusInternalServerError, err)
}
