package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

func decodeJSONBody[T any](r *http.Request) (T, error) {
	var params T
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&params); err != nil {
		return params, err
	}
	return params, nil
}

func respondFromDBErr(w http.ResponseWriter, msg string, err error) {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, 404, msg, err)
			return
		}
		respondWith500(w, err)
		return
	}
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

func respondWithError(w http.ResponseWriter, code int, msg string, err error) {
	log.Printf("%v", err)
	type respondBody struct {
		Error string `json:"error"`
	}
	data, err := json.Marshal(respondBody{Error: msg})
	if err != nil {
		log.Printf("Error while marshaling response body: %v", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}

func respondWith500(w http.ResponseWriter, err error) {
	respondWithError(w, 500, "Somthing went wrong", err)
}
