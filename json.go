package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
)

func decodeJSONBody[T any](r *http.Request) (T, error) {
	var params T
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&params); err != nil {
		return params, err
	}
	return params, nil
}

func respondFromDBErr(ctx context.Context, w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		respondWithError(ctx, w, http.StatusNotFound, err)
		return
	}
	respondWith500(ctx, w, err)
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

func respondWithError(ctx context.Context, w http.ResponseWriter, code int, err error) {
	errs := []error{err}
	codesToText := []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError, http.StatusNotFound}
	type respondBody struct {
		Error string `json:"error"`
	}
	var response respondBody
	if slices.Contains(codesToText, code) {
		response.Error = http.StatusText(code)
	} else {
		response.Error = err.Error()
	}
	data, err := json.Marshal(response)
	if err != nil {
		errs = append(errs, err)
		w.WriteHeader(500)
		return
	}
	if logCtx, ok := ctx.Value(logContextKey).(*LogContext); ok {
		logCtx.Error = errors.Join(errs...)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}

func respondWith500(ctx context.Context, w http.ResponseWriter, err error) {
	respondWithError(ctx, w, 500, err)
}
