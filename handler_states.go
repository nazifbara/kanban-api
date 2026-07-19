package main

import (
	"errors"
	"fmt"
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

type stateBoardID struct {
	BoardID uuid.UUID `json:"board_id"`
}

func (s *server) handlerGetStates(w http.ResponseWriter, r *http.Request) {
	param, err := decodeJSONBody[stateBoardID](r)
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, fmt.Errorf("Invalid request body"))
	}
	board, err := s.store.GetBoardByID(r.Context(), param.BoardID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	dbStates, err := s.store.GetStates(r.Context(), board.ID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	respondWithJSON(w, 200, dbToStateSlice(dbStates))
}

func (s *server) handlerCreateState(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[StateParams](r)
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, fmt.Errorf("malformed request body"))
		return
	}
	if err := validateState(params); err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, err)
		return
	}
	dbState, err := s.store.CreateState(
		r.Context(),
		database.CreateStateParams{BoardID: params.BoardID, Title: params.Title},
	)
	if err != nil {
		respondWith500(r.Context(), w, err)
		return
	}
	respondWithJSON(w, http.StatusCreated, dbToState(dbState))
}

func dbToStateSlice(dbStates []database.State) []State {
	states := []State{}
	for _, dbState := range dbStates {
		states = append(states, dbToState(dbState))
	}
	return states
}

func dbToState(dbState database.State) State {
	return State{
		ID:          dbState.ID,
		Title:       dbState.Title,
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
