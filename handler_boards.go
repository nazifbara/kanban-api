package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/apierrors"
	"github.com/nazifbara/kanban-api/internal/database"
	"github.com/nazifbara/kanban-api/internal/utils"
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
		s.respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}
	params, err := decodeJSONBody[BoardParam](r)
	if err != nil {
		s.respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	if params.Name == "" {
		s.respondWithError(r.Context(), w, apierrors.New(http.StatusBadRequest, "body.name is required"))
		return
	}
	dbBoard, err := s.store.UpdateBoard(r.Context(), database.UpdateBoardParams{Name: params.Name, ID: boardID})
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	s.respondWithJSON(w, 201, dbToBoard(dbBoard))
}

func (s *server) handlerDeleteBoard(w http.ResponseWriter, r *http.Request) {
	boardID, err := utils.GetIdFromPath(r, "boardID")
	if err != nil {
		log.Printf("invalid board id: %v", err)
		s.respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}
	_, err = s.store.DeleteBoard(r.Context(), boardID)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handlerGetBoard(w http.ResponseWriter, r *http.Request) {
	boardID, err := utils.GetIdFromPath(r, "boardID")
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}
	dbBoard, err := s.store.GetBoard(r.Context(), boardID)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	s.respondWithJSON(w, http.StatusOK, dbToBoard(dbBoard))
}

func (s *server) handlerGetAllBoards(w http.ResponseWriter, r *http.Request) {
	dbBoards, err := s.store.GetAllBoards(r.Context())
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	boards := dbToBoardSlice(dbBoards)

	s.respondWithJSON(w, 200, boards)
}

func (s *server) handlerCreateBoard(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[BoardParam](r)
	if err != nil {
		s.respondWithError(r.Context(), w, malformedBodyErr)
		return
	}

	if err := validateBoardParams(params); err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}

	dbBoard, err := s.store.CreateBoard(r.Context(), database.CreateBoardParams{
		Name:        params.Name,
		Description: sql.NullString{String: params.Description, Valid: params.Description != ""},
	})
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}

	s.respondWithJSON(w, 201, dbToBoard(dbBoard))
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
