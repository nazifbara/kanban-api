package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
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

func (cfg *ApiConfig) HandlerGetBoard(w http.ResponseWriter, r *http.Request) {
	idQuery := r.PathValue("boardID")
	boardID, err := uuid.Parse(idQuery)
	if err != nil {
		log.Printf("invalid board id: %s", idQuery)
		respondWithError(w, 400, "invalid uuid", err)
		return
	}
	dbBoard, err := cfg.DBQueries.GetBoardByID(r.Context(), boardID)
	if err != nil {
		log.Printf("failed to get the baord: %v", err)
		respondWithError(w, 404, "Board not found", err)
		return
	}
	respondWithJSON(w, 200, dbToBoard(dbBoard))
}

func (cfg *ApiConfig) HandlerGetAllBoards(w http.ResponseWriter, r *http.Request) {
	dbBoards, err := cfg.DBQueries.GetAllBoards(r.Context())
	if err != nil {
		log.Printf("failed to get the board: %v", err)
		respondWith500(w, err)
		return
	}
	boards := dbToBoardSlice(dbBoards)

	respondWithJSON(w, 200, boards)
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
