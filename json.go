package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"

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

func respondWithJSON(w http.ResponseWriter, code int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error while marshaling payload %v: %v", payload, err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}

func respondWithError(ctx context.Context, w http.ResponseWriter, err error) {
	statusCode := 500
	if apiErr, ok := err.(apierrors.APIErr); ok {
		statusCode = apiErr.StatusCode
	}
	errs := []error{err}
	codesToText := []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError, http.StatusNotFound}
	type respondBody struct {
		Errors []string `json:"errors"`
	}
	var response respondBody
	if slices.Contains(codesToText, statusCode) {
		response.Errors = append(response.Errors, http.StatusText(statusCode))
	} else if merr, ok := errors.AsType[multiErr](err); ok {
		for _, err := range merr.Unwrap() {
			response.Errors = append(response.Errors, err.Error())
		}
	} else {
		response.Errors = append(response.Errors, err.Error())
	}
	data, err := json.Marshal(response)
	if err != nil {
		errs = append(errs, err)
		w.WriteHeader(500)
		return
	}
	if logCtx, ok := ctx.Value(logContextKey).(*LogContext); ok {
		logCtx.Error = pkgerr.WithStack(errors.Join(errs...))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(data)
}
