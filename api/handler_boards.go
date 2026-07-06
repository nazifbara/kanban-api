package api

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

func (cfg *ApiConfig) HanlderUpdateBoard(w http.ResponseWriter, r *http.Request) {
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
	dbBoard, err := cfg.DBQueries.UpdateBoard(r.Context(), database.UpdateBoardParams{Name: params.Name, ID: boardID})
	if err != nil {
		respondFromDBErr(w, "Board not found", err)
		return
	}
	respondWithJSON(w, 201, dbToBoard(dbBoard))
}

func (cfg *ApiConfig) HandlerDeleteBoard(w http.ResponseWriter, r *http.Request) {
	boardID, err := utils.GetIdFromPath(r, "boardID")
	if err != nil {
		log.Printf("invalid board id: %v", err)
		respondWithError(w, 400, "invalid uuid", err)

		return
	}
	_, err = cfg.DBQueries.DeleteBoard(r.Context(), boardID)
	if err != nil {
		respondFromDBErr(w, "Board not found", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *ApiConfig) HandlerGetBoard(w http.ResponseWriter, r *http.Request) {
	boardID, err := utils.GetIdFromPath(r, "boardID")
	if err != nil {
		log.Printf("invalid board id: %v", err)
		respondWithError(w, 400, "invalid uuid", err)
		return
	}
	dbBoard, err := cfg.DBQueries.GetBoardByID(r.Context(), boardID)
	if err != nil {
		respondFromDBErr(w, "Board not found", err)
		return
	}
	respondWithJSON(w, 200, dbToBoard(dbBoard))
}

func (cfg *ApiConfig) HandlerGetAllBoards(w http.ResponseWriter, r *http.Request) {
	dbBoards, err := cfg.DBQueries.GetAllBoards(r.Context())
	if err != nil {
		respondWith500(w, err)
		return
	}
	boards := dbToBoardSlice(dbBoards)

	respondWithJSON(w, 200, boards)
}

func (cfg *ApiConfig) HandlerCreateBoard(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[BoardParam](r)
	if err != nil {
		respondWithError(w, 400, "invalid request body", err)
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
