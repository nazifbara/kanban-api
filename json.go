package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/nazifbara/kanban-api/internal/apierrors"
	pkgerr "github.com/pkg/errors"
)

func decodeJSONBody[T any](r *http.Request) (T, error) {
	var params T
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&params); err != nil {
		return params, err
	}
	return params, nil
}

func (s *server) respondWithJSON(w http.ResponseWriter, code int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("couldn't marshal response data", slog.Any("error", err))
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, err := w.Write(data); err != nil {
		s.logger.Error("couldn't write response data", slog.Any("error", err))
		w.WriteHeader(500)
	}
}

func (s *server) respondWithError(ctx context.Context, w http.ResponseWriter, err error) {
	statusCode := 500
	if apiErr, ok := err.(apierrors.APIErr); ok {
		statusCode = apiErr.StatusCode
	}
	errs := []error{err}
	type respondBody struct {
		Errors []string `json:"errors"`
	}
	var response respondBody
	if statusCode != http.StatusBadRequest {
		response.Errors = append(response.Errors, http.StatusText(statusCode))
	} else if merr, ok := errors.AsType[apierrors.MultiErr](err); ok {
		for _, err := range merr.Unwrap() {
			response.Errors = append(response.Errors, err.Error())
		}
	} else {
		response.Errors = append(response.Errors, err.Error())
	}

	if logCtx, ok := ctx.Value(logContextKey).(*LogContext); ok {
		logCtx.Error = pkgerr.WithStack(errors.Join(errs...))
	}
	s.respondWithJSON(w, statusCode, response)
}
