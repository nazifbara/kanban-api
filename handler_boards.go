package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	CreatorID   uuid.UUID `json:"creator_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type BoardParam struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

var boardContextKey contextKey = "board_context"

func getBoardFromCtx(ctx context.Context) (*database.Board, error) {
	if board, ok := ctx.Value(boardContextKey).(*database.Board); ok {
		return board, nil
	}
	return nil, errors.New("board context value not set")
}

func (s *server) withBoardAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		boardID, _ := utils.GetIdFromPath(r, "boardID")
		if boardID == uuid.Nil {
			respondWithError(r.Context(), w, apierrors.New(http.StatusBadRequest, "missing boardID"))
			return
		}
		userID, err := getUserIDFromCtx(r.Context())
		if err != nil {
			respondWithError(r.Context(), w, errors.New("auth context not set"))
			return
		}
		board, err := s.store.GetBoard(r.Context(), boardID)
		if err != nil {
			respondWithError(r.Context(), w, apierrors.FromDBErr(err))
			return
		}
		if board.CreatorID != userID {
			respondWithError(r.Context(), w, apierrors.New(http.StatusForbidden, "board access denied"))
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), boardContextKey, &board))
		next(w, r)
	}
}

func (s *server) hanlderUpdateBoard(w http.ResponseWriter, r *http.Request) {
	board, err := getBoardFromCtx(r.Context())
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	params, err := decodeJSONBody[BoardParam](r)
	if err != nil {
		respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	if params.Name == "" {
		respondWithError(r.Context(), w, apierrors.New(http.StatusBadRequest, "body.name is required"))
		return
	}
	dbBoard, err := s.store.UpdateBoard(r.Context(), database.UpdateBoardParams{Name: params.Name, ID: board.ID})
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	respondWithJSON(r.Context(), w, 201, dbToBoard(dbBoard))
}

func (s *server) handlerDeleteBoard(w http.ResponseWriter, r *http.Request) {
	board, err := getBoardFromCtx(r.Context())
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	_, err = s.store.DeleteBoard(r.Context(), board.ID)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handlerGetBoard(w http.ResponseWriter, r *http.Request) {
	dbBoard, err := getBoardFromCtx(r.Context())
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	respondWithJSON(r.Context(), w, http.StatusOK, dbToBoard(*dbBoard))
}

func (s *server) handlerGetAllBoards(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCtx(r.Context())
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	dbBoards, err := s.store.GetUserBoards(r.Context(), userID)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	boards := dbToBoardSlice(dbBoards)
	respondWithJSON(r.Context(), w, 200, boards)
}

func (s *server) handlerCreateBoard(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCtx(r.Context())
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	params, err := decodeJSONBody[BoardParam](r)
	if err != nil {
		respondWithError(r.Context(), w, malformedBodyErr)
		return
	}

	if err := validateBoardParams(params); err != nil {
		respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}

	dbBoard, err := s.store.CreateBoard(r.Context(), database.CreateBoardParams{
		Name:        params.Name,
		Description: sql.NullString{String: params.Description, Valid: params.Description != ""},
		CreatorID:   userID,
	})
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}

	respondWithJSON(r.Context(), w, 201, dbToBoard(dbBoard))
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
		CreatorID:   dbBoard.CreatorID,
		CreatedAt:   dbBoard.CreatedAt,
		UpdatedAt:   dbBoard.UpdatedAt,
	}
}

func dbToBoardSlice(dbBoards []database.Board) []Board {
	boards := []Board{}
	for _, dbBoard := range dbBoards {
		boards = append(boards, dbToBoard(dbBoard))
	}
	return boards
}
