package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/database"
)

type State struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	BoardID     uuid.UUID `json:"board_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Description string    `json:"description"`
}

type StateParams struct {
	Title   string    `json:"title"`
	BoardID uuid.UUID `json:"board_id"`
}

func (s *server) handlerCreateState(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[StateParams](r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if err := validateState(params); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error(), err)
		return
	}
	dbState, err := s.dbQueries.CreateState(
		r.Context(),
		database.CreateStateParams{BoardID: params.BoardID, Title: params.Title},
	)
	if err != nil {
		respondWith500(w, err)
		return
	}
	respondWithJSON(w, http.StatusCreated, dbToState(dbState))
}

func dbToState(dbState database.State) State {
	return State{
		ID:          dbState.ID,
		CreatedAt:   dbState.CreatedAt,
		UpdatedAt:   dbState.UpdatedAt,
		Description: dbState.Description.String,
		BoardID:     dbState.BoardID,
	}
}

func validateState(params StateParams) error {
	if params.BoardID == uuid.Nil {
		return errors.New("body.board_id is required")
	}
	if params.Title == "" {
		return errors.New("body.title is required")
	}
	return nil
}
