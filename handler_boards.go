package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	utils "github.com/nazifbara/kanban-api/internal"
	"github.com/nazifbara/kanban-api/internal/database"
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

func (s *server) hanlderUpdateBoard(w http.ResponseWriter, r *http.Request) {
	boardID, err := utils.GetIdFromPath(r, "boardID")
	if err != nil {
		log.Printf("invalid board id: %v", err)
		respondWithError(w, 400, "invalid uuid", err)
		return
	}
	params, err := decodeJSONBody[BoardParam](r)
	if err != nil {
		respondWithError(w, 400, "Invalid request body", err)
		return
	}
	if params.Name == "" {
		err := fmt.Errorf("body.name is requires")
		respondWithError(w, 400, err.Error(), err)
		return
	}
	dbBoard, err := s.dbQueries.UpdateBoard(r.Context(), database.UpdateBoardParams{Name: params.Name, ID: boardID})
	if err != nil {
		respondFromDBErr(w, "Board not found", err)
		return
	}
	respondWithJSON(w, 201, dbToBoard(dbBoard))
}

func (s *server) handlerDeleteBoard(w http.ResponseWriter, r *http.Request) {
	boardID, err := utils.GetIdFromPath(r, "boardID")
	if err != nil {
		log.Printf("invalid board id: %v", err)
		respondWithError(w, 400, "invalid uuid", err)
		return
	}
	_, err = s.dbQueries.DeleteBoard(r.Context(), boardID)
	if err != nil {
		respondFromDBErr(w, "Board not found", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handlerGetBoard(w http.ResponseWriter, r *http.Request) {
	boardID, err := utils.GetIdFromPath(r, "boardID")
	if err != nil {
		log.Printf("invalid board id: %v", err)
		respondWithError(w, 400, "invalid uuid", err)
		return
	}
	dbBoard, err := s.dbQueries.GetBoardByID(r.Context(), boardID)
	if err != nil {
		respondFromDBErr(w, "Board not found", err)
		return
	}
	respondWithJSON(w, 200, dbToBoard(dbBoard))
}

func (s *server) handlerGetAllBoards(w http.ResponseWriter, r *http.Request) {
	dbBoards, err := s.dbQueries.GetAllBoards(r.Context())
	if err != nil {
		respondWith500(w, err)
		return
	}
	boards := dbToBoardSlice(dbBoards)

	respondWithJSON(w, 200, boards)
}

func (s *server) handlerCreateBoard(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[BoardParam](r)
	if err != nil {
		respondWithError(w, 400, "invalid request body", err)
		return
	}

	if err := validateBoardParams(params); err != nil {
		respondWithError(w, 400, err.Error(), err)
		return
	}

	dbBoard, err := s.dbQueries.CreateBoard(r.Context(), params.Name)
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

func dbToBoard(dbBoard database.Board) Board {
	return Board(dbBoard)
}

func dbToBoardSlice(dbBoards []database.Board) []Board {
	var boards []Board
	for _, dbBoard := range dbBoards {
		boards = append(boards, dbToBoard(dbBoard))
	}
	return boards
}
