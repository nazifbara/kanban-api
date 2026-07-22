package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	utils "github.com/nazifbara/kanban-api/internal"
	"github.com/nazifbara/kanban-api/internal/database"
)

type Board struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type BoardParam struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *server) hanlderUpdateBoard(w http.ResponseWriter, r *http.Request) {
	boardID, err := utils.GetIdFromPath(r, "boardID")
	if err != nil {
		log.Printf("invalid board id: %v", err)
		respondWithError(r.Context(), w, http.StatusBadRequest, fmt.Errorf("invalid board id"))
		return
	}
	params, err := decodeJSONBody[BoardParam](r)
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, errors.New("malformed request body"))
		return
	}
	if params.Name == "" {
		respondWithError(r.Context(), w, http.StatusBadRequest, errors.New("body.name is required"))
		return
	}
	dbBoard, err := s.store.UpdateBoard(r.Context(), database.UpdateBoardParams{Name: params.Name, ID: boardID})
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	respondWithJSON(w, 201, dbToBoard(dbBoard))
}

func (s *server) handlerDeleteBoard(w http.ResponseWriter, r *http.Request) {
	boardID, err := utils.GetIdFromPath(r, "boardID")
	if err != nil {
		log.Printf("invalid board id: %v", err)
		respondWithError(r.Context(), w, http.StatusBadRequest, errors.New("invalid board id"))
		return
	}
	_, err = s.store.DeleteBoard(r.Context(), boardID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handlerGetBoard(w http.ResponseWriter, r *http.Request) {
	boardID, err := utils.GetIdFromPath(r, "boardID")
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, errors.New("invalid board id"))
		return
	}
	dbBoard, err := s.store.GetBoardByID(r.Context(), boardID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	respondWithJSON(w, http.StatusOK, dbToBoard(dbBoard))
}

func (s *server) handlerGetAllBoards(w http.ResponseWriter, r *http.Request) {
	dbBoards, err := s.store.GetAllBoards(r.Context())
	if err != nil {
		respondWith500(r.Context(), w, err)
		return
	}
	boards := dbToBoardSlice(dbBoards)

	respondWithJSON(w, 200, boards)
}

func (s *server) handlerCreateBoard(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[BoardParam](r)
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, errors.New("invalid request body"))
		return
	}

	if err := validateBoardParams(params); err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, err)
		return
	}

	dbBoard, err := s.store.CreateBoard(r.Context(), database.CreateBoardParams{
		Name:        params.Name,
		Description: sql.NullString{String: params.Description, Valid: params.Description != ""},
	})
	if err != nil {
		respondWith500(r.Context(), w, err)
		return
	}

	respondWithJSON(w, 201, dbToBoard(dbBoard))
}

func validateBoardParams(param BoardParam) error {
	if param.Name == "" {
		return fmt.Errorf("body.name is required")
	}

	return nil
}

func dbToBoard(dbBoard database.Board) Board {
	return Board{
		ID:          dbBoard.ID,
		Name:        dbBoard.Name,
		Description: dbBoard.Description.String,
		CreatedAt:   dbBoard.CreatedAt,
		UpdatedAt:   dbBoard.UpdatedAt,
	}
}

func dbToBoardSlice(dbBoards []database.Board) []Board {
	var boards []Board
	for _, dbBoard := range dbBoards {
		boards = append(boards, dbToBoard(dbBoard))
	}
	return boards
}
