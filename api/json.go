package api

import (
	"encoding/json"
	"log"
	"net/http"
)

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
