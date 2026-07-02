package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Board struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BoardParam struct {
	Name string `json:"name"`
}

func (cfg *ApiConfig) HandlerCreateBoard(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	params := BoardParam{}
	if err := decoder.Decode(&params); err != nil {
		respondWith500(w, err)
		return
	}

	if err := validateBoardParams(params); err != nil {
		respondWithError(w, 400, err.Error(), err)
		return
	}

	dbBoard, err := cfg.DBQueries.CreateBoard(r.Context(), params.Name)
	if err != nil {
		respondWith500(w, err)
		return
	}

	respondWithJSON(w, 201, Board(dbBoard))
}

func validateBoardParams(param BoardParam) error {
	if param.Name == "" {
		return fmt.Errorf("body.name is required")
	}

	return nil
}
